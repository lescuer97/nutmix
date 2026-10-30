//nolint:exhaustruct // Fixtures specify only fields relevant to the lifecycle scenario.
package mint

import (
	"context"
	"errors"
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lescuer97/nutmix/api/cashu"
	mockdb "github.com/lescuer97/nutmix/internal/database/mock_db"
	"github.com/lescuer97/nutmix/internal/lightning"
	"github.com/lescuer97/nutmix/internal/signer"
)

type changeLifecycleSigner struct{ lifecycleSigner }

func (changeLifecycleSigner) GetKeysets() (signer.GetKeysetsResponse, error) {
	return signer.GetKeysetsResponse{Keysets: []cashu.BasicKeysetResponse{{Id: testSatKeysetId, Unit: "sat"}}}, nil
}

func TestMeltChangeImmediateAndReconciledSettlement(t *testing.T) {
	for _, immediate := range []bool{true, false} {
		for _, zeroRefund := range []bool{true, false} {
			name := "reconciled"
			if immediate {
				name = "immediate"
			}
			if zeroRefund {
				name += " zero refund"
			}
			t.Run(name, func(t *testing.T) {
				db := &mockdb.MockDB{}
				mint := &Mint{MintDB: db, Signer: changeLifecycleSigner{}, LightningBackend: lightning.FakeWallet{Network: chaincfg.RegressionNetParams}}
				amount := uint64(15)
				if zeroRefund {
					amount = 12
				}
				keys := setupPendingMeltWithChange(t, mint, "melt", []uint64{amount}, 3)
				var quote cashu.MeltRequestDB
				var err error
				if immediate {
					quote = db.MeltRequest[0]
					quote.State, quote.Melted, quote.FeePaid = cashu.PAID, true, 10
					// Signing must use persisted blanks, not the request's output slice.
					quote, _, _, err = mint.finalizePaidBolt11Melt(context.Background(), bolt11MeltData{
						AmountProofs: cashu.NewAmount(cashu.Sat, amount), Fee: cashu.NewAmount(cashu.Sat, 0), Unit: cashu.Sat,
					}, cashu.PostMeltBolt11Request{Inputs: db.Proofs}, quote, cashu.NewAmount(cashu.Sat, 10))
				} else {
					quote, err = mint.RefreshMeltQuoteState(context.Background(), "melt")
				}
				if err != nil || quote.State != cashu.PAID {
					t.Fatalf("quote=%+v err=%v", quote, err)
				}
				want := 2
				if zeroRefund {
					want = 0
				}
				if len(quote.Change) != want || len(db.MeltChange) != 0 || len(db.RecoverSigDB) != want {
					t.Fatalf("change=%d pending=%d signatures=%d", len(quote.Change), len(db.MeltChange), len(db.RecoverSigDB))
				}
				for i, sig := range db.RecoverSigDB {
					if sig.MeltQuote != "melt" || sig.B_.ToHex() != keys[i].ToHex() || sig.Amount != uint64(1<<i) {
						t.Fatalf("wrong blank/denomination association: %+v", sig)
					}
				}
				for _, proof := range db.Proofs {
					if proof.State != cashu.PROOF_SPENT {
						t.Fatal("proofs were not spent with settlement")
					}
				}
				reloaded, err := mint.RefreshMeltQuoteState(context.Background(), "melt")
				if err != nil || len(reloaded.Change) != want {
					t.Fatalf("repeated quote read lost change: %+v %v", reloaded, err)
				}
				if err := db.CheckBlindMessagesAvailable(nil, []cashu.WrappedPublicKey{keys[2]}); err != nil {
					t.Fatalf("unused blank remains reserved: %v", err)
				}
			})
		}
	}
}

func TestMeltChangeUncertaintyRetainsBlanksAndFailureReleasesThem(t *testing.T) {
	for _, failed := range []bool{false, true} {
		name := "unknown"
		flags := []lightning.FakeWalletError{lightning.FailPaymentUnknown, lightning.FailQueryUnknown}
		if failed {
			name, flags = "failed", []lightning.FakeWalletError{lightning.FailPaymentFailed, lightning.FailQueryFailed}
		}
		t.Run(name, func(t *testing.T) {
			db := &mockdb.MockDB{}
			mint := &Mint{MintDB: db, LightningBackend: lightning.FakeWallet{Network: chaincfg.RegressionNetParams, UnpurposeErrors: flags}}
			keys := setupPendingMeltWithChange(t, mint, "melt", []uint64{15}, 2)
			quote, _, err := mint.attemptBolt11MeltPayment(context.Background(), cashu.PostMeltBolt11Request{Inputs: db.Proofs}, db.MeltRequest[0])
			if err != nil {
				t.Fatal(err)
			}
			if failed {
				if quote.State != cashu.UNPAID || len(db.MeltChange) != 0 {
					t.Fatal("definitive failure did not release blanks")
				}
			} else {
				if quote.State != cashu.PENDING || len(db.MeltChange) != 2 || len(db.Proofs) != 1 {
					t.Fatal("uncertain payment released ownership")
				}
				if err := db.CheckBlindMessagesAvailable(nil, keys); !errors.Is(err, cashu.ErrOutputPending) {
					t.Fatalf("uncertain blanks became available: %v", err)
				}
			}
		})
	}
}
