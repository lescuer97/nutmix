package mint

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"reflect"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/utils"
)

var errMeltFinalizationState = database.ErrMeltSettlementState

const (
	transactionCleanupTimeout      = 2 * time.Second
	meltReconciliationQuoteTimeout = 60 * time.Second
)

func validateMeltFinalizationProofs(quote string, requested, stored cashu.Proofs) (bool, error) {
	return database.ValidateMeltSettlementInputs(quote, requested, stored)
}

func (m *Mint) rollbackIndependent(ctx context.Context, tx pgx.Tx) {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), transactionCleanupTimeout)
	defer cancel()
	if err := m.MintDB.Rollback(cleanupCtx, tx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		slog.Warn("rollback error", slog.Any("error", err))
	}
}

func (m *Mint) settlePaidMelt(ctx context.Context, observed cashu.MeltRequestDB, expected cashu.Proofs, expectedUnit string, inputFee *cashu.Amount, lightningFee cashu.Amount) (cashu.MeltRequestDB, cashu.Proofs, error) {
	var keysets []cashu.BasicKeysetResponse
	tx, err := m.MintDB.GetTx(ctx)
	if err != nil {
		return cashu.MeltRequestDB{}, nil, fmt.Errorf("m.MintDB.GetTx(ctx): %w", err)
	}
	defer m.rollbackIndependent(ctx, tx)
	current, err := m.MintDB.GetMeltRequestById(ctx, tx, observed.Quote)
	if err != nil {
		return cashu.MeltRequestDB{}, nil, err
	}
	proofs, err := m.MintDB.GetProofsFromQuote(ctx, tx, current.Quote)
	if err != nil {
		return cashu.MeltRequestDB{}, nil, err
	}
	spent, err := database.ValidateMeltSettlementInputs(current.Quote, expected, proofs)
	if err != nil {
		return cashu.MeltRequestDB{}, nil, err
	}
	if current.Unit != observed.Unit || current.CheckingId != observed.CheckingId {
		return cashu.MeltRequestDB{}, nil, cashu.ErrQuoteIsPending
	}
	if spent {
		if current.State != cashu.PAID || !current.Melted {
			return cashu.MeltRequestDB{}, nil, database.ErrMeltSettlementState
		}
		pending, err := m.MintDB.LoadPendingMeltChangeForUpdate(ctx, tx, current.Quote)
		if err != nil {
			return cashu.MeltRequestDB{}, nil, err
		}
		if len(pending) != 0 {
			return cashu.MeltRequestDB{}, nil, database.ErrMeltSettlementState
		}
		if err = m.MintDB.Commit(ctx, tx); err != nil {
			return cashu.MeltRequestDB{}, nil, err
		}
		return current, proofs, nil
	}
	if current.State != cashu.PENDING && (current.State != cashu.PAID || !current.Melted) {
		return cashu.MeltRequestDB{}, nil, database.ErrMeltSettlementState
	}
	if current.State == cashu.PENDING && current.Melted || len(current.Change) != 0 {
		return cashu.MeltRequestDB{}, nil, database.ErrMeltSettlementState
	}
	if expectedUnit != "" && current.Unit != expectedUnit {
		return cashu.MeltRequestDB{}, nil, database.ErrMeltSettlementState
	}
	unit, err := cashu.UnitFromString(current.Unit)
	if err != nil {
		return cashu.MeltRequestDB{}, nil, err
	}
	if current.State == cashu.PENDING {
		current.State, current.Melted = cashu.PAID, true
		current.FeePaid, current.PaymentPreimage = observed.FeePaid, observed.PaymentPreimage
	} else {
		lightningFee = cashu.Amount{Unit: unit, Amount: current.FeePaid}
	}
	if inputFee == nil {
		response, err := m.Signer.GetKeysets()
		if err != nil {
			return cashu.MeltRequestDB{}, nil, fmt.Errorf("m.Signer.GetKeysets(): %w", err)
		}
		keysets = response.Keysets
		calculatedFee, feeErr := cashu.Fees(proofs, keysets)
		err = feeErr
		if err != nil {
			return cashu.MeltRequestDB{}, nil, err
		}
		calculatedAmount := cashu.Amount{Unit: unit, Amount: calculatedFee}
		inputFee = &calculatedAmount
	}
	lf := lightningFee
	if err = lf.To(unit); err != nil {
		return cashu.MeltRequestDB{}, nil, err
	}
	fee := *inputFee
	if err = fee.To(unit); err != nil {
		return cashu.MeltRequestDB{}, nil, err
	}
	amount, err := proofs.Amount()
	if err != nil {
		return cashu.MeltRequestDB{}, nil, err
	}
	if current.Amount > math.MaxUint64-lf.Amount || current.Amount+lf.Amount > math.MaxUint64-fee.Amount {
		return cashu.MeltRequestDB{}, nil, cashu.ErrAmountOverflow
	}
	total := current.Amount + lf.Amount + fee.Amount
	var refund uint64
	if amount > total {
		refund = amount - total
	}
	pending, err := m.MintDB.LoadPendingMeltChangeForUpdate(ctx, tx, current.Quote)
	if err != nil {
		return cashu.MeltRequestDB{}, nil, err
	}
	blanks := make([]cashu.BlindedMessage, len(pending))
	for i, p := range pending {
		blanks[i] = cashu.BlindedMessage{B_: p.B_, Id: p.Id, Witness: "", Amount: 0}
	}
	plan := utils.GetMessagesForChange(refund, append([]cashu.BlindedMessage(nil), blanks...))
	var sigs []cashu.BlindSignature
	var outputs []cashu.SignedOutput
	if len(plan) > 0 {
		signerBlanks := append([]cashu.BlindedMessage(nil), blanks...)
		sigs, outputs, err = m.GetChangeOutput(ctx, signerBlanks, refund, current.Unit)
		if err != nil {
			return cashu.MeltRequestDB{}, nil, err
		}
		if len(sigs) != len(plan) || len(outputs) != len(plan) {
			return cashu.MeltRequestDB{}, nil, database.ErrMeltSettlementState
		}
		for i, msg := range plan {
			if outputs[i].B_.PublicKey == nil || outputs[i].C_.PublicKey == nil ||
				(outputs[i].Dleq != nil && (outputs[i].Dleq.E == nil || outputs[i].Dleq.S == nil)) ||
				outputs[i].B_.ToHex() != msg.B_.ToHex() || outputs[i].Id != msg.Id || outputs[i].Amount != msg.Amount ||
				sigs[i].C_.PublicKey == nil || sigs[i].Id != outputs[i].Id || sigs[i].Amount != outputs[i].Amount ||
				sigs[i].C_.ToHex() != outputs[i].C_.ToHex() || !reflect.DeepEqual(sigs[i].Dleq, outputs[i].Dleq) {
				return cashu.MeltRequestDB{}, nil, database.ErrMeltSettlementState
			}
		}
	}
	command := database.MeltSettlementCommand{QuoteID: current.Quote, CheckingID: current.CheckingId, Unit: current.Unit, Inputs: expected, Outputs: outputs, LightningFee: lf.Amount, InputFee: fee.Amount, Preimage: current.PaymentPreimage}
	result, err := m.MintDB.CompleteMeltSettlement(ctx, tx, command)
	if err != nil {
		return cashu.MeltRequestDB{}, nil, err
	}
	if err = m.MintDB.Commit(ctx, tx); err != nil {
		return cashu.MeltRequestDB{}, nil, err
	}
	return result.Quote, result.Proofs, nil
}
