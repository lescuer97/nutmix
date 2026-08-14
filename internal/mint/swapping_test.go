package mint

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/signer"
	internalutils "github.com/lescuer97/nutmix/internal/utils"
)

type fixedKeysetSigner struct {
	signer.Signer
	keysets signer.GetKeysetsResponse
}

func (s fixedKeysetSigner) GetKeysets() (signer.GetKeysetsResponse, error) {
	return s.keysets, nil
}

func TestValidateSwapBalanceRejectsModularBypass(t *testing.T) {
	keyset := cashu.BasicKeysetResponse{Id: "keyset", Unit: cashu.Sat.String(), InputFeePpk: 1000}                            //nolint:exhaustruct // only balance fields matter
	mint := Mint{Signer: fixedKeysetSigner{keysets: signer.GetKeysetsResponse{Keysets: []cashu.BasicKeysetResponse{keyset}}}} //nolint:exhaustruct // only signer is reached
	request := cashu.PostSwapRequest{
		Inputs:  cashu.Proofs{{Id: keyset.Id, Amount: 0}},                       //nolint:exhaustruct // only balance fields matter
		Outputs: cashu.BlindedMessages{{Id: keyset.Id, Amount: math.MaxUint64}}, //nolint:exhaustruct // only balance fields matter
	}

	err := mint.validateSwapBalanceAndUnits(request)
	if !errors.Is(err, cashu.ErrUnbalanced) || !errors.Is(err, cashu.ErrAmountOverflow) {
		t.Fatalf("validateSwapBalanceAndUnits() error = %v, want unbalanced amount overflow", err)
	}
}

type swapFailingDB struct {
	database.MintDB
	saveRestoreSigsErr error
	setProofsStateErr  error
}

func (s swapFailingDB) SaveRestoreSigs(tx pgx.Tx, recoverSigs []cashu.RecoverSigDB) error {
	if s.saveRestoreSigsErr != nil {
		return s.saveRestoreSigsErr
	}

	return s.MintDB.SaveRestoreSigs(tx, recoverSigs)
}

func (s swapFailingDB) SetProofsState(tx pgx.Tx, proofs cashu.Proofs, state cashu.ProofState) error {
	if s.setProofsStateErr != nil {
		return s.setProofsStateErr
	}

	return s.MintDB.SetProofsState(tx, proofs, state)
}

type failingSigner struct {
	signer.Signer
	signBlindMessagesErr error
}

func (s failingSigner) SignBlindMessages(messages []cashu.BlindedMessage) ([]cashu.BlindSignature, []cashu.RecoverSigDB, error) {
	if s.signBlindMessagesErr != nil {
		return nil, nil, s.signBlindMessagesErr
	}

	return s.Signer.SignBlindMessages(messages)
}

func TestExecuteSwapRemovesPendingProofsWhenSaveRestoreSigsFails(t *testing.T) {
	runSwapCleanupFailureTest(t, "SaveRestoreSigs", func(mint *Mint) {
		mint.MintDB = swapFailingDB{MintDB: mint.MintDB, saveRestoreSigsErr: errors.New("save restore sigs failed"), setProofsStateErr: nil}
	})
}

func TestExecuteSwapRemovesPendingProofsWhenSetProofsStateFails(t *testing.T) {
	runSwapCleanupFailureTest(t, "SetProofsState", func(mint *Mint) {
		mint.MintDB = swapFailingDB{MintDB: mint.MintDB, saveRestoreSigsErr: nil, setProofsStateErr: errors.New("set proofs state failed")}
	})
}

func TestExecuteSwapRemovesPendingProofsWhenSigningFails(t *testing.T) {
	runSwapCleanupFailureTest(t, "SignBlindMessages", func(mint *Mint) {
		mint.Signer = failingSigner{Signer: mint.Signer, signBlindMessagesErr: errors.New("sign outputs failed")}
	})
}

func runSwapCleanupFailureTest(t *testing.T, expectedErr string, mutateMint func(*Mint)) {
	t.Helper()

	mint := SetupMintWithLightningMockPostgres(t)
	activeKeys, err := mint.Signer.GetActiveKeys()
	if err != nil {
		t.Fatalf("mint.Signer.GetActiveKeys(): %v", err)
	}

	inputs := createSpendableProofs(t, mint, 4, activeKeys)
	proofYs, err := internalProofYs(inputs)
	if err != nil {
		t.Fatalf("internalProofYs(inputs): %v", err)
	}

	mutateMint(mint)

	request := cashu.PostSwapRequest{
		Inputs:  inputs,
		Outputs: createMintTestBlindedMessages(t, 4, activeKeys),
	}

	_, err = mint.ExecuteSwap(context.Background(), request)
	if err == nil {
		t.Fatal("expected ExecuteSwap to fail")
	}
	if !strings.Contains(err.Error(), expectedErr) {
		t.Fatalf("expected %s failure, got %v", expectedErr, err)
	}

	assertProofsMissing(t, mint, proofYs)
}

func internalProofYs(proofs cashu.Proofs) ([]cashu.WrappedPublicKey, error) {
	proofCopy := append(cashu.Proofs(nil), proofs...)
	return internalutils.GetAndCalculateProofsValues(&proofCopy)
}

func assertProofsMissing(t *testing.T, mint *Mint, proofYs []cashu.WrappedPublicKey) {
	t.Helper()

	tx, err := mint.MintDB.GetTx(context.Background())
	if err != nil {
		t.Fatalf("mint.MintDB.GetTx(context.Background()): %v", err)
	}
	defer func() {
		_ = mint.MintDB.Rollback(context.Background(), tx)
	}()

	proofs, err := mint.MintDB.GetProofsFromSecretCurve(tx, proofYs)
	if err != nil {
		t.Fatalf("mint.MintDB.GetProofsFromSecretCurve(tx, proofYs): %v", err)
	}
	if len(proofs) != 0 {
		t.Fatalf("expected pending proofs cleanup, found %d proofs", len(proofs))
	}

	err = mint.MintDB.Commit(context.Background(), tx)
	if err != nil {
		t.Fatalf("mint.MintDB.Commit(context.Background(), tx): %v", err)
	}
}
