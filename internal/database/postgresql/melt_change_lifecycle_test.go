//nolint:exhaustruct_v5 // Fixtures specify only fields relevant to the lifecycle scenario.
package postgresql

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lescuer97/nutmix/api/cashu"
)

func TestMeltChangeConcurrentClaimAndSignature(t *testing.T) {
	db, baseCtx := setupTestDB(t)
	t.Cleanup(db.Close)
	ctx, cancel := context.WithTimeout(baseCtx, 10*time.Second)
	defer cancel()
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(baseCtx) }()
	if err := db.SaveMeltRequest(tx, cashu.MeltRequestDB{Quote: "q", Request: "invoice", Unit: "sat", State: cashu.PENDING}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	first, second := lifecycleSignature(1), lifecycleSignature(2)
	start, results := make(chan struct{}), make(chan error, 2)
	for _, reserve := range []bool{true, false} {
		go func(reserve bool) {
			<-start
			workerTx, err := db.GetTx(ctx)
			if err != nil {
				results <- err
				return
			}
			defer func() { _ = workerTx.Rollback(baseCtx) }()
			if reserve {
				err = db.ReserveMeltChangeOutputs(ctx, workerTx, []cashu.BlindedMessage{second.GetBlindedMessage(), first.GetBlindedMessage()}, "q")
			} else {
				err = db.InsertSignedOutputs(ctx, workerTx, []cashu.SignedOutput{first, second})
			}
			if err == nil {
				err = workerTx.Commit(ctx)
			}
			results <- err
		}(reserve)
	}
	close(start)
	successes := 0
	for range 2 {
		err := <-results
		if err == nil {
			successes++
		} else if !errors.Is(err, cashu.ErrOutputPending) && !errors.Is(err, cashu.ErrBlindMessageAlreadySigned) {
			t.Fatalf("unexpected competing-writer error: %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("successful writers=%d", successes)
	}
	var messages, signatures, signed int
	if err := db.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM blind_messages),
		(SELECT count(*) FROM blind_signatures), (SELECT count(*) FROM blind_messages WHERE state = 'SIGNED')`).Scan(&messages, &signatures, &signed); err != nil {
		t.Fatal(err)
	}
	if messages != 2 || signatures != signed || (signed != 0 && signed != 2) {
		t.Fatalf("partial/orphan persistence: messages=%d signatures=%d signed=%d", messages, signatures, signed)
	}
}

func TestMeltChangeMessageMigration(t *testing.T) {
	db, ctx := setupTestDB(t)
	t.Cleanup(db.Close)
	migrate := func(up bool) error { return runLifecycleMigration(t, db, ctx, "43_melt_change_lifecycle.sql", up) }
	if err := migrate(false); err != nil {
		t.Fatal(err)
	}
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := db.SaveMeltRequest(tx, cashu.MeltRequestDB{Quote: "q", Request: "invoice", Unit: "sat", State: cashu.PENDING}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	first, second := lifecycleSignature(1), lifecycleSignature(2)
	if _, err := db.pool.Exec(ctx, `INSERT INTO melt_change_message ("B_", id, quote, created_at)
		VALUES ($1, 'keyset', 'q', 20), ($2, 'keyset', 'q', 10)`, first.B_, second.B_); err != nil {
		t.Fatal(err)
	}
	if err := migrate(true); err != nil {
		t.Fatal(err)
	}
	read, err := db.GetTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = read.Rollback(ctx) }()
	pending, err := db.LoadPendingMeltChangeForUpdate(ctx, read, "q")
	if err != nil || len(pending) != 2 || pending[0].B_.ToHex() != second.B_.ToHex() || pending[0].CreatedAt != 10 {
		t.Fatalf("legacy blanks not preserved deterministically: %v %v", pending, err)
	}
	if err := read.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if err := migrate(false); err == nil {
		t.Fatal("downgrade discarded pending change")
	}
	clear, err := db.GetTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = clear.Rollback(ctx) }()
	if err := db.ReleasePendingMeltChangeOutputs(ctx, clear, "q"); err != nil {
		t.Fatal(err)
	}
	if err := clear.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := migrate(false); err != nil {
		t.Fatal(err)
	}

	for _, scenario := range []string{"duplicate", "signed overlap", "missing quote"} {
		t.Run(scenario, func(t *testing.T) {
			if _, err := db.pool.Exec(ctx, "TRUNCATE melt_change_message"); err != nil {
				t.Fatal(err)
			}
			quote := "q"
			point := first.B_
			if scenario == "missing quote" {
				quote, point = "missing", lifecycleSignature(3).B_
			}
			if scenario == "signed overlap" {
				tx, err := db.GetTx(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = tx.Rollback(ctx) }()
				if err := db.InsertSignedOutputs(ctx, tx, []cashu.SignedOutput{first}); err != nil {
					t.Fatal(err)
				}
				if err := tx.Commit(ctx); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := db.pool.Exec(ctx, `INSERT INTO melt_change_message ("B_", id, quote, created_at) VALUES ($1, 'keyset', $2, 1)`, point, quote); err != nil {
				t.Fatal(err)
			}
			if scenario == "duplicate" {
				if _, err := db.pool.Exec(ctx, `INSERT INTO melt_change_message ("B_", id, quote, created_at)
					SELECT "B_", id, quote, created_at FROM melt_change_message`); err != nil {
					t.Fatal(err)
				}
			}
			if err := migrate(true); err == nil {
				t.Fatal("conflicting legacy data migrated")
			}
			var legacy *string
			if err := db.pool.QueryRow(ctx, "SELECT to_regclass('melt_change_message')::text").Scan(&legacy); err != nil || legacy == nil {
				t.Fatalf("failed migration dropped old data: %v %v", legacy, err)
			}
		})
	}
}
