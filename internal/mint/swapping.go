package mint

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/utils"
)

func (m *Mint) ExecuteSwap(ctx context.Context, request cashu.PostSwapRequest) (cashu.PostSwapResponse, error) {
	amountValidationErr := m.validateSwapBalanceAndUnits(request)
	if amountValidationErr != nil {
		return cashu.PostSwapResponse{}, fmt.Errorf("m.validateSwapBalanceAndUnits(request). %w", amountValidationErr)
	}

	// validate sig all
	err := m.validateSwapProofsAndSpendConditions(request)
	if err != nil {
		return cashu.PostSwapResponse{}, fmt.Errorf("m.validateSwapProofsAndSpendConditions(request). %w", err)
	}

	// check if proofs are spent and if outputs are spent
	sizeCheckTx, err := m.MintDB.GetTx(ctx)
	if err != nil {
		return cashu.PostSwapResponse{}, fmt.Errorf("m.MintDB.GetTx(ctx). %w", err)
	}
	defer m.rollbackIndependent(ctx, sizeCheckTx)

	proofs, err := m.validateProofsUnspent(ctx, sizeCheckTx, request.Inputs)
	if err != nil {
		return cashu.PostSwapResponse{}, fmt.Errorf("m.validateProofsUnspent(sizeCheckTx, request.Inputs). %w", err)
	}

	err = m.ValidateOutputsNotSpent(ctx, sizeCheckTx, request.Outputs)
	if err != nil {
		return cashu.PostSwapResponse{}, fmt.Errorf("m.ValidateOutputsNotSpent(sizeCheckTx, request.Outputs). %w", err)
	}

	proofs.SetProofsState(cashu.PROOF_PENDING)
	err = m.MintDB.SaveProof(ctx, sizeCheckTx, proofs)
	if err != nil {
		return cashu.PostSwapResponse{}, fmt.Errorf("m.MintDB.SaveProof(sizeCheckTx, proofs). %w", err)
	}

	err = sizeCheckTx.Commit(ctx)
	if err != nil {
		return cashu.PostSwapResponse{}, fmt.Errorf("sizeCheckTx.Commit(ctx). %w", err)
	}

	blindSignatures, shouldRemovePendingProofs, err := m.signSwapOutputsAndMarkInputsSpent(ctx, proofs, request)
	if err != nil {
		if shouldRemovePendingProofs {
			cleanupErr := m.removePendingSwapProofs(ctx, proofs)
			if cleanupErr != nil {
				return cashu.PostSwapResponse{}, fmt.Errorf("m.signSwapOutputsAndMarkInputsSpent(ctx, proofs, request). %w; m.removePendingSwapProofs(proofs). %w", err, cleanupErr)
			}
		}

		return cashu.PostSwapResponse{}, fmt.Errorf("m.signSwapOutputsAndMarkInputsSpent(ctx, proofs, request). %w", err)
	}

	proofs.SetProofsState(cashu.PROOF_SPENT)
	go m.Observer.SendProofsEvent(proofs)
	// mark as pending and sign
	return cashu.PostSwapResponse{
		Signatures: blindSignatures,
	}, nil
}

func (m *Mint) signSwapOutputsAndMarkInputsSpent(ctx context.Context, inputs cashu.Proofs, swapRequest cashu.PostSwapRequest) (blindedSignatures []cashu.BlindSignature, shouldRemovePendingProofs bool, err error) {
	// sign the outputs
	blindedSignatures, signedOutputs, err := m.Signer.SignBlindMessages(ctx, swapRequest.Outputs)
	if err != nil {
		return nil, true, fmt.Errorf("m.Signer.SignBlindMessages(outputs). %w", err)
	}

	afterSigningTx, err := m.MintDB.GetTx(ctx)
	if err != nil {
		return nil, true, fmt.Errorf("m.MintDB.GetTx(ctx). %w", err)
	}

	rollbackAttempted := false
	defer func() {
		if rollbackAttempted {
			return
		}
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), transactionCleanupTimeout)
		rollbackErr := m.MintDB.Rollback(cleanupCtx, afterSigningTx)
		cancel()
		if rollbackErr == nil && err != nil {
			shouldRemovePendingProofs = true
		} else if rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			slog.Warn("could not rollback failed swap persistence", slog.Any("error", rollbackErr))
		}
	}()
	err = m.MintDB.SetProofsState(ctx, afterSigningTx, inputs, cashu.PROOF_SPENT)
	if err != nil {
		return nil, false, fmt.Errorf("m.MintDB.SetProofsState(afterSigningTx, inputs, cashu.PROOF_SPENT). %w", err)
	}
	err = m.MintDB.InsertSignedOutputs(ctx, afterSigningTx, signedOutputs)
	if err != nil {
		return nil, false, fmt.Errorf("m.MintDB.InsertSignedOutputs(ctx, afterSigningTx, signedOutputs). %w", err)
	}
	err = m.MintDB.Commit(ctx, afterSigningTx)
	if err != nil {
		rollbackAttempted = true
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), transactionCleanupTimeout)
		rollbackErr := m.MintDB.Rollback(cleanupCtx, afterSigningTx)
		cancel()
		if rollbackErr == nil {
			return nil, true, fmt.Errorf("m.MintDB.Commit(ctx, afterSigningTx). %w", err)
		}
		if !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			slog.Warn("could not rollback failed swap commit", slog.Any("error", rollbackErr))
		}
		return nil, false, fmt.Errorf("m.MintDB.Commit(ctx, afterSigningTx). %w", err)
	}
	return blindedSignatures, false, nil
}

