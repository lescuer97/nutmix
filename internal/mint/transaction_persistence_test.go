package mint

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"testing"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/lightning"
	"github.com/lescuer97/nutmix/internal/signer"
	internalutils "github.com/lescuer97/nutmix/internal/utils"
	nutcrypto "github.com/lescuer97/nutmix/pkg/crypto"
	"github.com/lightningnetwork/lnd/zpay32"
)

type quoteAmountBackend struct {
	lightning.FakeWallet
	feesResponse lightning.FeesResponse
}

func (b quoteAmountBackend) QueryFees(invoice string, zpayInvoice *zpay32.Invoice, mpp bool, amount cashu.Amount) (lightning.FeesResponse, error) {
	return b.feesResponse, nil
}

func createMintTestBlindedMessagesWithSecrets(t *testing.T, amount uint64, activeKeys signer.GetKeysResponse) (cashu.BlindedMessages, []string, []*secp256k1.PrivateKey) {
	t.Helper()

	splitAmounts := cashu.AmountSplit(amount)
	blindedMessages := make(cashu.BlindedMessages, len(splitAmounts))
	secrets := make([]string, len(splitAmounts))
	blindingFactors := make([]*secp256k1.PrivateKey, len(splitAmounts))

	for i, amt := range splitAmounts {
		r, err := secp256k1.GeneratePrivateKey()
		if err != nil {
			t.Fatalf("secp256k1.GeneratePrivateKey(): %v", err)
		}
		blindingFactors[i] = r

		for {
			secretBytes := make([]byte, 32)
			_, err = crand.Read(secretBytes)
			if err != nil {
				t.Fatalf("rand.Read(secretBytes): %v", err)
			}

			secret := hex.EncodeToString(secretBytes)
			B_, _, blindErr := nutcrypto.BlindMessage(secret, r)
			if blindErr == nil {
				blindedMessages[i] = cashu.BlindedMessage{
					Amount:  amt,
					B_:      cashu.WrappedPublicKey{PublicKey: B_},
					Id:      activeKeys.Keysets[0].Id,
					Witness: "",
				}
				secrets[i] = secret
				break
			}
		}
	}

	return blindedMessages, secrets, blindingFactors
}

func createMintTestBlindedMessages(t *testing.T, amount uint64, activeKeys signer.GetKeysResponse) cashu.BlindedMessages {
	blindedMessages, _, _ := createMintTestBlindedMessagesWithSecrets(t, amount, activeKeys)
	return blindedMessages
}

func createSpendableProofs(t *testing.T, mint *Mint, amount uint64, activeKeys signer.GetKeysResponse) cashu.Proofs {
	t.Helper()

	blindedMessages, secrets, blindingFactors := createMintTestBlindedMessagesWithSecrets(t, amount, activeKeys)
	blindSignatures, _, err := mint.Signer.SignBlindMessages(blindedMessages)
	if err != nil {
		t.Fatalf("mint.Signer.SignBlindMessages(blindedMessages): %v", err)
	}

	proofs := make(cashu.Proofs, len(blindSignatures))
	for i, blindSignature := range blindSignatures {
		pubkeyStr := activeKeys.Keysets[0].Keys[blindSignature.Amount]
		pubkeyBytes, err := hex.DecodeString(pubkeyStr)
		if err != nil {
			t.Fatalf("hex.DecodeString(pubkeyStr): %v", err)
		}

		mintPublicKey, err := secp256k1.ParsePubKey(pubkeyBytes)
		if err != nil {
			t.Fatalf("secp256k1.ParsePubKey(pubkeyBytes): %v", err)
		}

		C := nutcrypto.UnblindSignature(blindSignature.C_.PublicKey, blindingFactors[i], mintPublicKey)
		proofs[i] = cashu.Proof{
			Id:      blindSignature.Id,
			Amount:  blindSignature.Amount,
			C:       cashu.WrappedPublicKey{PublicKey: C},
			Secret:  secrets[i],
			Y:       cashu.WrappedPublicKey{PublicKey: nil},
			Quote:   nil,
			Witness: "",
			State:   cashu.PROOF_UNSPENT,
			SeenAt:  0,
		}
	}

	return proofs
}

