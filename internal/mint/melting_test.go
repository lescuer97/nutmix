//nolint:exhaustruct_v5 // Tests use focused fixtures with only relevant fields.
package mint

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"github.com/lescuer97/nutmix/api/cashu"
	mockdb "github.com/lescuer97/nutmix/internal/database/mock_db"
	"github.com/lescuer97/nutmix/internal/lightning"
	"github.com/lescuer97/nutmix/internal/signer"
	"github.com/lightningnetwork/lnd/zpay32"
)

type noLightningReconciliationWallet struct{ lightning.FakeWallet }

func (noLightningReconciliationWallet) CheckPayed(context.Context, string, *zpay32.Invoice, string) (lightning.PaymentStatus, string, cashu.Amount, error) {
	return lightning.UNKNOWN, "", cashu.Amount{}, errors.New("unexpected CheckPayed call")
}

func (noLightningReconciliationWallet) PayInvoice(cashu.MeltRequestDB, *zpay32.Invoice, cashu.Amount, bool, cashu.Amount) (lightning.PaymentResponse, error) {
	return lightning.PaymentResponse{}, errors.New("unexpected PayInvoice call")
}

type reconciliationCountingSigner struct {
	lifecycleSigner
	calls int
}

func (s *reconciliationCountingSigner) GetKeysets() (signer.GetKeysetsResponse, error) {
	return signer.GetKeysetsResponse{Keysets: []cashu.BasicKeysetResponse{{Id: testSatKeysetId, Unit: cashu.Sat.String()}}}, nil
}

func (s *reconciliationCountingSigner) SignBlindMessages(ctx context.Context, outputs []cashu.BlindedMessage) ([]cashu.BlindSignature, []cashu.SignedOutput, error) {
	s.calls++
	return s.lifecycleSigner.SignBlindMessages(ctx, outputs)
}

func TestMeltSettlementStartupRecoversInternalPreparation(t *testing.T) {
	db := &mockdb.MockDB{}
	signer := &reconciliationCountingSigner{}
	mint := &Mint{MintDB: db, Signer: signer, LightningBackend: noLightningReconciliationWallet{}}
	setupPendingMeltWithChange(t, mint, "startup-internal", []uint64{8, 4, 2, 1}, 3)
	db.MeltRequest[0].State = cashu.PAID
	db.MeltRequest[0].Melted = true
	db.MeltRequest[0].FeePaid = 0
	db.MeltRequest[0].PaymentPreimage = "durable-preimage"

	if err := mint.ReconcilePendingMeltQuotes(context.Background()); err != nil {
		t.Fatalf("startup reconciliation: %v", err)
	}
	if signer.calls != 1 {
		t.Fatalf("SignBlindMessages calls = %d, want 1", signer.calls)
	}
	if db.MeltRequest[0].PaymentPreimage != "durable-preimage" || db.MeltRequest[0].FeePaid != 0 {
		t.Fatalf("durable settlement fields changed: %+v", db.MeltRequest[0])
	}
	proofs, err := db.GetProofsFromQuote(context.Background(), nil, "startup-internal")
	if err != nil {
		t.Fatalf("GetProofsFromQuote: %v", err)
	}
	for _, proof := range proofs {
		if proof.State != cashu.PROOF_SPENT {
			t.Errorf("proof state = %v, want SPENT", proof.State)
		}
	}
	if len(db.MeltChange) != 0 {
		t.Fatalf("unused reserved blanks remain: %d", len(db.MeltChange))
	}
	for i, want := range []uint64{1, 4, 8} {
		if i >= len(db.RecoverSigDB) || db.RecoverSigDB[i].Amount != want {
			t.Fatalf("change signature %d = %+v, want amount %d", i, db.RecoverSigDB, want)
		}
	}
	if err := mint.ReconcilePendingMeltQuotes(context.Background()); err != nil {
		t.Fatalf("second startup reconciliation: %v", err)
	}
	if signer.calls != 1 {
		t.Fatalf("second startup signed again: %d calls", signer.calls)
	}
}

