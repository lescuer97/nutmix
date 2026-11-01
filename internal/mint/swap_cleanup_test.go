//nolint:exhaustruct_v5 // Fixtures specify only fields used by each cleanup scenario.
package mint

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
)

type swapCleanupContextKey struct{}

type swapContextObservation struct {
	err         error
	value       any
	deadline    time.Time
	hasDeadline bool
}

func observeSwapContext(ctx context.Context) swapContextObservation {
	deadline, hasDeadline := ctx.Deadline()
	return swapContextObservation{err: ctx.Err(), value: ctx.Value(swapCleanupContextKey{}), deadline: deadline, hasDeadline: hasDeadline}
}

type swapCleanupTestDB struct { //nolint:govet // Test double field order is not performance critical.
	database.MintDB
	getTxErr, setErr, insertErr, rollbackErr, commitErr, deleteErr error
	cancelOnSet                                                    context.CancelFunc
	closeOnCommitError                                             bool
	closed                                                         bool
	rollbacks, commits, deletes                                    int
	observations                                                   []swapContextObservation
}

func (d *swapCleanupTestDB) GetTx(ctx context.Context) (pgx.Tx, error) {
	d.recordContext(ctx)
	if d.getTxErr == nil {
		d.closed = false
	}
	return nil, d.getTxErr
}
func (d *swapCleanupTestDB) SetProofsState(context.Context, pgx.Tx, cashu.Proofs, cashu.ProofState) error {
	if d.cancelOnSet != nil {
		d.cancelOnSet()
	}
	return d.setErr
}
func (d *swapCleanupTestDB) InsertSignedOutputs(context.Context, pgx.Tx, []cashu.SignedOutput) error {
	return d.insertErr
}
func (d *swapCleanupTestDB) Rollback(ctx context.Context, _ pgx.Tx) error {
	d.recordContext(ctx)
	d.rollbacks++
	if d.closed {
		return pgx.ErrTxClosed
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if d.rollbackErr != nil {
		return d.rollbackErr
	}
	d.closed = true
	return nil
}
func (d *swapCleanupTestDB) Commit(ctx context.Context, _ pgx.Tx) error {
	d.recordContext(ctx)
	d.commits++
	if err := ctx.Err(); err != nil {
		return err
	}
	if d.commitErr != nil {
		if d.closeOnCommitError {
			d.closed = true
		}
		return d.commitErr
	}
	d.closed = true
	return nil
}
func (d *swapCleanupTestDB) DeleteProofs(ctx context.Context, _ pgx.Tx, _ cashu.Proofs) error {
	d.recordContext(ctx)
	d.deletes++
	return d.deleteErr
}
func (d *swapCleanupTestDB) recordContext(ctx context.Context) {
	d.observations = append(d.observations, observeSwapContext(ctx))
}

func TestSwapRollbackPreservesPendingCleanupEligibility(t *testing.T) {
	sentinel := errors.New("persistence failed")
	otherRollbackErr := errors.New("rollback failed")
	for _, tc := range []struct { //nolint:govet // Test case readability is more important than field packing.
		name                                                string
		setErr, insertErr, rollbackErr, getTxErr, commitErr error
		closeOnCommitError                                  bool
		wantCleanup                                         bool
		wantRollbacks, wantCommits                          int
	}{
		{name: "signing failure", wantCleanup: true},
		{name: "get transaction failure", getTxErr: sentinel, wantCleanup: true},
		{name: "set failure rollback succeeds", setErr: sentinel, wantCleanup: true, wantRollbacks: 1},
		{name: "insert failure rollback succeeds", insertErr: sentinel, wantCleanup: true, wantRollbacks: 1},
		{name: "rollback fails", setErr: sentinel, rollbackErr: otherRollbackErr, wantRollbacks: 1},
		{name: "commit failure rollback succeeds", commitErr: sentinel, wantCleanup: true, wantRollbacks: 1, wantCommits: 1},
		{name: "commit rollback other error", commitErr: sentinel, rollbackErr: otherRollbackErr, wantRollbacks: 1, wantCommits: 1},
		{name: "commit ambiguous closed", commitErr: sentinel, closeOnCommitError: true, rollbackErr: pgx.ErrTxClosed, wantRollbacks: 1, wantCommits: 1},
		{name: "commit succeeds", wantRollbacks: 1, wantCommits: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := &swapCleanupTestDB{setErr: tc.setErr, insertErr: tc.insertErr, rollbackErr: tc.rollbackErr, getTxErr: tc.getTxErr, commitErr: tc.commitErr, closeOnCommitError: tc.closeOnCommitError}
			m := Mint{MintDB: db, Signer: lifecycleSigner{}}
			if tc.name == "signing failure" {
				m.Signer = failingSwapSigningSigner{}
			}
			_, eligible, err := m.signSwapOutputsAndMarkInputsSpent(context.Background(), nil, cashu.PostSwapRequest{Outputs: cashu.BlindedMessages{{}}})
			if (err == nil) != (tc.name == "commit succeeds") {
				t.Fatalf("error=%v", err)
			}
			if eligible != tc.wantCleanup {
				t.Fatalf("cleanup eligibility=%t, want %t", eligible, tc.wantCleanup)
			}
			if db.rollbacks != tc.wantRollbacks || db.commits != tc.wantCommits {
				t.Fatalf("rollbacks=%d commits=%d, want %d/%d", db.rollbacks, db.commits, tc.wantRollbacks, tc.wantCommits)
			}
		})
	}
}