func createMeltTestProofs(t *testing.T, amount uint64, activeKeys signer.GetKeysResponse) cashu.Proofs {
	t.Helper()

	splitAmounts := cashu.AmountSplit(amount)
	proofs := make(cashu.Proofs, len(splitAmounts))

	for i, amt := range splitAmounts {
		secretBytes := make([]byte, 32)
		_, err := crand.Read(secretBytes)
		if err != nil {
			t.Fatalf("rand.Read(secretBytes): %v", err)
		}

		cPriv, err := secp256k1.GeneratePrivateKey()
		if err != nil {
			t.Fatalf("secp256k1.GeneratePrivateKey(): %v", err)
		}

		proof := cashu.Proof{
			Amount:  amt,
			C:       cashu.WrappedPublicKey{PublicKey: cPriv.PubKey()},
			Id:      activeKeys.Keysets[0].Id,
			Secret:  hex.EncodeToString(secretBytes),
			Y:       cashu.WrappedPublicKey{PublicKey: nil},
			Quote:   nil,
			Witness: "",
			State:   cashu.PROOF_UNSPENT,
			SeenAt:  0,
		}

		proof, err = proof.HashSecretToCurve()
		if err != nil {
			t.Fatalf("proof.HashSecretToCurve(): %v", err)
		}

		proofs[i] = proof
	}

	return proofs
}

func TestSignAndSaveSigsPersistsIssuedState(t *testing.T) {
	mint := SetupMintWithLightningMockPostgres(t)
	ctx := context.Background()

	activeKeys, err := mint.Signer.GetActiveKeys()
	if err != nil {
		t.Fatalf("mint.Signer.GetActiveKeys(): %v", err)
	}

	amount := uint64(2)
	mintRequest := cashu.MintRequestDB{
		Amount:      &amount,
		Pubkey:      cashu.WrappedPublicKey{PublicKey: nil},
		Description: nil,
		Quote:       "sign-save-quote",
		Request:     RegtestRequest,
		Unit:        cashu.Sat.String(),
		State:       cashu.PAID,
		CheckingId:  "check-sign-save",
		Expiry:      time.Now().Add(time.Minute).Unix(),
		SeenAt:      time.Now().Unix(),
		Minted:      false,
	}

	tx, err := mint.MintDB.GetTx(ctx)
	if err != nil {
		t.Fatalf("mint.MintDB.GetTx(ctx): %v", err)
	}
	defer func() {
		_ = mint.MintDB.Rollback(ctx, tx)
	}()

	err = mint.MintDB.SaveMintRequest(tx, mintRequest)
	if err != nil {
		t.Fatalf("mint.MintDB.SaveMintRequest(tx, mintRequest): %v", err)
	}
	err = mint.MintDB.Commit(ctx, tx)
	if err != nil {
		t.Fatalf("mint.MintDB.Commit(ctx, tx): %v", err)
	}

	outputs := createMintTestBlindedMessages(t, amount, activeKeys)
	_, err = mint.signAndSaveSigs(ctx, cashu.PostMintBolt11Request{Signature: nil, Quote: mintRequest.Quote, Outputs: outputs}, mintRequest)
	if err != nil {
		t.Fatalf("mint.signAndSaveSigs(ctx, request, mintRequest): %v", err)
	}

	tx, err = mint.MintDB.GetTx(ctx)
	if err != nil {
		t.Fatalf("mint.MintDB.GetTx(ctx): %v", err)
	}
	defer func() {
		_ = mint.MintDB.Rollback(ctx, tx)
	}()

	savedMintRequest, err := mint.MintDB.GetMintRequestById(tx, mintRequest.Quote)
	if err != nil {
		t.Fatalf("mint.MintDB.GetMintRequestById(tx, mintRequest.Quote): %v", err)
	}

	if savedMintRequest.State != cashu.ISSUED {
		t.Fatalf("expected saved mint request to be ISSUED, got %v", savedMintRequest.State)
	}
	if !savedMintRequest.Minted {
		t.Fatalf("expected saved mint request to be marked minted")
	}

	err = mint.MintDB.Commit(ctx, tx)
	if err != nil {
		t.Fatalf("mint.MintDB.Commit(ctx, tx): %v", err)
	}
}

