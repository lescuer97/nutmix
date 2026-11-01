//nolint:exhaustruct_v5 // Test fixtures set only fields relevant to settlement deadlines.
package mint

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	mockdb "github.com/lescuer97/nutmix/internal/database/mock_db"
	"github.com/lescuer97/nutmix/internal/lightning"
	"github.com/lightningnetwork/lnd/zpay32"
)

type settlementDeadlineWallet struct {
	check func(context.Context, string, *zpay32.Invoice, string) (lightning.PaymentStatus, string, cashu.Amount, error)
	lightning.FakeWallet
}

func (w settlementDeadlineWallet) CheckPayed(ctx context.Context, quote string, invoice *zpay32.Invoice, checkingID string) (lightning.PaymentStatus, string, cashu.Amount, error) {
	return w.check(ctx, quote, invoice, checkingID)
}

func TestMeltSettlementReconciliationDeadline(t *testing.T) {
	db := &mockdb.MockDB{}
	var observedDeadline time.Time
	var observedAt time.Time
	wallet := &settlementDeadlineWallet{
		FakeWallet: lightning.FakeWallet{Network: chaincfg.RegressionNetParams},
		check: func(ctx context.Context, _ string, _ *zpay32.Invoice, _ string) (lightning.PaymentStatus, string, cashu.Amount, error) {
			observedDeadline, _ = ctx.Deadline()
			observedAt = time.Now()
			return lightning.UNKNOWN, "", cashu.Amount{Unit: cashu.Sat}, errors.New("stop reconciliation")
		},
	}
	mint := &Mint{MintDB: db, Signer: changeLifecycleSigner{}, LightningBackend: wallet}
	setupPendingMeltWithChange(t, mint, "deadline-quote", []uint64{15}, 3)

	err := mint.ReconcilePendingMeltQuotes(context.Background())
	if err == nil {
		t.Fatal("ReconcilePendingMeltQuotes() error = nil, want backend error")
	}
	if observedDeadline.IsZero() {
		t.Fatal("backend context has no reconciliation deadline")
	}
	remaining := observedDeadline.Sub(observedAt)
	if remaining > 60*time.Second || remaining < 30*time.Second {
		t.Fatalf("reconciliation deadline has %v remaining, want approximately 60 seconds", remaining)
	}
	assertPendingMockSettlement(t, db, "deadline-quote")
}

func TestMeltSettlementReconciliationParentDeadlineAndStopsOnError(t *testing.T) {
	db := &mockdb.MockDB{}
	var calls []string
	wallet := &settlementDeadlineWallet{
		FakeWallet: lightning.FakeWallet{Network: chaincfg.RegressionNetParams},
		check: func(_ context.Context, quote string, _ *zpay32.Invoice, _ string) (lightning.PaymentStatus, string, cashu.Amount, error) {
			calls = append(calls, quote)
			return lightning.UNKNOWN, "", cashu.Amount{Unit: cashu.Sat}, errors.New("stop on first quote")
		},
	}
	mint := &Mint{MintDB: db, Signer: changeLifecycleSigner{}, LightningBackend: wallet}
	setupPendingMeltWithChange(t, mint, "first-deadline-quote", []uint64{15}, 3)
	setupPendingMeltWithChange(t, mint, "second-deadline-quote", []uint64{15}, 3)

	parent, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	parentDeadline, _ := parent.Deadline()
	if err := mint.ReconcilePendingMeltQuotes(parent); err == nil {
		t.Fatal("ReconcilePendingMeltQuotes() error = nil, want backend error")
	}
	if len(calls) != 1 || calls[0] != "first-deadline-quote" {
		t.Fatalf("backend calls=%v, want only first quote", calls)
	}
	assertPendingMockSettlement(t, db, "first-deadline-quote")
	assertPendingMockSettlement(t, db, "second-deadline-quote")

	// Confirm the per-quote deadline cannot outlive an earlier parent deadline.
	var observedDeadline time.Time
	wallet.check = func(ctx context.Context, _ string, _ *zpay32.Invoice, _ string) (lightning.PaymentStatus, string, cashu.Amount, error) {
		observedDeadline, _ = ctx.Deadline()
		return lightning.UNKNOWN, "", cashu.Amount{Unit: cashu.Sat}, errors.New("parent deadline observed")
	}
	if err := mint.ReconcilePendingMeltQuotes(parent); err == nil {
		t.Fatal("second reconciliation error = nil, want backend error")
	}
	if !observedDeadline.Equal(parentDeadline) {
		t.Fatalf("effective deadline=%v, want inherited parent deadline %v", observedDeadline, parentDeadline)
	}
}