type failingSwapSigningSigner struct{ lifecycleSigner }

func (failingSwapSigningSigner) SignBlindMessages(context.Context, []cashu.BlindedMessage) ([]cashu.BlindSignature, []cashu.SignedOutput, error) {
	return nil, nil, errors.New("signing failed")
}

func TestSwapCleanupSurvivesCancellationDuringPersistence(t *testing.T) {
	root, cancel := context.WithCancel(context.WithValue(context.Background(), swapCleanupContextKey{}, "kept"))
	db := &swapCleanupTestDB{setErr: context.Canceled, cancelOnSet: cancel}
	m := Mint{MintDB: db, Signer: lifecycleSigner{}}
	_, eligible, err := m.signSwapOutputsAndMarkInputsSpent(root, nil, cashu.PostSwapRequest{Outputs: cashu.BlindedMessages{{}}})
	if !errors.Is(err, context.Canceled) || !eligible {
		t.Fatalf("eligible=%t error=%v", eligible, err)
	}
	if len(db.observations) != 2 {
		t.Fatalf("observed %d calls, want GetTx and Rollback", len(db.observations))
	}
	assertIndependentCleanupObservations(t, db.observations[1:])
	if err := m.removePendingSwapProofs(root, nil); err != nil {
		t.Fatalf("removePendingSwapProofs after cancellation: %v", err)
	}
	assertIndependentCleanupObservations(t, db.observations[2:])
}

func TestRemovePendingSwapProofsUsesBoundedIndependentContext(t *testing.T) {
	root, cancel := context.WithDeadline(context.WithValue(context.Background(), swapCleanupContextKey{}, "kept"), time.Now().Add(-time.Second))
	defer cancel()
	db := &swapCleanupTestDB{}
	m := Mint{MintDB: db}
	if err := m.removePendingSwapProofs(root, nil); err != nil {
		t.Fatalf("removePendingSwapProofs: %v", err)
	}
	if db.deletes != 1 || db.commits != 1 || len(db.observations) != 4 {
		t.Fatalf("deletes=%d commits=%d observations=%d", db.deletes, db.commits, len(db.observations))
	}
	assertIndependentCleanupObservations(t, db.observations)
}

func assertIndependentCleanupObservations(t *testing.T, observations []swapContextObservation) {
	t.Helper()
	for _, got := range observations {
		if got.err != nil || got.value != "kept" {
			t.Fatalf("cleanup context error=%v value=%v", got.err, got.value)
		}
		remaining := time.Until(got.deadline)
		if !got.hasDeadline || remaining <= 0 || remaining > transactionCleanupTimeout {
			t.Fatalf("invalid cleanup deadline: remaining=%v present=%t", remaining, got.hasDeadline)
		}
	}
}