func TestMeltSettlementReconciliationRejectsMixedProofStates(t *testing.T) {
	db := &mockdb.MockDB{}
	signer := &reconciliationCountingSigner{}
	mint := &Mint{MintDB: db, Signer: signer, LightningBackend: noLightningReconciliationWallet{}}
	setupPendingMeltWithChange(t, mint, "startup-mixed", []uint64{8, 4, 2, 1}, 3)
	db.MeltRequest[0].State, db.MeltRequest[0].Melted = cashu.PAID, true
	db.MeltRequest[0].PaymentPreimage = "durable-preimage"
	db.Proofs[0].State = cashu.PROOF_SPENT
	before := append(cashu.Proofs(nil), db.Proofs...)
	quoteBefore := db.MeltRequest[0]

	if err := mint.ReconcilePendingMeltQuotes(context.Background()); err == nil {
		t.Fatal("mixed proof states reconciled without error")
	}
	for i := range before {
		if db.Proofs[i].State != before[i].State {
			t.Fatalf("proof %d changed from %v to %v", i, before[i].State, db.Proofs[i].State)
		}
	}
	if len(db.MeltChange) != 3 || len(db.RecoverSigDB) != 0 {
		t.Fatalf("mixed proof states changed settlement records: pending blanks %d, signatures %d", len(db.MeltChange), len(db.RecoverSigDB))
	}
	if signer.calls != 0 {
		t.Fatalf("mixed proof states invoked signer %d times", signer.calls)
	}
	if db.MeltRequest[0].State != quoteBefore.State || db.MeltRequest[0].Melted != quoteBefore.Melted || db.MeltRequest[0].FeePaid != quoteBefore.FeePaid || db.MeltRequest[0].PaymentPreimage != quoteBefore.PaymentPreimage {
		t.Fatalf("mixed proof states changed quote: before %+v, after %+v", quoteBefore, db.MeltRequest[0])
	}
}

func TestValidateBolt11MeltInputsRejectsAggregateOverflow(t *testing.T) {
	mint := Mint{}                                                                                      //nolint:exhaustruct_v5 // overflow is rejected before dependencies are used
	request := cashu.PostMeltBolt11Request{Inputs: cashu.Proofs{{Amount: math.MaxUint64}, {Amount: 1}}} //nolint:exhaustruct_v5 // only inputs matter

	_, err := mint.validateBolt11MeltInputs(request, cashu.MeltRequestDB{}) //nolint:exhaustruct_v5 // quote is unused before rejection
	if !errors.Is(err, cashu.ErrNotEnoughtProofs) || !errors.Is(err, cashu.ErrAmountOverflow) {
		t.Fatalf("validateBolt11MeltInputs() error = %v, want insufficient proofs amount overflow", err)
	}
}

func TestValidateBolt11MeltInputsRejectsRequiredTotalOverflow(t *testing.T) {
	keyset := cashu.BasicKeysetResponse{Id: "keyset", Unit: cashu.Sat.String()}                                               //nolint:exhaustruct_v5 // only ID and unit matter
	mint := Mint{Signer: fixedKeysetSigner{keysets: signer.GetKeysetsResponse{Keysets: []cashu.BasicKeysetResponse{keyset}}}} //nolint:exhaustruct_v5 // only signer is reached
	request := cashu.PostMeltBolt11Request{Inputs: cashu.Proofs{{Id: keyset.Id, Amount: math.MaxUint64}}}                     //nolint:exhaustruct_v5 // only inputs matter
	quote := cashu.MeltRequestDB{Amount: math.MaxUint64, FeeReserve: 1}                                                       //nolint:exhaustruct_v5 // only required totals matter

	_, err := mint.validateBolt11MeltInputs(request, quote)
	if !errors.Is(err, cashu.ErrNotEnoughtProofs) || !errors.Is(err, cashu.ErrAmountOverflow) {
		t.Fatalf("validateBolt11MeltInputs() error = %v, want insufficient proofs amount overflow", err)
	}
}

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
	mint := Mint{ //nolint:exhaustruct_v5 // test only needs the LightningBackend; Config has 30+ fields
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
	quote := cashu.MeltRequestDB{ //nolint:exhaustruct_v5 // Only reconciliation fields are relevant.
		Quote: "legacy-pending", Request: RegtestRequest, State: cashu.PENDING,
		Unit: cashu.Sat.String(), CheckingId: "legacy-checking-id",
	}
	db := &mockdb.MockDB{MeltRequest: []cashu.MeltRequestDB{quote}}                                     //nolint:exhaustruct_v5
	mint := Mint{MintDB: db, LightningBackend: lightning.Strike{Network: chaincfg.RegressionNetParams}} //nolint:exhaustruct_v5

	if err := mint.ReconcilePendingMeltQuotes(context.Background()); err != nil {
		t.Fatalf("ReconcilePendingMeltQuotes: %v", err)
	}
	if db.MeltRequest[0].State != cashu.PENDING || db.MeltRequest[0].CheckingId != quote.CheckingId {
		t.Fatalf("pending quote changed: %+v", db.MeltRequest[0])
	}
}

