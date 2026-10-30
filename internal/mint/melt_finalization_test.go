//nolint:exhaustruct // Fixtures specify only fields relevant to the finalization scenario.
package mint

import (
	"context"
	"errors"
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lescuer97/nutmix/api/cashu"
	mockdb "github.com/lescuer97/nutmix/internal/database/mock_db"
	"github.com/lescuer97/nutmix/internal/lightning"
)

func TestMeltFinalizationReturnsConcurrentlySettledChange(t *testing.T) {
	for _, zeroRefund := range []bool{false, true} {
		name := "refund"
		amount := uint64(15)
		if zeroRefund {
			name, amount = "zero refund", 12
		}
		t.Run(name, func(t *testing.T) {
			db := &mockdb.MockDB{}
			mint := &Mint{MintDB: db, Signer: changeLifecycleSigner{}, LightningBackend: lightning.FakeWallet{Network: chaincfg.RegressionNetParams}}
			setupPendingMeltWithChange(t, mint, "melt", []uint64{amount}, 3)
			cached := db.MeltRequest[0]
			request := cashu.PostMeltBolt11Request{Inputs: append(cashu.Proofs(nil), db.Proofs...)}
			settled, err := mint.RefreshMeltQuoteState(context.Background(), "melt")
			if err != nil {
				t.Fatal(err)
			}
			want := len(settled.Change)
			if (!zeroRefund && want != 2) || (zeroRefund && want != 0) {
				t.Fatalf("unexpected setup refund: %+v", settled.Change)
			}
			// The HTTP payment response arrives after the quote poll committed,
			// with stale fee/preimage data that must not overwrite that settlement.
			cached.State, cached.Melted, cached.FeePaid, cached.PaymentPreimage = cashu.PAID, true, 99, "stale-preimage"
			mint.Signer = failingSigner{signBlindMessagesErr: errors.New("must not sign again")}
			for range 2 {
				quote, response, proofs, err := mint.finalizePaidBolt11Melt(context.Background(), bolt11MeltData{
					AmountProofs: cashu.NewAmount(cashu.Sat, amount), Fee: cashu.NewAmount(cashu.Sat, 0), Unit: cashu.Sat,
				}, request, cached, cashu.NewAmount(cashu.Sat, 99))
				if err != nil {
					t.Fatal(err)
				}
				if len(response.Change) != want || len(quote.Change) != want || len(db.RecoverSigDB) != want {
					t.Fatal("concurrent settlement lost or duplicated refund signatures")
				}
				if quote.FeePaid != settled.FeePaid || response.PaymentPreimage != settled.PaymentPreimage || db.MeltRequest[0].FeePaid != settled.FeePaid {
					t.Fatal("cached response overwrote authoritative settlement metadata")
				}
				for i, sig := range response.Change {
					if sig.Amount != settled.Change[i].Amount || sig.C_.ToHex() != settled.Change[i].C_.ToHex() {
						t.Fatal("returned refund differs from committed signatures")
					}
				}
				for _, proof := range proofs {
					if proof.State != cashu.PROOF_SPENT {
						t.Fatal("returned proofs are not finalized")
					}
				}
			}
		})
	}
}

func TestMeltFinalizationCompletesInternallyPaidPendingProofs(t *testing.T) {
	db := &mockdb.MockDB{}
	mint := &Mint{MintDB: db, Signer: changeLifecycleSigner{}}
	setupPendingMeltWithChange(t, mint, "internal", []uint64{15}, 3)
	db.MeltRequest[0].State, db.MeltRequest[0].Melted = cashu.PAID, true
	cached := db.MeltRequest[0]
	cached.FeePaid = 99
	quote, response, proofs, err := mint.finalizePaidBolt11Melt(context.Background(), bolt11MeltData{
		AmountProofs: cashu.NewAmount(cashu.Sat, 15), Fee: cashu.NewAmount(cashu.Sat, 0), Unit: cashu.Sat,
	}, cashu.PostMeltBolt11Request{Inputs: append(cashu.Proofs(nil), db.Proofs...)}, cached, cashu.NewAmount(cashu.Sat, 99))
	if err != nil {
		t.Fatal(err)
	}
	if quote.State != cashu.PAID || quote.FeePaid != 0 || len(response.Change) != 3 || len(db.MeltChange) != 0 {
		t.Fatalf("internal settlement skipped finalization: %+v", quote)
	}
	var refund uint64
	for _, sig := range response.Change {
		refund += sig.Amount
	}
	if refund != 13 {
		t.Fatalf("internal refund=%d want=13", refund)
	}
	for _, proof := range proofs {
		if proof.State != cashu.PROOF_SPENT {
			t.Fatal("internal proofs remain pending")
		}
	}
}

