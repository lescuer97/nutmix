package mockdb

import (
	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	"time"
)

func (m *MockDB) SaveMeltChange(tx pgx.Tx, change []cashu.BlindedMessage, quote string) error {
	if len(change) == 0 {
		return nil
	}
	request, err := m.GetMeltRequestById(tx, quote)
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
	if err := m.CheckBlindMessagesAvailable(tx, points); err != nil {
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
func (m *MockDB) GetMeltChangeByQuote(tx pgx.Tx, quote string) ([]cashu.MeltChange, error) {
	var change []cashu.MeltChange
	for i := 0; i < len(m.MeltChange); i++ {
		if m.MeltChange[i].Quote == quote {
			change = append(change, m.MeltChange[i])
		}
	}

	return change, nil
}

func (m *MockDB) DeleteChangeByQuote(tx pgx.Tx, quote string) error {
	remaining := m.MeltChange[:0]
	for _, message := range m.MeltChange {
		if message.Quote != quote {
			remaining = append(remaining, message)
		}
	}
	m.MeltChange = remaining

	return nil
}

func (m *MockDB) FinalizeMeltChange(tx pgx.Tx, quote string, signatures []cashu.RecoverSigDB) error {
	request, err := m.GetMeltRequestById(tx, quote)
	if err != nil {
		return err
	}
	if request.State != cashu.PENDING && request.State != cashu.PAID {
		return cashu.ErrQuoteIsPending
	}
	pending, err := m.GetMeltChangeByQuote(tx, quote)
	if err != nil {
		return err
	}
	if len(signatures) > len(pending) {
		return cashu.ErrQuoteIsPending
	}
	for i, sig := range signatures {
		if sig.B_.PublicKey == nil || sig.C_.PublicKey == nil || sig.Amount == 0 || sig.MintQuote != "" ||
			(sig.MeltQuote != "" && sig.MeltQuote != quote) ||
			(sig.Dleq != nil && (sig.Dleq.E == nil || sig.Dleq.S == nil)) {
			return cashu.ErrInvalidBlindMessage
		}
		if sig.B_.ToHex() != pending[i].B_.ToHex() || sig.Id != pending[i].Id {
			return cashu.ErrInvalidBlindMessage
		}
	}
	for _, sig := range signatures {
		sig.MeltQuote = quote
		m.RecoverSigDB = append(m.RecoverSigDB, sig)
	}
	return m.DeleteChangeByQuote(tx, quote)
}