// setupPendingMeltWithChange stores a PENDING melt quote, pending proofs and
// blank change outputs. FakeWallet.CheckPayed settles with a flat 10 sat fee.
func setupPendingMeltWithChange(t *testing.T, mint *Mint, quoteId string, proofAmounts []uint64, numBlanks int) []cashu.WrappedPublicKey {
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
		Amount:          2,
		FeeReserve:      2,
		FeePaid:         0,
		SeenAt:          now,
		Melted:          false,
		Mpp:             false,
		Change:          nil,
	}

	proofs := make(cashu.Proofs, 0, len(proofAmounts))
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
	if err := mint.MintDB.SaveProof(ctx, tx, proofs); err != nil {
		t.Fatalf("SaveProof: %v", err)
	}
	if err := mint.MintDB.ReserveMeltChangeOutputs(ctx, tx, blanks, quoteId); err != nil {
		t.Fatalf("ReserveMeltChangeOutputs: %v", err)
	}
	if err := mint.MintDB.Commit(ctx, tx); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	return blankKeys
}

func TestRefreshMeltQuoteReturnsChangeOnSettle(t *testing.T) {
	mint := SetupMintWithLightningMockPostgres(t)

	// 15 sats of proofs, 2 sat invoice, 10 sat fake fee -> overpaid = 3 -> change [1, 2]
	blankKeys := setupPendingMeltWithChange(t, mint, "quote-change-ok", []uint64{8, 4, 2, 1}, 2)

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
	if quote.Change[0].Amount != 1 || quote.Change[1].Amount != 2 {
		t.Errorf("change amounts should preserve wallet order [1, 2], got: [%d %d]", quote.Change[0].Amount, quote.Change[1].Amount)
	}

	// signatures must have been persisted for both blank outputs
	ctx := context.Background()
	tx, err := mint.MintDB.GetTx(ctx)
	if err != nil {
		t.Fatalf("mint.MintDB.GetTx: %v", err)
	}
	defer func() { _ = mint.MintDB.Rollback(ctx, tx) }()

	restoreSigs, err := mint.MintDB.GetRestoreSigsFromBlindedMessages(ctx, tx, blankKeys)
	if err != nil {
		t.Fatalf("GetRestoreSigsFromBlindedMessages: %v", err)
	}
	if len(restoreSigs) != 2 {
		t.Errorf("expected restore sigs for both blank outputs, got: %d", len(restoreSigs))
	}

	remainingChange, err := mint.MintDB.LoadPendingMeltChangeForUpdate(ctx, tx, "quote-change-ok")
	if err != nil {
		t.Fatalf("LoadPendingMeltChangeForUpdate: %v", err)
	}
	if len(remainingChange) != 0 {
		t.Errorf("signed change must no longer be pending, got: %d", len(remainingChange))
	}
	reloaded, err := mint.MintDB.GetMeltRequestById(ctx, tx, "quote-change-ok")
	if err != nil || len(reloaded.Change) != 2 {
		t.Fatalf("reloaded quote lost change: %+v %v", reloaded, err)
	}
}

