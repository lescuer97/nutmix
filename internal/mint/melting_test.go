package mint

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"github.com/lescuer97/nutmix/api/cashu"
	mockdb "github.com/lescuer97/nutmix/internal/database/mock_db"
	"github.com/lescuer97/nutmix/internal/lightning"
	"github.com/lightningnetwork/lnd/zpay32"
)

const testSatKeysetId = "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52"

func makeAmountlessRegtestInvoice(t *testing.T) string {
	t.Helper()

	var paymentHash [32]byte
	if _, err := rand.Read(paymentHash[:]); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}

	payReq, err := zpay32.NewInvoice(&chaincfg.RegressionNetParams, paymentHash, time.Now(), zpay32.Description("amountless"))
	if err != nil {
		t.Fatalf("zpay32.NewInvoice: %v", err)
	}

	payReqString, err := payReq.Encode(zpay32.MessageSigner{
		SignCompact: func(msg []byte) ([]byte, error) {
			key, err := secp256k1.GeneratePrivateKey()
			if err != nil {
				return nil, fmt.Errorf("GeneratePrivateKey: %w", err)
			}
			return ecdsa.SignCompact(key, msg, true), nil
		},
	})
	if err != nil {
		t.Fatalf("payReq.Encode: %v", err)
	}
	return payReqString
}

func TestValidateBolt11MeltQuoteRejectsAmountlessInvoice(t *testing.T) {
	mint := Mint{ //nolint:exhaustruct // test only needs the LightningBackend; Config has 30+ fields
		LightningBackend: lightning.FakeWallet{UnpurposeErrors: nil, Network: chaincfg.RegressionNetParams, InvoiceFee: 0, NodeStatus: lightning.ONLINE_STATUS},
	}

	invoice := makeAmountlessRegtestInvoice(t)

	_, err := mint.validateBolt11MeltQuoteRequest(context.Background(), cashu.PostMeltQuoteBolt11Request{
		Options: cashu.PostMeltQuoteBolt11Options{Mpp: nil},
		Request: invoice,
		Unit:    cashu.Sat.String(),
	})
	if err == nil {
		t.Fatal("expected error for amountless invoice, got nil")
	}
	if !errors.Is(err, cashu.ErrAmountlessInvoiceNotSupported) {
		t.Fatalf("expected ErrAmountlessInvoiceNotSupported, got: %+v", err)
	}
}

func TestReconcilePendingMeltQuotesIgnoresStrikeEndOfLife(t *testing.T) {
	quote := cashu.MeltRequestDB{ //nolint:exhaustruct // Only reconciliation fields are relevant.
		Quote: "legacy-pending", Request: RegtestRequest, State: cashu.PENDING,
		Unit: cashu.Sat.String(), CheckingId: "legacy-checking-id",
	}
	db := &mockdb.MockDB{MeltRequest: []cashu.MeltRequestDB{quote}}                                     //nolint:exhaustruct
	mint := Mint{MintDB: db, LightningBackend: lightning.Strike{Network: chaincfg.RegressionNetParams}} //nolint:exhaustruct

	if err := mint.ReconcilePendingMeltQuotes(); err != nil {
		t.Fatalf("ReconcilePendingMeltQuotes: %v", err)
	}
	if db.MeltRequest[0].State != cashu.PENDING || db.MeltRequest[0].CheckingId != quote.CheckingId {
		t.Fatalf("pending quote changed: %+v", db.MeltRequest[0])
	}
}

// setupPendingMeltWithChange stores a PENDING melt quote, pending proofs and
// blank change outputs. FakeWallet.CheckPayed settles with a flat 10 sat fee.
func setupPendingMeltWithChange(t *testing.T, mint *Mint, quoteId string, quoteAmount uint64, proofAmounts []uint64, numBlanks int) []cashu.WrappedPublicKey {
	t.Helper()
	ctx := context.Background()
	now := time.Now().Unix()

	meltQuote := cashu.MeltRequestDB{
		PaymentPreimage: "",
		Unit:            cashu.Sat.String(),
		Request:         "lnbcrt2u1pna2hrlpp5gv4edjsvjzflaxex5y4jcm87yhhm7s6clt6hjar50yhswan83fesdqqcqzzsxqzuysp5u3kq8etcat22w2hraktrgppltaegt3prrf5qz9z4cplreje2kzrq9qxpqysgq2ujupalzlwz9nhn55pl6nuwtv4qqkdlkn02rf835l3janjwy7w3n0tl0whh6v3frpvfcsyzud3g6dsx6gqgmw04xj2c0alz4px5hjecq0pnclr",
		State:           cashu.PENDING,
		Quote:           quoteId,
		CheckingId:      "",
		Expiry:          now,
		Amount:          quoteAmount,
		FeeReserve:      2,
		FeePaid:         0,
		SeenAt:          now,
		Melted:          false,
		Mpp:             false,
		Change:          nil,
	}

	proofs := cashu.Proofs{}
	for i, amount := range proofAmounts {
		cPriv, err := secp256k1.GeneratePrivateKey()
		if err != nil {
			t.Fatalf("secp256k1.GeneratePrivateKey: %v", err)
		}
		proofs = append(proofs, cashu.Proof{
			C:       cashu.WrappedPublicKey{PublicKey: cPriv.PubKey()},
			Y:       cashu.WrappedPublicKey{PublicKey: nil},
			Quote:   &meltQuote.Quote,
			Id:      testSatKeysetId,
			Secret:  fmt.Sprintf("secret-%s-%d", quoteId, i),
			Witness: "",
			State:   cashu.PROOF_PENDING,
			Amount:  amount,
			SeenAt:  now,
		})
	}
	for i := range proofs {
		p, err := proofs[i].HashSecretToCurve()
		if err != nil {
			t.Fatalf("proofs.HashSecretToCurve: %v", err)
		}
		proofs[i] = p
	}

	blanks := make([]cashu.BlindedMessage, 0, numBlanks)
	blankKeys := make([]cashu.WrappedPublicKey, 0, numBlanks)
	for i := 0; i < numBlanks; i++ {
		bPriv, err := secp256k1.GeneratePrivateKey()
		if err != nil {
			t.Fatalf("secp256k1.GeneratePrivateKey: %v", err)
		}
		b := cashu.WrappedPublicKey{PublicKey: bPriv.PubKey()}
		blanks = append(blanks, cashu.BlindedMessage{Id: testSatKeysetId, B_: b, Witness: "", Amount: 0})
		blankKeys = append(blankKeys, b)
	}

	tx, err := mint.MintDB.GetTx(ctx)
	if err != nil {
		t.Fatalf("mint.MintDB.GetTx: %v", err)
	}
	defer func() { _ = mint.MintDB.Rollback(ctx, tx) }()

	if err := mint.MintDB.SaveMeltRequest(tx, meltQuote); err != nil {
		t.Fatalf("SaveMeltRequest: %v", err)
	}
	if err := mint.MintDB.SaveProof(tx, proofs); err != nil {
		t.Fatalf("SaveProof: %v", err)
	}
	if err := mint.MintDB.SaveMeltChange(tx, blanks, quoteId); err != nil {
		t.Fatalf("SaveMeltChange: %v", err)
	}
	if err := mint.MintDB.Commit(ctx, tx); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	return blankKeys
}