func TestSettleIfInternalMeltPersistsMintRequestState(t *testing.T) {
	mint := SetupMintWithLightningMockPostgres(t)
	ctx := context.Background()

	amount := uint64(2)
	mintRequest := cashu.MintRequestDB{
		Amount:      &amount,
		Pubkey:      cashu.WrappedPublicKey{PublicKey: nil},
		Description: nil,
		Quote:       "internal-mint-quote",
		Request:     RegtestRequest,
		Unit:        cashu.Sat.String(),
		State:       cashu.UNPAID,
		CheckingId:  "internal-mint-check",
		Expiry:      time.Now().Add(time.Minute).Unix(),
		SeenAt:      time.Now().Unix(),
		Minted:      false,
	}
	meltQuote := cashu.MeltRequestDB{
		Amount:          amount,
		Quote:           "internal-melt-quote",
		Request:         RegtestRequest,
		Unit:            cashu.Sat.String(),
		Expiry:          time.Now().Add(time.Minute).Unix(),
		FeeReserve:      0,
		State:           cashu.PENDING,
		PaymentPreimage: "",
		SeenAt:          time.Now().Unix(),
		Mpp:             false,
		FeePaid:         0,
		Melted:          false,
		CheckingId:      "internal-melt-check",
	}

	tx, err := mint.MintDB.GetTx(ctx)
	if err != nil {
		t.Fatalf("mint.MintDB.GetTx(ctx): %v", err)
	}
	defer func() {
		_ = mint.MintDB.Rollback(ctx, tx)
	}()

	err = mint.MintDB.SaveMintRequest(tx, mintRequest)
	if err != nil {
		t.Fatalf("mint.MintDB.SaveMintRequest(tx, mintRequest): %v", err)
	}
	err = mint.MintDB.SaveMeltRequest(tx, meltQuote)
	if err != nil {
		t.Fatalf("mint.MintDB.SaveMeltRequest(tx, meltQuote): %v", err)
	}

	settledQuote, err := mint.settleIfInternalMelt(tx, meltQuote)
	if err != nil {
		t.Fatalf("mint.settleIfInternalMelt(tx, meltQuote): %v", err)
	}
	if settledQuote.State != cashu.PAID {
		t.Fatalf("expected settled quote state to be PAID, got %v", settledQuote.State)
	}
	if !settledQuote.Melted {
		t.Fatalf("expected settled quote to be marked melted")
	}

	err = mint.MintDB.Commit(ctx, tx)
	if err != nil {
		t.Fatalf("mint.MintDB.Commit(ctx, tx): %v", err)
	}

	tx, err = mint.MintDB.GetTx(ctx)
	if err != nil {
		t.Fatalf("mint.MintDB.GetTx(ctx): %v", err)
	}
	defer func() {
		_ = mint.MintDB.Rollback(ctx, tx)
	}()

	savedMintRequest, err := mint.MintDB.GetMintRequestById(tx, mintRequest.Quote)
	if err != nil {
		t.Fatalf("mint.MintDB.GetMintRequestById(tx, mintRequest.Quote): %v", err)
	}
	if savedMintRequest.State != cashu.PAID {
		t.Fatalf("expected mint request state to be PAID, got %v", savedMintRequest.State)
	}

	savedMeltQuote, err := mint.MintDB.GetMeltRequestById(tx, meltQuote.Quote)
	if err != nil {
		t.Fatalf("mint.MintDB.GetMeltRequestById(tx, meltQuote.Quote): %v", err)
	}
	if savedMeltQuote.State != cashu.PAID {
		t.Fatalf("expected melt quote state to be PAID, got %v", savedMeltQuote.State)
	}
	if !savedMeltQuote.Melted {
		t.Fatalf("expected melt quote to be marked melted")
	}

	err = mint.MintDB.Commit(ctx, tx)
	if err != nil {
		t.Fatalf("mint.MintDB.Commit(ctx, tx): %v", err)
	}
}

