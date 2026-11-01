package database

import (
	"errors"

	"github.com/lescuer97/nutmix/api/cashu"
)

var ErrMeltSettlementState = errors.New("invalid melt settlement state")

// MeltSettlementCommand supplies the caller-validated fees and newly signed
// output prefix for one quote's atomic settlement. Fees are trusted inputs for
// a pending quote; for an already-paid quote LightningFee and Preimage must
// match the durable values.
type MeltSettlementCommand struct {
	QuoteID      string
	CheckingID   string
	Unit         string
	Preimage     string
	Inputs       cashu.Proofs
	Outputs      []cashu.SignedOutput
	LightningFee uint64
	InputFee     uint64
}

// MeltSettlementResult contains authoritative durable state. The caller owns
// commit and may expose signatures only after it succeeds.
type MeltSettlementResult struct {
	Proofs           cashu.Proofs
	Quote            cashu.MeltRequestDB
	AlreadyCompleted bool
}

// ValidateMeltSettlementInputs checks an exact, quote-owned input set with
// homogeneous PENDING or SPENT durable states.
func ValidateMeltSettlementInputs(quote string, requested, stored cashu.Proofs) (bool, error) {
	if quote == "" || len(requested) == 0 || len(requested) != len(stored) {
		return false, ErrMeltSettlementState
	}
	expected := make(map[string]cashu.Proof, len(requested))
	for _, p := range requested {
		if _, ok := expected[p.Secret]; ok {
			return false, ErrMeltSettlementState
		}
		expected[p.Secret] = p
	}
	spent := 0
	for _, p := range stored {
		original, ok := expected[p.Secret]
		if !ok || p.Quote == nil || *p.Quote != quote || original.Id != p.Id || original.Amount != p.Amount {
			return false, ErrMeltSettlementState
		}
		delete(expected, p.Secret)
		switch p.State {
		case cashu.PROOF_SPENT:
			spent++
		case cashu.PROOF_PENDING:
		default:
			return false, ErrMeltSettlementState
		}
	}
	if len(expected) != 0 || (spent != 0 && spent != len(stored)) {
		return false, ErrMeltSettlementState
	}
	return spent == len(stored), nil
}

// ValidateMeltSettlementOutputs validates the ordered pending prefix and refund bound.
func ValidateMeltSettlementOutputs(pending []cashu.MeltChange, outputs []cashu.SignedOutput, refund uint64) (uint64, error) {
	if len(outputs) > len(pending) {
		return 0, ErrMeltSettlementState
	}
	seen := make(map[string]struct{}, len(outputs))
	var total uint64
	for i, o := range outputs {
		if o.B_.PublicKey == nil || o.C_.PublicKey == nil || o.Id == "" || o.Amount == 0 || (o.Dleq != nil && (o.Dleq.E == nil || o.Dleq.S == nil)) {
			return 0, cashu.ErrInvalidBlindMessage
		}
		if pending[i].B_.PublicKey == nil {
			return 0, cashu.ErrInvalidBlindMessage
		}
		key := o.B_.ToHex()
		if key != pending[i].B_.ToHex() || o.Id != pending[i].Id {
			return 0, cashu.ErrInvalidBlindMessage
		}
		if _, ok := seen[key]; ok {
			return 0, cashu.ErrRepeatedOutput
		}
		seen[key] = struct{}{}
		if ^uint64(0)-total < o.Amount {
			return 0, cashu.ErrAmountOverflow
		}
		total += o.Amount
	}
	if total > refund {
		return 0, ErrMeltSettlementState
	}
	return total, nil
}
