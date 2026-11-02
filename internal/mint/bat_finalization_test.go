//nolint:exhaustruct // Test fixtures intentionally specify only relevant fields.
package mint

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
)

type batFinalizationDB struct { //nolint:govet // Test fixture favors readable call counters over alignment.
	database.MintDB
	proof                                          cashu.Proof
	getTxCalls, commitCalls, setCalls, deleteCalls int
	getTxErr                                       func(int) error
	commit                                         func(int, *batFinalizationDB) error
	deadlineObserved                               bool
	waitForCancellation                            bool
}

func (db *batFinalizationDB) GetTx(ctx context.Context) (pgx.Tx, error) {
	db.getTxCalls++
	if db.waitForCancellation {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	deadline, ok := ctx.Deadline()
	db.deadlineObserved = db.deadlineObserved || ok && time.Until(deadline) <= 5*time.Second
	if db.getTxErr != nil {
		if err := db.getTxErr(db.getTxCalls); err != nil {
			return nil, err
		}
	}
	return nil, nil
}
func (db *batFinalizationDB) Commit(context.Context, pgx.Tx) error {
	db.commitCalls++
	if db.commit != nil {
		return db.commit(db.commitCalls, db)
	}
	return nil
}
func (*batFinalizationDB) Rollback(context.Context, pgx.Tx) error { return nil }
func (db *batFinalizationDB) GetProofsFromSecretCurve(pgx.Tx, []cashu.WrappedPublicKey) (cashu.Proofs, error) {
	if db.proof.Id == "" {
		return nil, nil
	}
	return cashu.Proofs{db.proof}, nil
}
func (db *batFinalizationDB) SetProofsState(_ pgx.Tx, proofs cashu.Proofs, state cashu.ProofState) error {
	db.setCalls++
	proofs.SetProofsState(state)
	db.proof = proofs[0]
	return nil
}
func (db *batFinalizationDB) DeleteProofs(pgx.Tx, cashu.Proofs) error {
	db.deleteCalls++
	db.proof = cashu.Proof{}
	return nil
}

func TestBATFinalizationSpendsAndIsIdempotent(t *testing.T) {
	owner := "ownerA"
	db := &batFinalizationDB{proof: cashu.Proof{Id: "id", Quote: &owner, State: cashu.PROOF_PENDING}}
	m := &Mint{MintDB: db}
	if err := m.FinishAuthBlindToken(db.proof, true); err != nil {
		t.Fatal(err)
	}
	if db.proof.State != cashu.PROOF_SPENT {
		t.Fatalf("state=%v", db.proof.State)
	}
	if err := m.FinishAuthBlindToken(db.proof, true); err != nil {
		t.Fatalf("idempotent finish: %v", err)
	}
}

func TestBATFinalizationFailureDeletesReservation(t *testing.T) {
	owner := "ownerA"
	db := &batFinalizationDB{proof: cashu.Proof{Id: "id", Quote: &owner, State: cashu.PROOF_PENDING}}
	if err := (&Mint{MintDB: db}).FinishAuthBlindToken(db.proof, false); err != nil {
		t.Fatal(err)
	}
	if db.proof.Id != "" || db.deleteCalls != 1 {
		t.Fatalf("proof=%+v deletes=%d", db.proof, db.deleteCalls)
	}
}

func TestBATFinalizationRetriesTransientGetTx(t *testing.T) {
	owner := "ownerA"
	db := &batFinalizationDB{proof: cashu.Proof{Id: "id", Quote: &owner, State: cashu.PROOF_PENDING}, getTxErr: func(n int) error {
		if n == 1 {
			return io.ErrUnexpectedEOF
		}
		return nil
	}}
	if err := (&Mint{MintDB: db}).FinishAuthBlindToken(db.proof, true); err != nil {
		t.Fatal(err)
	}
	if db.getTxCalls != 2 || db.proof.State != cashu.PROOF_SPENT {
		t.Fatalf("calls=%d state=%v", db.getTxCalls, db.proof.State)
	}
}

func TestBATFinalizationRetriesAmbiguousCommit(t *testing.T) {
	owner := "ownerA"
	db := &batFinalizationDB{proof: cashu.Proof{Id: "id", Quote: &owner, State: cashu.PROOF_PENDING}}
	db.commit = func(n int, db *batFinalizationDB) error {
		if n == 1 {
			db.proof.State = cashu.PROOF_SPENT
			return io.ErrUnexpectedEOF
		}
		return nil
	}
	if err := (&Mint{MintDB: db}).FinishAuthBlindToken(db.proof, true); err != nil {
		t.Fatal(err)
	}
	if db.commitCalls != 1 || db.setCalls != 1 || db.proof.State != cashu.PROOF_SPENT {
		t.Fatalf("commits=%d writes=%d state=%v", db.commitCalls, db.setCalls, db.proof.State)
	}
}

func TestBATFinalizationOwnershipChangeDoesNotMutateReplacement(t *testing.T) {
	ownerA, ownerB := "ownerA", "ownerB"
	db := &batFinalizationDB{proof: cashu.Proof{Id: "id", Quote: &ownerB, State: cashu.PROOF_PENDING}}
	reservation := cashu.Proof{Id: "id", Quote: &ownerA, State: cashu.PROOF_PENDING}
	if err := (&Mint{MintDB: db}).FinishAuthBlindToken(reservation, false); err != nil {
		t.Fatal(err)
	}
	if db.deleteCalls != 0 || *db.proof.Quote != ownerB {
		t.Fatalf("replacement mutated: %+v", db.proof)
	}
	if err := (&Mint{MintDB: db}).FinishAuthBlindToken(reservation, true); err == nil {
		t.Fatal("success with changed owner should fail")
	}
}

func TestBATFinalizationRetriesBoundedAndHonorsReservationValidation(t *testing.T) {
	db := &batFinalizationDB{getTxErr: func(int) error { return errors.New("db unavailable") }}
	owner := "ownerA"
	proof := cashu.Proof{Quote: &owner}
	if err := (&Mint{MintDB: db}).FinishAuthBlindToken(proof, true); err == nil {
		t.Fatal("expected error")
	}
	if db.getTxCalls != 3 {
		t.Fatalf("attempts=%d", db.getTxCalls)
	}
	if !db.deadlineObserved {
		t.Fatal("GetTx did not receive the bounded finalization context")
	}
}

func TestBATFinalizationContextBoundsBlockingGetTx(t *testing.T) {
	owner := "ownerA"
	db := &batFinalizationDB{waitForCancellation: true}
	started := time.Now()
	err := (&Mint{MintDB: db}).FinishAuthBlindToken(cashu.Proof{Quote: &owner}, true)
	if err == nil {
		t.Fatal("expected timed out finalization")
	}
	if elapsed := time.Since(started); elapsed > 6*time.Second {
		t.Fatalf("finalization exceeded its 5-second budget: %v", elapsed)
	}
	if db.getTxCalls != 1 || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("GetTx calls=%d error=%v", db.getTxCalls, err)
	}
}