func TestValidateMeltStatusAndSpentPersistsProofQuoteReference(t *testing.T) {
	mint := SetupMintWithLightningMockPostgres(t)
	ctx := context.Background()

	activeKeys, err := mint.Signer.GetActiveKeys()
	if err != nil {
		t.Fatalf("mint.Signer.GetActiveKeys(): %v", err)
	}

	meltQuote := cashu.MeltRequestDB{
		Amount:          2,
		Quote:           "pending-proof-quote",
		Request:         RegtestRequest,
		Unit:            cashu.Sat.String(),
		Expiry:          time.Now().Add(time.Minute).Unix(),
		FeeReserve:      1,
		State:           cashu.UNPAID,
		PaymentPreimage: "",
		SeenAt:          time.Now().Unix(),
		Mpp:             false,
		CheckingId:      "pending-proof-check",
		FeePaid:         0,
		Melted:          false,
	}

	tx, err := mint.MintDB.GetTx(ctx)
	if err != nil {
		t.Fatalf("mint.MintDB.GetTx(ctx): %v", err)
	}
	defer func() {
		_ = mint.MintDB.Rollback(ctx, tx)
	}()

	err = mint.MintDB.SaveMeltRequest(tx, meltQuote)
	if err != nil {
		t.Fatalf("mint.MintDB.SaveMeltRequest(tx, meltQuote): %v", err)
	}

	err = mint.MintDB.Commit(ctx, tx)
	if err != nil {
		t.Fatalf("mint.MintDB.Commit(ctx, tx): %v", err)
	}

	request := cashu.PostMeltBolt11Request{
		Quote:   meltQuote.Quote,
		Inputs:  createMeltTestProofs(t, 4, activeKeys),
		Outputs: createMintTestBlindedMessages(t, 1, activeKeys),
	}

	savedQuote, err := mint.validateMeltStatusAndSpent(ctx, request)
	if err != nil {
		t.Fatalf("mint.validateMeltStatusAndSpent(ctx, request): %v", err)
	}

	if savedQuote.State != cashu.PENDING {
		t.Fatalf("expected saved quote state to be PENDING, got %v", savedQuote.State)
	}

	tx, err = mint.MintDB.GetTx(ctx)
	if err != nil {
		t.Fatalf("mint.MintDB.GetTx(ctx): %v", err)
	}
	defer func() {
		_ = mint.MintDB.Rollback(ctx, tx)
	}()

	savedProofs, err := mint.MintDB.GetProofsFromQuote(tx, meltQuote.Quote)
	if err != nil {
		t.Fatalf("mint.MintDB.GetProofsFromQuote(tx, meltQuote.Quote): %v", err)
	}

	if len(savedProofs) != len(request.Inputs) {
		t.Fatalf("expected %d saved proofs, got %d", len(request.Inputs), len(savedProofs))
	}

	for _, proof := range savedProofs {
		if proof.Quote == nil || *proof.Quote != meltQuote.Quote {
			t.Fatalf("expected proof quote reference %q, got %+v", meltQuote.Quote, proof.Quote)
		}
		if proof.State != cashu.PROOF_PENDING {
			t.Fatalf("expected proof state PENDING, got %v", proof.State)
		}
	}

	err = mint.MintDB.Commit(ctx, tx)
	if err != nil {
		t.Fatalf("mint.MintDB.Commit(ctx, tx): %v", err)
	}
}