func (m *Mint) removePendingSwapProofs(ctx context.Context, proofs cashu.Proofs) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), transactionCleanupTimeout)
	defer cancel()
	tx, err := m.MintDB.GetTx(cleanupCtx)
	if err != nil {
		return fmt.Errorf("m.MintDB.GetTx(cleanupCtx). %w", err)
	}
	defer m.rollbackIndependent(cleanupCtx, tx)

	err = m.MintDB.DeleteProofs(cleanupCtx, tx, proofs)
	if err != nil {
		return fmt.Errorf("m.MintDB.DeleteProofs(tx, proofs). %w", err)
	}

	err = m.MintDB.Commit(cleanupCtx, tx)
	if err != nil {
		return fmt.Errorf("m.MintDB.Commit(cleanupCtx, tx). %w", err)
	}

	return nil
}

func (m *Mint) validateSwapProofsAndSpendConditions(request cashu.PostSwapRequest) error {
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

func (m *Mint) validateSwapBalanceAndUnits(request cashu.PostSwapRequest) error {
	if len(request.Inputs) == 0 || len(request.Outputs) == 0 {
		return fmt.Errorf("inputs or outputs are empty")
	}
	proofsAmount, err := request.Inputs.Amount()
	if err != nil {
		return errors.Join(cashu.ErrUnbalanced, err)
	}
	blindMessageAmount, err := request.Outputs.Amount()
	if err != nil {
		return errors.Join(cashu.ErrUnbalanced, err)
	}

	keysets, err := m.Signer.GetKeysets()
	if err != nil {
		return err
	}

	// check for needed amount of fees
	fee, err := cashu.Fees(request.Inputs, keysets.Keysets)
	if err != nil {
		if errors.Is(err, cashu.ErrAmountOverflow) {
			return errors.Join(cashu.ErrUnbalanced, err)
		}
		return fmt.Errorf("cashu.Fees(request.Inputs, keysets.Keysets). %w", err)
	}

	if blindMessageAmount > math.MaxUint64-fee {
		return errors.Join(cashu.ErrUnbalanced, cashu.ErrAmountOverflow)
	}
	if proofsAmount != blindMessageAmount+fee {
		return fmt.Errorf("proofs amount != outputs amount + fee. %w", cashu.ErrUnbalanced)
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
		return fmt.Errorf("proofUnit != messageUnit. %w", cashu.ErrDifferentInputOutputUnit)
	}

	return nil
}

// returns the proofs with the Y's and seen at.
func (m *Mint) validateProofsUnspent(ctx context.Context, tx pgx.Tx, proofs cashu.Proofs) (cashu.Proofs, error) {
	// gets the list of Y's. You need to calculate
	YsList, err := utils.GetAndCalculateProofsValues(&proofs)
	if err != nil {
		return nil, fmt.Errorf("utils.GetAndCalculateProofsValues(&proofs). %w", err)
	}

	// check if we know any of the proofs
	knownProofs, err := m.MintDB.GetProofsFromSecretCurve(ctx, tx, YsList)
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
func (m *Mint) ValidateOutputsNotSpent(ctx context.Context, tx pgx.Tx, blindedMessages cashu.BlindedMessages) error {
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

	return m.MintDB.CheckBlindMessagesAvailable(ctx, tx, blindingFactors)
}