type rollbackDeadlineDB struct {
	*mockdb.MockDB
	rollbackErr      error
	rollbackDeadline time.Time
	hasDeadline      bool
	rollbackCalls    int
}

func (db *rollbackDeadlineDB) Rollback(ctx context.Context, tx pgx.Tx) error {
	db.rollbackCalls++
	db.rollbackErr = ctx.Err()
	db.rollbackDeadline, db.hasDeadline = ctx.Deadline()
	return db.MockDB.Rollback(ctx, tx)
}

type cancelDuringMeltSigning struct {
	changeLifecycleSigner
	cancel context.CancelFunc
}

func (s cancelDuringMeltSigning) SignBlindMessages(context.Context, []cashu.BlindedMessage) ([]cashu.BlindSignature, []cashu.SignedOutput, error) {
	s.cancel()
	return nil, nil, context.Canceled
}

func TestMeltSettlementIndependentRollbackBudget(t *testing.T) {
	db := &rollbackDeadlineDB{MockDB: &mockdb.MockDB{}}
	mint := &Mint{MintDB: db, Signer: changeLifecycleSigner{}}
	setupPendingMeltWithChange(t, mint, "rollback-deadline-quote", []uint64{15}, 3)
	// The fixture performs its own cleanup rollback; only inspect the transaction
	// rollback performed by the settlement call below.
	db.rollbackErr, db.rollbackDeadline, db.hasDeadline, db.rollbackCalls = nil, time.Time{}, false, 0

	parent, cancel := context.WithCancel(context.Background())
	mint.Signer = cancelDuringMeltSigning{cancel: cancel}
	quote := db.MeltRequest[0]
	quote.State, quote.Melted, quote.FeePaid, quote.PaymentPreimage = cashu.PAID, true, 1, "preimage-fixture"
	_, response, _, err := mint.finalizePaidBolt11Melt(parent,
		bolt11MeltData{AmountProofs: cashu.NewAmount(cashu.Sat, 15), Fee: cashu.NewAmount(cashu.Sat, 0), Unit: cashu.Sat},
		cashu.PostMeltBolt11Request{Inputs: db.Proofs}, quote, cashu.NewAmount(cashu.Sat, 1))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("finalizePaidBolt11Melt() error=%v, want context cancellation", err)
	}
	if len(response.Change) != 0 {
		t.Fatalf("returned %d change signatures after cancellation", len(response.Change))
	}
	if db.rollbackCalls != 1 {
		t.Fatalf("settlement rollback calls=%d, want one cleanup rollback", db.rollbackCalls)
	}
	if db.rollbackErr != nil {
		t.Fatalf("cleanup rollback context error at invocation: %v", db.rollbackErr)
	}
	if !db.hasDeadline {
		t.Fatal("cleanup rollback context has no bounded deadline")
	}
	remaining := time.Until(db.rollbackDeadline)
	if remaining <= time.Second || remaining > 2*time.Second {
		t.Fatalf("cleanup rollback deadline has %v remaining, want at most two seconds", remaining)
	}
	assertPendingMockSettlement(t, db.MockDB, "rollback-deadline-quote")
}

func assertPendingMockSettlement(t *testing.T, db *mockdb.MockDB, quoteID string) {
	t.Helper()
	foundQuote := false
	for _, quote := range db.MeltRequest {
		if quote.Quote == quoteID {
			foundQuote = quote.State == cashu.PENDING && !quote.Melted
			break
		}
	}
	if !foundQuote {
		t.Fatalf("quote %q is not pending", quoteID)
	}
	foundProof := false
	for _, proof := range db.Proofs {
		if proof.Quote != nil && *proof.Quote == quoteID {
			foundProof = true
			if proof.State != cashu.PROOF_PENDING {
				t.Fatalf("proof for quote %q changed to %s", quoteID, proof.State)
			}
		}
	}
	if !foundProof {
		t.Fatalf("no proof found for quote %q", quoteID)
	}
	foundBlanks := 0
	for _, blank := range db.MeltChange {
		if blank.Quote == quoteID {
			foundBlanks++
		}
	}
	if foundBlanks != 3 {
		t.Fatalf("quote %q has %d reserved blanks, want 3", quoteID, foundBlanks)
	}
}