func TestMeltFinalizationRejectsInconsistentStateWithoutWrites(t *testing.T) {
	for _, state := range []string{"unpaid", "missing proofs", "replaced inputs", "mixed proofs", "spent with blanks"} {
		t.Run(state, func(t *testing.T) {
			db := &mockdb.MockDB{}
			mint := &Mint{MintDB: db, Signer: changeLifecycleSigner{}}
			setupPendingMeltWithChange(t, mint, "melt", []uint64{8, 7}, 3)
			request := cashu.PostMeltBolt11Request{Inputs: append(cashu.Proofs(nil), db.Proofs...)}
			cached := db.MeltRequest[0]
			cached.State, cached.Melted, cached.FeePaid = cashu.PAID, true, 10
			switch state {
			case "unpaid":
				db.MeltRequest[0].State = cashu.UNPAID
			case "missing proofs":
				db.Proofs = nil
			case "replaced inputs":
				db.Proofs[0].Secret = "different-execution"
			case "mixed proofs":
				db.Proofs[0].State = cashu.PROOF_SPENT
			case "spent with blanks":
				db.MeltRequest[0].State, db.MeltRequest[0].Melted = cashu.PAID, true
				for i := range db.Proofs {
					db.Proofs[i].State = cashu.PROOF_SPENT
				}
			}
			before := append(cashu.Proofs(nil), db.Proofs...)
			originalState := db.MeltRequest[0].State
			_, response, _, err := mint.finalizePaidBolt11Melt(context.Background(), bolt11MeltData{
				AmountProofs: cashu.NewAmount(cashu.Sat, 15), Fee: cashu.NewAmount(cashu.Sat, 0), Unit: cashu.Sat,
			}, request, cached, cashu.NewAmount(cashu.Sat, 10))
			if !errors.Is(err, errMeltFinalizationState) {
				t.Fatalf("error=%v", err)
			}
			if len(response.Change) != 0 || len(db.RecoverSigDB) != 0 || len(db.MeltChange) != 3 || db.MeltRequest[0].State != originalState {
				t.Fatal("inconsistent settlement mutated output/quote records")
			}
			for i, proof := range db.Proofs {
				if proof.State != before[i].State {
					t.Fatal("inconsistent settlement mutated proofs")
				}
			}
		})
	}
}

func TestMeltFinalizationProofSetValidation(t *testing.T) {
	quote := "q"
	first := cashu.Proof{Secret: "first", Id: "keyset", Amount: 1, Quote: &quote, State: cashu.PROOF_PENDING}
	second := first
	second.Secret = "second"
	for _, tt := range []struct {
		name              string
		requested, stored cashu.Proofs
		spent, invalid    bool
	}{
		{"pending", cashu.Proofs{first, second}, cashu.Proofs{second, first}, false, false},
		{"empty", nil, nil, false, true},
		{"duplicate request", cashu.Proofs{first, first}, cashu.Proofs{first, second}, false, true},
		{"duplicate stored", cashu.Proofs{first, second}, cashu.Proofs{first, first}, false, true},
		{"unknown state", cashu.Proofs{first}, cashu.Proofs{{Secret: first.Secret, Id: first.Id, Amount: first.Amount, Quote: &quote, State: cashu.PROOF_UNSPENT}}, false, true},
		{"wrong amount", cashu.Proofs{first}, cashu.Proofs{{Secret: first.Secret, Id: first.Id, Amount: 2, Quote: &quote, State: first.State}}, false, true},
		{"missing ownership", cashu.Proofs{first}, cashu.Proofs{{Secret: first.Secret, Id: first.Id, Amount: first.Amount, State: first.State}}, false, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			spent, err := validateMeltFinalizationProofs(quote, tt.requested, tt.stored)
			if (err != nil) != tt.invalid || spent != tt.spent {
				t.Fatalf("spent=%v err=%v", spent, err)
			}
		})
	}
}
