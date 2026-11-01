//nolint:exhaustruct_v5 // Fixtures specify only fields relevant to input ownership.
package database

import (
	"testing"

	"github.com/lescuer97/nutmix/api/cashu"
)

func TestValidateMeltSettlementInputsRequiresExactConsistentSet(t *testing.T) {
	quote := "melt-quote"
	requested := cashu.Proofs{
		{Secret: "a", Id: "ks", Amount: 2},
		{Secret: "b", Id: "ks", Amount: 3},
	}
	stored := cashu.Proofs{
		{Secret: "b", Id: "ks", Amount: 3, Quote: &quote, State: cashu.PROOF_PENDING},
		{Secret: "a", Id: "ks", Amount: 2, Quote: &quote, State: cashu.PROOF_PENDING},
	}
	if spent, err := ValidateMeltSettlementInputs(quote, requested, stored); err != nil || spent {
		t.Fatalf("pending validation spent=%v err=%v", spent, err)
	}
	for _, tc := range []struct { //nolint:govet // Table favors name-first readability over packing.
		name   string
		mutate func(cashu.Proofs) cashu.Proofs
	}{
		{"missing", func(p cashu.Proofs) cashu.Proofs { return p[:1] }},
		{"replaced secret", func(p cashu.Proofs) cashu.Proofs { p[0].Secret = "other"; return p }},
		{"wrong amount", func(p cashu.Proofs) cashu.Proofs { p[0].Amount++; return p }},
		{"wrong owner", func(p cashu.Proofs) cashu.Proofs { other := "other"; p[0].Quote = &other; return p }},
		{"mixed states", func(p cashu.Proofs) cashu.Proofs { p[0].State = cashu.PROOF_SPENT; return p }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := append(cashu.Proofs(nil), stored...)
			if _, err := ValidateMeltSettlementInputs(quote, requested, tc.mutate(candidate)); err != ErrMeltSettlementState {
				t.Fatalf("error=%v want=%v", err, ErrMeltSettlementState)
			}
		})
	}
	spentProofs := append(cashu.Proofs(nil), stored...)
	spentProofs.SetProofsState(cashu.PROOF_SPENT)
	if spent, err := ValidateMeltSettlementInputs(quote, requested, spentProofs); err != nil || !spent {
		t.Fatalf("spent validation spent=%v err=%v", spent, err)
	}
}