func TestMeltQuoteUsesBackendAmountToSend(t *testing.T) {
	mint := SetupMintWithLightningMockPostgres(t)
	mint.LightningBackend = quoteAmountBackend{
		FakeWallet: lightning.FakeWallet{
			Network:         *mint.LightningBackend.GetNetwork(),
			UnpurposeErrors: []lightning.FakeWalletError{},
			InvoiceFee:      0,
		},
		feesResponse: lightning.FeesResponse{
			CheckingId:   "backend-checking-id",
			Fees:         cashu.NewAmount(cashu.Sat, 0),
			AmountToSend: cashu.NewAmount(cashu.Sat, 2),
		},
	}

	quote, err := mint.MeltQuote(context.Background(), cashu.PostMeltQuoteBolt11Request{
		Options: cashu.PostMeltQuoteBolt11Options{Mpp: nil},
		Request: RegtestRequest,
		Unit:    cashu.Sat.String(),
	}, Bolt11)
	if err != nil {
		t.Fatalf("mint.MeltQuote(context.Background(), request, Bolt11): %v", err)
	}

	if quote.Amount != 2 {
		t.Fatalf("expected melt quote amount to use backend amount 2, got %d", quote.Amount)
	}
	if quote.CheckingId != "backend-checking-id" {
		t.Fatalf("expected checking id to use backend response, got %q", quote.CheckingId)
	}
}

