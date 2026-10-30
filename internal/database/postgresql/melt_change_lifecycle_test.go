//nolint:exhaustruct,contextcheck // Minimal fixtures exercise the existing context-free database interface.
package postgresql

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lescuer97/nutmix/api/cashu"
)

func TestMeltChangeMessageTransactions(t *testing.T) {
	db, ctx := setupTestDB(t)
	t.Cleanup(db.Close)
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for _, quote := range []string{"q1", "q2"} {
		if err := db.SaveMeltRequest(tx, cashu.MeltRequestDB{Quote: quote, Request: quote + "-invoice", Unit: "sat", State: cashu.PENDING}); err != nil {
			t.Fatal(err)
		}
	}
	first, second, unused := lifecycleSignature(3), lifecycleSignature(1), lifecycleSignature(2)
	second.Amount = 2
	outputs := []cashu.BlindedMessage{first.GetBlindedMessage(), second.GetBlindedMessage(), unused.GetBlindedMessage()}
	if err := db.SaveMeltChange(tx, outputs, "q1"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var unknown int
	if err := db.pool.QueryRow(ctx, "SELECT count(*) FROM blind_messages WHERE melt_quote_id = 'q1' AND amount IS NULL AND signature_id IS NULL").Scan(&unknown); err != nil || unknown != 3 {
		t.Fatalf("blank amounts known prematurely: %d %v", unknown, err)
	}

	check, err := db.GetTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = check.Rollback(ctx) }()
	pending, err := db.GetMeltChangeByQuote(check, "q1")
	if err != nil || len(pending) != 3 || pending[0].B_.ToHex() != first.B_.ToHex() || pending[1].B_.ToHex() != second.B_.ToHex() {
		t.Fatalf("wallet order changed: %v %v", pending, err)
	}
	if err := db.SaveRestoreSigs(check, []cashu.RecoverSigDB{first}); !errors.Is(err, cashu.ErrOutputPending) {
		t.Fatalf("generic writer signed pending blank: %v", err)
	}
	if err := db.FinalizeMeltChange(check, "q2", []cashu.RecoverSigDB{first}); !errors.Is(err, cashu.ErrQuoteIsPending) {
		t.Fatalf("wrong quote signed blank: %v", err)
	}
	changed := first
	changed.Id = "other-keyset"
	if err := db.FinalizeMeltChange(check, "q1", []cashu.RecoverSigDB{changed}); !errors.Is(err, cashu.ErrInvalidBlindMessage) {
		t.Fatalf("keyset substitution accepted: %v", err)
	}
	if err := db.FinalizeMeltChange(check, "q1", []cashu.RecoverSigDB{first, second}); err != nil {
		t.Fatal(err)
	}
	if err := db.ChangeMeltRequestState(check, "q1", cashu.PAID, true, 0); err != nil {
		t.Fatal(err)
	}
	if err := check.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var signatures int
	if err := db.pool.QueryRow(ctx, "SELECT count(*) FROM blind_signatures").Scan(&signatures); err != nil || signatures != 0 {
		t.Fatalf("rollback leaked signatures: %d %v", signatures, err)
	}

	finish, err := db.GetTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = finish.Rollback(ctx) }()
	quote, err := db.GetMeltRequestById(finish, "q1")
	if err != nil || quote.State != cashu.PENDING {
		t.Fatalf("rollback lost pending quote: %+v %v", quote, err)
	}
	pending, err = db.GetMeltChangeByQuote(finish, "q1")
	if err != nil || len(pending) != 3 {
		t.Fatalf("rollback lost blanks: %v %v", pending, err)
	}
	if err := db.FinalizeMeltChange(finish, "q1", []cashu.RecoverSigDB{first, second}); err != nil {
		t.Fatal(err)
	}
	if err := db.ChangeMeltRequestState(finish, "q1", cashu.PAID, true, 0); err != nil {
		t.Fatal(err)
	}
	if err := finish.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	read, err := db.GetTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = read.Rollback(ctx) }()
	quote, err = db.GetMeltRequestById(read, "q1")
	if err != nil || len(quote.Change) != 2 || quote.Change[0].Amount != 1 || quote.Change[1].Amount != 2 {
		t.Fatalf("reloaded quote lost ordered change: %+v %v", quote, err)
	}
	recovered, err := db.GetRestoreSigsFromBlindedMessages(read, []cashu.WrappedPublicKey{first.B_, second.B_, unused.B_})
	if err != nil || len(recovered) != 2 {
		t.Fatalf("recovery=%v err=%v", recovered, err)
	}
	for _, sig := range recovered {
		if sig.MeltQuote != "q1" {
			t.Fatal("change provenance lost")
		}
	}
	if err := db.DeleteChangeByQuote(read, "q1"); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveMeltChange(read, []cashu.BlindedMessage{unused.GetBlindedMessage()}, "q2"); err != nil {
		t.Fatalf("unused blank not released: %v", err)
	}
	if err := db.FinalizeMeltChange(read, "q2", nil); err != nil {
		t.Fatal(err)
	}
	if err := db.CheckBlindMessagesAvailable(read, []cashu.WrappedPublicKey{unused.B_}); err != nil {
		t.Fatalf("zero refund retained blank: %v", err)
	}
	if err := read.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	assertLifecycleCounts(t, ctx, db, 2)
}

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
			tx, err := db.GetTx(ctx)
			if err != nil {
				results <- err
				return
			}
			defer func() { _ = tx.Rollback(baseCtx) }()
			if reserve {
				err = db.SaveMeltChange(tx, []cashu.BlindedMessage{second.GetBlindedMessage(), first.GetBlindedMessage()}, "q")
			} else {
				err = db.SaveRestoreSigs(tx, []cashu.RecoverSigDB{first, second})
			}
			if err == nil {
				err = tx.Commit(ctx)
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
	pending, err := db.GetMeltChangeByQuote(read, "q")
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
	if err := db.DeleteChangeByQuote(clear, "q"); err != nil {
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
				if err := db.SaveRestoreSigs(tx, []cashu.RecoverSigDB{first}); err != nil {
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
