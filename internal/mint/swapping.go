package mint

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/utils"
)

func (m *Mint) Swap(ctx context.Context, request cashu.PostSwapRequest) (cashu.PostSwapResponse, error) {
	amountValidationErr := m.swapRequestValidateAmount(request)
	if amountValidationErr != nil {
		return cashu.PostSwapResponse{}, fmt.Errorf("m.validateInputAndOutput(request.Inputs, request.Outputs). %w", amountValidationErr)
	}

	// validate sig all
	err := m.validateSwapRequest(request)
	if err != nil {
		return cashu.PostSwapResponse{}, fmt.Errorf("m.validateInputAndOutput(request.Inputs, request.Outputs). %w", err)
	}

	// check if proofs are spent and if outputs are spent
	sizeCheckTx, err := m.MintDB.GetTx(ctx)
	if err != nil {
		return cashu.PostSwapResponse{}, fmt.Errorf("m.MintDB.GetTx(ctx). %w", err)
	}
	defer func() {
		if err != nil {
			rollbackErr := m.MintDB.Rollback(ctx, sizeCheckTx)
			if rollbackErr != nil {
				slog.Warn("rollback error", slog.Any("error", rollbackErr))
			}
		}
	}()

	proofs, err := m.checkProofSpent(sizeCheckTx, request.Inputs)
	if err != nil {
		return cashu.PostSwapResponse{}, fmt.Errorf("m.checkProofSpent(sizeCheckTx, request.Inputs). %w", err)
	}

	err = m.CheckOutputSpent(sizeCheckTx, request.Outputs)
	if err != nil {
		return cashu.PostSwapResponse{}, fmt.Errorf("m.checkOutputSpent(sizeCheckTx, request.Outputs). %w", err)
	}

	proofs.SetProofsState(cashu.PROOF_PENDING)
	err = m.MintDB.SaveProof(sizeCheckTx, proofs)
	if err != nil {
		return cashu.PostSwapResponse{}, fmt.Errorf("m.checkOutputSpent(sizeCheckTx, request.Outputs). %w", err)
	}

	err = sizeCheckTx.Commit(ctx)
	if err != nil {
		return cashu.PostSwapResponse{}, fmt.Errorf("m.checkOutputSpent(sizeCheckTx, request.Outputs). %w", err)
	}

	blindSignatures, err := m.signAndSetInputs(ctx, proofs, request)
	if err != nil {
		return cashu.PostSwapResponse{}, fmt.Errorf("m.checkOutputSpent(sizeCheckTx, request.Outputs). %w", err)
	}

	// mark as pending and sign
	return cashu.PostSwapResponse{
		Signatures: blindSignatures,
	}, nil
}

func (m *Mint) signAndSetInputs(ctx context.Context, inputs cashu.Proofs, swapRequest cashu.PostSwapRequest) ([]cashu.BlindSignature, error) {
	// sign the outputs
	blindedSignatures, recoverySigsDb, err := m.Signer.SignBlindMessages(swapRequest.Outputs)
	if err != nil {
		return nil, fmt.Errorf("m.Signer.SignBlindMessages(outputs). %w", err)
	}

	afterSigningTx, err := m.MintDB.GetTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("m.checkOutputSpent(sizeCheckTx, request.Outputs). %w", err)
	}
	defer func() {
		rollbackErr := m.MintDB.Rollback(ctx, afterSigningTx)
		if rollbackErr != nil {
			if !errors.Is(rollbackErr, pgx.ErrTxClosed) {
				slog.Warn("could not swap state", slog.Any("error", rollbackErr))
			}
		}
	}()
	err = m.MintDB.SetProofsState(afterSigningTx, inputs, cashu.PROOF_SPENT)
	if err != nil {
		return nil, fmt.Errorf("m.MintDB.SetProofsState(afterSigningTx, inputs, cashu.PROOF_SPENT). %w", err)
	}
	err = m.MintDB.SaveRestoreSigs(afterSigningTx, recoverySigsDb)
	if err != nil {
		return nil, fmt.Errorf("m.MintDB.SaveRestoreSigs(afterSigningTx, recoverySigsDb). %w", err)
	}
	err = m.MintDB.Commit(ctx, afterSigningTx)
	if err != nil {
		return nil, fmt.Errorf("m.MintDB.Commit(ctx, afterSigningTx). %w", err)
	}
	return blindedSignatures, nil
}