func TestRefreshMeltQuoteNoChangeWhenFeeExceedsProofs(t *testing.T) {
	mint := SetupMintWithLightningMockPostgres(t)

	// 4 sats of proofs, 2 sat invoice, 10 sat fake fee -> totalExpent (12) > proofs (4).
	// Without the comparison guard this underflows uint64 and signs huge change.
	setupPendingMeltWithChange(t, mint, "quote-underflow", []uint64{2, 2}, 2)

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
	remainingChange, err := mint.MintDB.LoadPendingMeltChangeForUpdate(ctx, tx, "quote-underflow")
	if err != nil || len(remainingChange) != 0 {
		t.Fatalf("zero-refund settlement retained blanks: %v %v", remainingChange, err)
	}

	proofs, err := mint.MintDB.GetProofsFromQuote(ctx, tx, "quote-underflow")
	if err != nil {
		t.Fatalf("GetProofsFromQuote: %v", err)
	}
	for _, proof := range proofs {
		if proof.State != cashu.PROOF_SPENT {
			t.Errorf("proof should be spent, got: %v", proof.State)
		}
	}
}

func TestMeltFinalizationAuthoritativeStatePostgres(t *testing.T) {
	mint := SetupMintWithLightningMockPostgres(t)
	ctx := context.Background()
	for _, scenario := range []string{"concurrent refund", "concurrent zero refund", "internal preparation"} {
		t.Run(scenario, func(t *testing.T) {
			amounts := []uint64{8, 4, 2, 1}
			wantChange, wantFee := 2, uint64(10)
			if scenario == "concurrent zero refund" {
				amounts, wantChange = []uint64{8, 4}, 0
			}
			if scenario == "internal preparation" {
				wantChange, wantFee = 3, 0
			}
			keys := setupPendingMeltWithChange(t, mint, scenario, amounts, 3) //nolint:contextcheck // Fixture setup intentionally uses its own transaction context.
			tx, err := mint.MintDB.GetTx(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = mint.MintDB.Rollback(ctx, tx) }()
			if scenario == "internal preparation" {
				if err := mint.MintDB.ChangeMeltRequestState(ctx, tx, scenario, cashu.PAID, true, 0); err != nil {
					t.Fatal(err)
				}
			}
			cached, err := mint.MintDB.GetMeltRequestById(ctx, tx, scenario)
			if err != nil {
				t.Fatal(err)
			}
			inputs, err := mint.MintDB.GetProofsFromQuote(ctx, tx, scenario)
			if err != nil {
				t.Fatal(err)
			}
			if err := mint.MintDB.Commit(ctx, tx); err != nil {
				t.Fatal(err)
			}
			if scenario != "internal preparation" {
				// Force the quote-poll commit before the original HTTP finalizer.
				if _, err := mint.RefreshMeltQuoteState(ctx, scenario); err != nil {
					t.Fatal(err)
				}
			}
			cached.State, cached.Melted, cached.FeePaid, cached.PaymentPreimage = cashu.PAID, true, 99, "stale-preimage"
			amount, err := inputs.Amount()
			if err != nil {
				t.Fatal(err)
			}
			for range 2 {
				quote, response, spent, err := mint.finalizePaidBolt11Melt(ctx, bolt11MeltData{
					AmountProofs: cashu.NewAmount(cashu.Sat, amount), Fee: cashu.NewAmount(cashu.Sat, 0), Unit: cashu.Sat,
				}, cashu.PostMeltBolt11Request{Quote: scenario, Inputs: inputs, Outputs: nil}, cached, cashu.NewAmount(cashu.Sat, 99))
				if err != nil {
					t.Fatal(err)
				}
				if len(response.Change) != wantChange || quote.FeePaid != wantFee || quote.PaymentPreimage == "stale-preimage" {
					t.Fatalf("wrong authoritative response: %+v", quote)
				}
				for _, proof := range spent {
					if proof.State != cashu.PROOF_SPENT {
						t.Fatal("proof remains pending")
					}
				}
			}
			read, err := mint.MintDB.GetTx(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = mint.MintDB.Rollback(ctx, read) }()
			restored, err := mint.MintDB.GetRestoreSigsFromBlindedMessages(ctx, read, keys)
			if err != nil || len(restored) != wantChange {
				t.Fatalf("refund lost or duplicated: %v %v", restored, err)
			}
			if err := mint.MintDB.Commit(ctx, read); err != nil {
				t.Fatal(err)
			}
		})
	}
}