func TestRefreshMeltQuoteReturnsChangeOnSettle(t *testing.T) {
	mint := SetupMintWithLightningMockPostgres(t)

	// 15 sats of proofs, 2 sat invoice, 10 sat fake fee -> overpaid = 3 -> change [1, 2]
	blankKeys := setupPendingMeltWithChange(t, mint, "quote-change-ok", 2, []uint64{8, 4, 2, 1}, 2)

	quote, err := mint.RefreshMeltQuoteState(context.Background(), "quote-change-ok")
	if err != nil {
		t.Fatalf("RefreshMeltQuoteState: %+v", err)
	}

	if quote.State != cashu.PAID {
		t.Fatalf("quote should be PAID, got: %v", quote.State)
	}

	if len(quote.Change) != 2 {
		t.Fatalf("expected 2 change signatures, got: %d", len(quote.Change))
	}
	// ponytail: row order from melt_change_message is not guaranteed (no ordinal
	// column) — assert the multiset of amounts, not positions.
	changeAmounts := map[uint64]bool{quote.Change[0].Amount: true, quote.Change[1].Amount: true}
	if !changeAmounts[1] || !changeAmounts[2] {
		t.Errorf("change amounts should be {1, 2}, got: [%d %d]", quote.Change[0].Amount, quote.Change[1].Amount)
	}

	// signatures must have been persisted for both blank outputs
	ctx := context.Background()
	tx, err := mint.MintDB.GetTx(ctx)
	if err != nil {
		t.Fatalf("mint.MintDB.GetTx: %v", err)
	}
	defer func() { _ = mint.MintDB.Rollback(ctx, tx) }()

	restoreSigs, err := mint.MintDB.GetRestoreSigsFromBlindedMessages(tx, blankKeys)
	if err != nil {
		t.Fatalf("GetRestoreSigsFromBlindedMessages: %v", err)
	}
	if len(restoreSigs) != 2 {
		t.Errorf("expected restore sigs for both blank outputs, got: %d", len(restoreSigs))
	}

	remainingChange, err := mint.MintDB.GetMeltChangeByQuote(tx, "quote-change-ok")
	if err != nil {
		t.Fatalf("GetMeltChangeByQuote: %v", err)
	}
	if len(remainingChange) != 0 {
		t.Errorf("change messages should be deleted after signing, got: %d", len(remainingChange))
	}
}

func TestRefreshMeltQuoteNoChangeWhenFeeExceedsProofs(t *testing.T) {
	mint := SetupMintWithLightningMockPostgres(t)

	// 4 sats of proofs, 2 sat invoice, 10 sat fake fee -> totalExpent (12) > proofs (4).
	// Without the comparison guard this underflows uint64 and signs huge change.
	setupPendingMeltWithChange(t, mint, "quote-underflow", 2, []uint64{2, 2}, 2)

	quote, err := mint.RefreshMeltQuoteState(context.Background(), "quote-underflow")
	if err != nil {
		t.Fatalf("RefreshMeltQuoteState: %+v", err)
	}

	if quote.State != cashu.PAID {
		t.Fatalf("quote should be PAID, got: %v", quote.State)
	}
	if len(quote.Change) != 0 {
		t.Errorf("no change should be signed on fee overdraw, got: %d signatures", len(quote.Change))
	}

	ctx := context.Background()
	tx, err := mint.MintDB.GetTx(ctx)
	if err != nil {
		t.Fatalf("mint.MintDB.GetTx: %v", err)
	}
	defer func() { _ = mint.MintDB.Rollback(ctx, tx) }()

	proofs, err := mint.MintDB.GetProofsFromQuote(tx, "quote-underflow")
	if err != nil {
		t.Fatalf("GetProofsFromQuote: %v", err)
	}
	for _, proof := range proofs {
		if proof.State != cashu.PROOF_SPENT {
			t.Errorf("proof should be spent, got: %v", proof.State)
		}
	}
}