func (m *Mint) validateSwapRequest(request cashu.PostSwapRequest) error {
	// validate if the proofs are correctly signed
	err := m.VerifyProofsBDHKE(request.Inputs)
	if err != nil {
		return fmt.Errorf("m.VerifyProofsBDHKE(proofs). %w", err)
	}

	// Verify spending conditions - EXCLUSIVE paths following CDK pattern
	hasSigAll, err := cashu.ProofsHaveSigAll(request.Inputs)
	if err != nil {
		return fmt.Errorf("cashu.ProofsHaveSigAll(inputs). %w", err)
	}

	if hasSigAll {
		// SIG_ALL path: verify all conditions match and signature is valid against combined message
		err = request.ValidateSigflag()
		if err != nil {
			return fmt.Errorf("request.ValidateSigflag(). %w", err)
		}
	} else {
		// Individual verification path: verify each proof's P2PK/HTLC spend conditions
		err = cashu.VerifyProofsSpendConditions(request.Inputs)
		if err != nil {
			return fmt.Errorf("cashu.VerifyProofsSpendConditions(request.Inputs). %w", err)
		}
	}
	return nil
}

func (m *Mint) swapRequestValidateAmount(request cashu.PostSwapRequest) error {
	if len(request.Inputs) == 0 || len(request.Outputs) == 0 {
		return fmt.Errorf("inputs or outputs are empty")
	}
	proofsAmount := request.Inputs.Amount()
	blindMessageAmount := request.Outputs.Amount()

	keysets, err := m.Signer.GetKeysets()
	if err != nil {
		return err
	}

	// check for needed amount of fees
	fee, err := cashu.Fees(request.Inputs, keysets.Keysets)
	if err != nil {
		return fmt.Errorf("cashu.Fees(request.Inputs, keysets.Keysets). %w", err)
	}

	balance := (proofsAmount - (uint64(fee) + blindMessageAmount))
	if balance != 0 {
		return fmt.Errorf("(proofs.Amount() - (uint64(fee) + AmountSignature)). %w", cashu.ErrUnbalanced)
	}

	// get unit from proofs
	proofUnit, err := checkProofsAreSameUnit(request.Inputs, keysets.Keysets)
	if err != nil {
		return fmt.Errorf("m.CheckProofsAreSameUnit(proofs, keysets.Keysets). %w", err)
	}

	// check if outputs are
	outputUnit, err := verifyOutputs(request.Outputs, keysets.Keysets)
	if err != nil {
		return fmt.Errorf("m.VerifyOutputs(outputs). %w", err)
	}

	if proofUnit != outputUnit {
		return fmt.Errorf("proofUnit != messageUnit. %w", cashu.ErrNotSameUnits)
	}

	return nil
}

// returns the proofs with the Y's and seen at.
func (m *Mint) checkProofSpent(tx pgx.Tx, proofs cashu.Proofs) (cashu.Proofs, error) {
	// gets the list of Y's. You need to calculate
	YsList, err := utils.GetAndCalculateProofsValues(&proofs)
	if err != nil {
		return nil, fmt.Errorf("utils.GetAndCalculateProofsValues(&proofs). %w", err)
	}

	// check if we know any of the proofs
	knownProofs, err := m.MintDB.GetProofsFromSecretCurve(tx, YsList)
	if err != nil {
		return nil, fmt.Errorf("m.MintDB.GetProofsFromSecretCurve(tx, SecretsList). %w", err)
	}

	if len(knownProofs) != 0 {
		for _, p := range knownProofs {
			if p.State == cashu.PROOF_PENDING {
				return nil, cashu.ErrProofPending
			}
		}
		return nil, cashu.ErrProofSpent
	}

	return proofs, nil
}
func (m *Mint) CheckOutputSpent(tx pgx.Tx, blindedMessages cashu.BlindedMessages) error {
	outputsMap := make(map[string]bool)
	blindingFactors := []cashu.WrappedPublicKey{}

	// Check if there is a repeated output, if not add it to the blindingFactors
	for _, output := range blindedMessages {
		outputKey := output.B_.String()
		exists := outputsMap[outputKey]
		if exists {
			return cashu.ErrRepeatedOutput
		}
		outputsMap[outputKey] = true

		blindingFactors = append(blindingFactors, output.B_)
	}

	blindRecoverySigs, err := m.MintDB.GetRestoreSigsFromBlindedMessages(tx, blindingFactors)
	if err != nil {
		return fmt.Errorf("m.GetRestorySigsFromBlindFactor(blindingFactors). %w", err)
	}

	if len(blindRecoverySigs) != 0 {
		return fmt.Errorf("blind message already has been signed: %w", cashu.ErrBlindMessageAlreadySigned)
	}

	return nil
}
