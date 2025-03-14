package mint

import (
	"context"
	"fmt"

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

	var AmountSignature uint64
	outputsMap := make(map[string]bool)

	// Check if there is a repeated output
	for _, output := range outputs {
		exists, _ := outputsMap[output.B_]

		if exists {
			return unit, fmt.Errorf("Repeated Blind Message")
		}
		outputsMap[output.B_] = true
		AmountSignature += output.Amount
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