func TestSignAndSaveSigsSendsMintEvent(t *testing.T) {
	mint := SetupMintWithLightningMockPostgres(t)
	ctx := context.Background()

	activeKeys, err := mint.Signer.GetActiveKeys()
	if err != nil {
		t.Fatalf("mint.Signer.GetActiveKeys(): %v", err)
	}

	amount := uint64(2)
	mintRequest := cashu.MintRequestDB{
		Amount:      &amount,
		Pubkey:      cashu.WrappedPublicKey{PublicKey: nil},
		Description: nil,
		Quote:       "mint-event-quote",
		Request:     RegtestRequest,
		Unit:        cashu.Sat.String(),
		State:       cashu.PAID,
		CheckingId:  "mint-event-check",
		Expiry:      time.Now().Add(time.Minute).Unix(),
		SeenAt:      time.Now().Unix(),
		Minted:      false,
	}

	tx, err := mint.MintDB.GetTx(ctx)
	if err != nil {
		t.Fatalf("mint.MintDB.GetTx(ctx): %v", err)
	}
	defer func() {
		_ = mint.MintDB.Rollback(ctx, tx)
	}()

	err = mint.MintDB.SaveMintRequest(tx, mintRequest)
	if err != nil {
		t.Fatalf("mint.MintDB.SaveMintRequest(tx, mintRequest): %v", err)
	}
	err = mint.MintDB.Commit(ctx, tx)
	if err != nil {
		t.Fatalf("mint.MintDB.Commit(ctx, tx): %v", err)
	}

	mintChan := make(chan cashu.MintRequestDB, 1)
	mint.Observer.AddMintWatch(mintRequest.Quote, MintQuoteChannel{SubId: "mint-event", Channel: mintChan})

	outputs := createMintTestBlindedMessages(t, amount, activeKeys)
	_, err = mint.signAndSaveSigs(ctx, cashu.PostMintBolt11Request{Signature: nil, Quote: mintRequest.Quote, Outputs: outputs}, mintRequest)
	if err != nil {
		t.Fatalf("mint.signAndSaveSigs(ctx, request, mintRequest): %v", err)
	}

	select {
	case observedMint := <-mintChan:
		if observedMint.Quote != mintRequest.Quote {
			t.Fatalf("expected mint event quote %q, got %q", mintRequest.Quote, observedMint.Quote)
		}
		if observedMint.State != cashu.ISSUED {
			t.Fatalf("expected mint event state ISSUED, got %v", observedMint.State)
		}
		if !observedMint.Minted {
			t.Fatalf("expected mint event to be marked minted")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for mint event")
	}
}

func TestBolt11MeltSendsSuccessEvents(t *testing.T) {
	mint := SetupMintWithLightningMockPostgres(t)
	ctx := context.Background()

	activeKeys, err := mint.Signer.GetActiveKeys()
	if err != nil {
		t.Fatalf("mint.Signer.GetActiveKeys(): %v", err)
	}

	proofs := createSpendableProofs(t, mint, 4, activeKeys)
	proofsForWatch := append(cashu.Proofs(nil), proofs...)
	_, err = internalutils.GetAndCalculateProofsValues(&proofsForWatch)
	if err != nil {
		t.Fatalf("internalutils.GetAndCalculateProofsValues(&proofsForWatch): %v", err)
	}

	mintAmount := uint64(2)
	mintRequest := cashu.MintRequestDB{
		Amount:      &mintAmount,
		Pubkey:      cashu.WrappedPublicKey{PublicKey: nil},
		Description: nil,
		Quote:       "success-event-mint-quote",
		Request:     RegtestRequest,
		Unit:        cashu.Sat.String(),
		State:       cashu.UNPAID,
		CheckingId:  "success-event-mint-check",
		Expiry:      time.Now().Add(time.Minute).Unix(),
		SeenAt:      time.Now().Unix(),
		Minted:      false,
	}
	meltQuote := cashu.MeltRequestDB{
		Amount:          mintAmount,
		Quote:           "success-event-melt-quote",
		Request:         RegtestRequest,
		Unit:            cashu.Sat.String(),
		Expiry:          time.Now().Add(time.Minute).Unix(),
		FeeReserve:      0,
		State:           cashu.UNPAID,
		PaymentPreimage: "",
		SeenAt:          time.Now().Unix(),
		Mpp:             false,
		FeePaid:         0,
		Melted:          false,
		CheckingId:      "success-event-melt-check",
	}

	tx, err := mint.MintDB.GetTx(ctx)
	if err != nil {
		t.Fatalf("mint.MintDB.GetTx(ctx): %v", err)
	}
	defer func() {
		_ = mint.MintDB.Rollback(ctx, tx)
	}()

	err = mint.MintDB.SaveMintRequest(tx, mintRequest)
	if err != nil {
		t.Fatalf("mint.MintDB.SaveMintRequest(tx, mintRequest): %v", err)
	}
	err = mint.MintDB.SaveMeltRequest(tx, meltQuote)
	if err != nil {
		t.Fatalf("mint.MintDB.SaveMeltRequest(tx, meltQuote): %v", err)
	}
	err = mint.MintDB.Commit(ctx, tx)
	if err != nil {
		t.Fatalf("mint.MintDB.Commit(ctx, tx): %v", err)
	}

	proofChan := make(chan cashu.Proof, len(proofsForWatch))
	meltChan := make(chan cashu.MeltRequestDB, 1)
	for _, proof := range proofsForWatch {
		mint.Observer.AddProofWatch(proof.Y.ToHex(), ProofWatchChannel{SubId: "melt-proof-event", Channel: proofChan})
	}
	mint.Observer.AddMeltWatch(meltQuote.Quote, MeltQuoteChannel{SubId: "melt-quote-event", Channel: meltChan})

	quote, response, err := mint.bolt11Melt(ctx, cashu.PostMeltBolt11Request{Quote: meltQuote.Quote, Inputs: proofs, Outputs: nil})
	if err != nil {
		t.Fatalf("mint.bolt11Melt(ctx, request): %v", err)
	}

	if quote.State != cashu.PAID {
		t.Fatalf("expected melt quote state PAID, got %v", quote.State)
	}
	if response.State != cashu.PAID {
		t.Fatalf("expected melt response state PAID, got %v", response.State)
	}

	select {
	case observedMelt := <-meltChan:
		if observedMelt.Quote != meltQuote.Quote {
			t.Fatalf("expected melt event quote %q, got %q", meltQuote.Quote, observedMelt.Quote)
		}
		if observedMelt.State != cashu.PAID {
			t.Fatalf("expected melt event state PAID, got %v", observedMelt.State)
		}
		if !observedMelt.Melted {
			t.Fatalf("expected melt event to be marked melted")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for melt event")
	}

	for i := 0; i < len(proofsForWatch); i++ {
		select {
		case observedProof := <-proofChan:
			if observedProof.State != cashu.PROOF_SPENT {
				t.Fatalf("expected proof event state SPENT, got %v", observedProof.State)
			}
			if observedProof.Y.PublicKey == nil {
				t.Fatal("expected proof event Y to be populated")
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for proof event")
		}
	}
}
