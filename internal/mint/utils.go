package mint

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/utils"
)

func (m *Mint) GetChangeOutput(messages []cashu.BlindedMessage, overPaidFees uint64, unit string) ([]cashu.RecoverSigDB, error) {
	if overPaidFees > 0 && len(messages) > 0 {

		change := utils.GetMessagesForChange(overPaidFees, messages)

		_, recoverySigsDb, err := m.Signer.SignBlindMessages(change)

		if err != nil {
			return recoverySigsDb, fmt.Errorf("m.Signer.SignBlindMessages(change). %w", err)
		}

		return recoverySigsDb, nil

	}
	return []cashu.RecoverSigDB{}, nil
}

func (m *Mint) VerifyUnitSupport(unitStr string) error {
	unit, err := cashu.UnitFromString(unitStr)
	if err != nil {
		return fmt.Errorf(" cashu.UnitFromString(unitStr). %w. %w", err, cashu.ErrUnitNotSupported)
	}

	supported := m.LightningBackend.VerifyUnitSupport(unit)

	if !supported {
		return fmt.Errorf(" m.LightningBackend.VerifyUnitSupport(unit). %w. %w", err, cashu.ErrUnitNotSupported)
	}
	return nil
}

func (m *Mint) checkMessagesAreSameUnit(messages []cashu.BlindedMessage, keys []cashu.BasicKeysetResponse) (cashu.Unit, error) {

	units := make(map[string]bool)

	seenKeys := make(map[string]cashu.BasicKeysetResponse)

	for _, v := range keys {
		seenKeys[v.Id] = v
	}
	for _, proof := range messages {

		val, exists := seenKeys[proof.Id]

		if exists {
			units[val.Unit] = true
		}
		if len(units) > 1 {
			return cashu.Sat, fmt.Errorf("Proofs are not the same unit")
		}
	}

	if len(units) == 0 {
		return cashu.Sat, fmt.Errorf("No units found")
	}

	var returnedUnit cashu.Unit
	for unit := range units {
		finalUnit, err := cashu.UnitFromString(unit)
		if err != nil {
			return cashu.Sat, fmt.Errorf("UnitFromString: %w", err)
		}

		returnedUnit = finalUnit
	}

	return returnedUnit, nil

}

func (m *Mint) GetRestorySigsFromBlindFactor(blindingFactors []string) ([]cashu.RecoverSigDB, error) {

	var recoverySigs []cashu.RecoverSigDB
	ctx := context.Background()
	tx, err := m.MintDB.GetTx(ctx)
	if err != nil {
		return recoverySigs, fmt.Errorf("mint.MintDB.GetTx(ctx): %w", err)
	}
	defer m.MintDB.Rollback(ctx, tx)

	recoverySigs, err = m.MintDB.GetRestoreSigsFromBlindedMessages(tx, blindingFactors)
	if err != nil {
		return recoverySigs, fmt.Errorf("m.MintDB.GetRestoreSigsFromBlindedMessages(tx, blindingFactors): %w", err)
	}

	err = m.MintDB.Commit(ctx, tx)
	if err != nil {
		return recoverySigs, fmt.Errorf("m.MintDB.Commit(ctx, tx): %w", err)
	}
	return recoverySigs, nil
}

func (m *Mint) VerifyOutputs(outputs []cashu.BlindedMessage, keys []cashu.BasicKeysetResponse) (cashu.Unit, error) {

	// check output have the correct unit
	unit, err := m.checkMessagesAreSameUnit(outputs, keys)
	if err != nil {
		return unit, fmt.Errorf("m.checkMessagesAreSameUnit(outputs, keysets.Keysets). %w", err)
	}

	outputsMap := make(map[string]bool)

	// Check if there is a repeated output
	for _, output := range outputs {
		exists, _ := outputsMap[output.B_]

		if exists {
			return unit, cashu.ErrRepeatedOutput
		}
		outputsMap[output.B_] = true
	}

	// check if it has been signed before
	blindingFactors := []string{}
	for _, output := range outputs {
		blindingFactors = append(blindingFactors, output.B_)
	}
	blindRecoverySigs, err := m.GetRestorySigsFromBlindFactor(blindingFactors)
	if err != nil {
		return unit, fmt.Errorf("m.GetRestorySigsFromBlindFactor(blindingFactors). %w", err)
	}

	if len(blindRecoverySigs) != 0 {
		return unit, fmt.Errorf("Blind Message already has been signed. %w", cashu.ErrBlindMessageAlreadySigned)
	}
	return unit, nil
}

func (m *Mint) VerifyInputsAndOutputs(proofs cashu.Proofs, outputs []cashu.BlindedMessage) error {
	keysets, err := m.Signer.GetKeys()
	if err != nil {
		return fmt.Errorf("m.Signer.GetKeys(). %w", err)
	}

	// get unit from proofs
	proofUnit, err := m.CheckProofsAreSameUnit(proofs, keysets.Keysets)
	if err != nil {
		return fmt.Errorf("m.CheckProofsAreSameUnit(proofs, keysets.Keysets). %w", err)
	}

	outputUnit, err := m.VerifyOutputs(outputs, keysets.Keysets)
	if err != nil {
		return fmt.Errorf("m.VerifyOutputs(outputs). %w", err)
	}

	if proofUnit != outputUnit {
		return fmt.Errorf("proofUnit != messageUnit. %w", cashu.ErrNotSameUnits)
	}

	// check for needed amount of fees
	fee, err := cashu.Fees(proofs, keysets.Keysets)
	if err != nil {
		return fmt.Errorf("cashu.Fees(proofs, keysets.Keysets). %w", err)
	}

	var AmountSignature uint64
	// Check out amount signature
	for _, output := range outputs {
		AmountSignature += output.Amount
	}

	balance := (proofs.Amount() - (uint64(fee) + AmountSignature))
	if balance != 0 {
		return fmt.Errorf("(proofs.Amount() - (uint64(fee) + AmountSignature)). %w %w", err, cashu.ErrUnbalanced)
	}

	err = m.Signer.VerifyProofs(proofs, outputs)

	if err != nil {
		return fmt.Errorf("m.Signer.VerifyProofs(proofs, outputs). %w", err)
	}

	return nil
}
func (m *Mint) IsInternalTransaction(request string) (bool, error) {
	ctx := context.Background()
	tx, err := m.MintDB.GetTx(context.Background())
	if err != nil {
		return false, fmt.Errorf("m.MintDB.GetTx(context.Background()). %w", err)
	}
	defer m.MintDB.Rollback(ctx, tx)

	mintRequest, err := m.MintDB.GetMintRequestByRequest(tx, request)
	log.Printf("Mint request %+v", mintRequest)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("m.MintDB.GetMintRequestById() %w", err)
	}

	err = m.MintDB.Commit(ctx, tx)
	if err != nil {
		return false, fmt.Errorf("m.MintDB.Commit(ctx, tx) %w", err)
	}

	if mintRequest.Request == request {
		return true, nil
	}

	return false, nil
}
