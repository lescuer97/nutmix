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

func checkMessagesAreSameUnit(messages []cashu.BlindedMessage, keys []cashu.BasicKeysetResponse) (cashu.Unit, error) {
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
			return cashu.Sat, fmt.Errorf("proofs are not the same unit")
		}
	}

	if len(units) == 0 {
		return cashu.Sat, fmt.Errorf("no units found")
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

func verifyOutputs(outputs []cashu.BlindedMessage, keys []cashu.BasicKeysetResponse) (cashu.Unit, error) {
	// check output have the correct unit
	unit, err := checkMessagesAreSameUnit(outputs, keys)
	if err != nil {
		return unit, fmt.Errorf("m.checkMessagesAreSameUnit(outputs, keysets.Keysets). %w", err)
	}

	return unit, nil
}

func (m *Mint) VerifyOutputs(outputs []cashu.BlindedMessage, keys []cashu.BasicKeysetResponse) (cashu.Unit, error) {
	return verifyOutputs(outputs, keys)
}

// VerifyProofsBDHKE verifies the BDHKE cryptographic signatures of the proofs.
// This should always be called regardless of SIG_ALL.
func (m *Mint) VerifyProofsBDHKE(proofs cashu.Proofs) error {
	err := m.Signer.VerifyProofs(proofs)
	if err != nil {
		return fmt.Errorf("m.Signer.VerifyProofs(proofs). %w", err)
	}
	return nil
}

func (m *Mint) IsInternalTransaction(ctx context.Context, request string) (bool, error) {
	tx, err := m.MintDB.GetTx(ctx)
	if err != nil {
		return false, fmt.Errorf("m.MintDB.GetTx(ctx). %w", err)
	}
	defer func() {
		rollbackErr := m.MintDB.Rollback(ctx, tx)
		if rollbackErr != nil {
			if !errors.Is(rollbackErr, pgx.ErrTxClosed) {
				slog.Warn("rollback error", slog.Any("error", rollbackErr))
			}
		}
	}()

	mintRequest, err := m.MintDB.GetMintRequestByRequest(tx, request)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("m.MintDB.GetMintRequestByRequest() %w", err)
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
