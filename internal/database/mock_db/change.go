package mockdb

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	"math"
	"time"
)

func (m *MockDB) ReserveMeltChangeOutputs(ctx context.Context, tx pgx.Tx, change []cashu.BlindedMessage, quote string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(change) == 0 {
		return nil
	}
	request, err := m.GetMeltRequestById(ctx, tx, quote)
	if err != nil {
		return err
	}
	if request.State != cashu.PENDING {
		return cashu.ErrQuoteIsPending
	}
	seen := make(map[string]bool, len(change))
	points := make([]cashu.WrappedPublicKey, len(change))
	for i, output := range change {
		if output.B_.PublicKey == nil || output.Id == "" {
			return cashu.ErrInvalidBlindMessage
		}
		key := output.B_.ToHex()
		if seen[key] {
			return cashu.ErrRepeatedOutput
		}
		seen[key], points[i] = true, output.B_
	}
	if err := m.CheckBlindMessagesAvailable(ctx, tx, points); err != nil {
		return err
	}
	for _, existing := range m.MeltChange {
		if existing.Quote == quote {
			return cashu.ErrQuoteIsPending
		}
	}
	for _, v := range change {
		m.MeltChange = append(m.MeltChange, cashu.MeltChange{
			B_:        v.B_,
			Id:        v.Id,
			Quote:     quote,
			CreatedAt: time.Now().Unix(),
		})
	}
	return nil
}
func (m *MockDB) LoadPendingMeltChangeForUpdate(ctx context.Context, tx pgx.Tx, quote string) ([]cashu.MeltChange, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var change []cashu.MeltChange
	for i := 0; i < len(m.MeltChange); i++ {
		if m.MeltChange[i].Quote == quote {
			change = append(change, m.MeltChange[i])
		}
	}

	return change, nil
}

func (m *MockDB) ReleasePendingMeltChangeOutputs(ctx context.Context, tx pgx.Tx, quote string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	remaining := m.MeltChange[:0]
	for _, message := range m.MeltChange {
		if message.Quote != quote {
			remaining = append(remaining, message)
		}
	}
	m.MeltChange = remaining

	return nil
}

func (m *MockDB) CompleteMeltSettlement(ctx context.Context, _ pgx.Tx, command database.MeltSettlementCommand) (database.MeltSettlementResult, error) {
	var empty database.MeltSettlementResult
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	if command.QuoteID == "" || command.Unit == "" {
		return empty, database.ErrMeltSettlementState
	}
	qi := -1
	for i := range m.MeltRequest {
		if m.MeltRequest[i].Quote == command.QuoteID {
			qi = i
			break
		}
	}
	if qi < 0 {
		return empty, pgx.ErrNoRows
	}
	quote := m.MeltRequest[qi]
	if quote.Unit != command.Unit || quote.CheckingId != command.CheckingID {
		return empty, database.ErrMeltSettlementState
	}
	stored := make(cashu.Proofs, 0)
	for _, p := range m.Proofs {
		if p.Quote != nil && *p.Quote == command.QuoteID {
			stored = append(stored, p)
		}
	}
	allSpent, err := database.ValidateMeltSettlementInputs(command.QuoteID, command.Inputs, stored)
	if err != nil {
		return empty, err
	}
	var signed []cashu.RecoverSigDB
	for _, s := range m.RecoverSigDB {
		if s.MeltQuote == command.QuoteID {
			signed = append(signed, s)
		}
	}
	pending, err := m.LoadPendingMeltChangeForUpdate(ctx, nil, command.QuoteID)
	if err != nil {
		return empty, err
	}
	if quote.State == cashu.PAID && quote.Melted && allSpent {
		if len(pending) != 0 {
			return empty, database.ErrMeltSettlementState
		}
		quote.Change = make([]cashu.BlindSignature, len(signed))
		for i, s := range signed {
			quote.Change[i] = s.GetBlindSignature()
		}
		return database.MeltSettlementResult{Quote: quote, Proofs: stored, AlreadyCompleted: true}, nil
	}
	if (quote.State != cashu.PENDING && quote.State != cashu.PAID) || (quote.State == cashu.PAID && !quote.Melted) || (quote.State == cashu.PENDING && quote.Melted) || allSpent {
		return empty, database.ErrMeltSettlementState
	}
	if quote.State == cashu.PAID && (quote.FeePaid != command.LightningFee || quote.PaymentPreimage != command.Preimage) {
		return empty, database.ErrMeltSettlementState
	}
	if len(signed) != 0 {
		return empty, database.ErrMeltSettlementState
	}
	inputAmount, err := (&stored).Amount()
	if err != nil {
		return empty, err
	}
	expense := quote.Amount
	if math.MaxUint64-expense < command.LightningFee {
		return empty, cashu.ErrAmountOverflow
	}
	expense += command.LightningFee
	if math.MaxUint64-expense < command.InputFee {
		return empty, cashu.ErrAmountOverflow
	}
	expense += command.InputFee
	refund := uint64(0)
	if inputAmount > expense {
		refund = inputAmount - expense
	}
	if _, err := database.ValidateMeltSettlementOutputs(pending, command.Outputs, refund); err != nil {
		return empty, err
	}
	// All validation is complete; mutations below mirror the atomic SQL boundary.
	for i := range m.Proofs {
		if m.Proofs[i].Quote != nil && *m.Proofs[i].Quote == command.QuoteID {
			m.Proofs[i].State = cashu.PROOF_SPENT
		}
	}
	for _, o := range command.Outputs {
		m.RecoverSigDB = append(m.RecoverSigDB, cashu.RecoverSigDB{B_: o.B_, C_: o.C_, Dleq: o.Dleq, Id: o.Id, Amount: o.Amount, CreatedAt: o.CreatedAt, MintQuote: "", MeltQuote: command.QuoteID})
	}
	remaining := m.MeltChange[:0]
	for _, v := range m.MeltChange {
		if v.Quote != command.QuoteID {
			remaining = append(remaining, v)
		}
	}
	m.MeltChange = remaining
	quote.State, quote.Melted, quote.FeePaid, quote.PaymentPreimage = cashu.PAID, true, command.LightningFee, command.Preimage
	quote.Change = make([]cashu.BlindSignature, len(command.Outputs))
	for i, o := range command.Outputs {
		quote.Change[i] = o.GetBlindSignature()
	}
	m.MeltRequest[qi] = quote
	spent := append(cashu.Proofs(nil), stored...)
	spent.SetProofsState(cashu.PROOF_SPENT)
	return database.MeltSettlementResult{Quote: quote, Proofs: spent, AlreadyCompleted: false}, nil
}
