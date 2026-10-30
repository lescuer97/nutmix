//nolint:exhaustruct,contextcheck // Minimal fixtures exercise the existing context-free database interface.
package postgresql

import (
	"bytes"
	"context"
	"errors"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
)

func TestBlindMessageLifecycleStorage(t *testing.T) {
	db, ctx := setupTestDB(t)
	t.Cleanup(db.Close)
	base := lifecycleSignature(1)
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := db.SaveMintRequest(tx, cashu.MintRequestDB{Quote: "mint-origin", Request: "invoice", Unit: "sat", State: cashu.ISSUED, Minted: true}); err != nil {
		t.Fatal(err)
	}
	base.MintQuote, base.Amount = "mint-origin", math.MaxUint64
	base.Dleq = &cashu.BlindSignatureDLEQ{E: secp256k1.PrivKeyFromBytes([]byte{1}), S: secp256k1.PrivKeyFromBytes([]byte{2})}
	if err := db.SaveRestoreSigs(tx, []cashu.RecoverSigDB{base}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	t.Run("recovery and statistics retain uint64 metadata", func(t *testing.T) {
		tx, err := db.GetTx(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		sigs, err := db.GetRestoreSigsFromBlindedMessages(tx, []cashu.WrappedPublicKey{base.B_})
		if err != nil || len(sigs) != 1 {
			t.Fatalf("recovery=%v err=%v", sigs, err)
		}
		got := sigs[0]
		if got.Amount != base.Amount || got.Id != base.Id || got.MintQuote != base.MintQuote || got.CreatedAt != base.CreatedAt || got.C_.ToHex() != base.C_.ToHex() || got.Dleq == nil || !bytes.Equal(got.Dleq.E.Serialize(), base.Dleq.E.Serialize()) || !bytes.Equal(got.Dleq.S.Serialize(), base.Dleq.S.Serialize()) {
			t.Fatalf("recovery changed metadata: %+v", got)
		}
		stats, err := db.GetBlindSigStatsRows(ctx, tx, base.CreatedAt, base.CreatedAt)
		if err != nil || len(stats) != 1 || stats[0].Amount != base.Amount {
			t.Fatalf("stats=%v err=%v", stats, err)
		}
		if err := db.SaveRestoreSigs(tx, []cashu.RecoverSigDB{base}); !errors.Is(err, cashu.ErrBlindMessageAlreadySigned) {
			t.Fatalf("signed output reserved again: %v", err)
		}
	})

	for _, query := range []string{
		"UPDATE blind_messages SET amount = 2",
		"UPDATE blind_messages SET state = 'PENDING', signature_id = NULL",
		"DELETE FROM blind_messages",
		"UPDATE blind_signatures SET signature = decode('01', 'hex')",
		"DELETE FROM blind_signatures",
		"TRUNCATE blind_messages, blind_signatures",
		"DELETE FROM mint_request WHERE quote = 'mint-origin'",
		`INSERT INTO blind_messages (blinded_message, amount, keyset_id, state, signature_id, created_at)
		 SELECT decode('ff', 'hex'), 1, 'keyset', 'SIGNED', signature_id, 1 FROM blind_messages`,
		`INSERT INTO blind_messages (blinded_message, amount, keyset_id, state, created_at)
		 VALUES (decode('ff', 'hex'), 1, 'keyset', 'SIGNED', 1)`,
		`INSERT INTO blind_messages (blinded_message, amount, keyset_id, state, signature_id, created_at)
		 SELECT decode('ff', 'hex'), 1, 'keyset', 'PENDING', signature_id, 1 FROM blind_messages`,
		`INSERT INTO blind_messages (blinded_message, amount, keyset_id, state, created_at)
		 VALUES (decode('ff', 'hex'), 18446744073709551616, 'keyset', 'PENDING', 1)`,
		`INSERT INTO blind_messages (blinded_message, amount, keyset_id, state, created_at)
		 VALUES (decode('ff', 'hex'), 0, 'keyset', 'PENDING', 1)`,
		`INSERT INTO blind_signatures (signature, dleq_e, created_at) VALUES (decode('ff','hex'), decode('01','hex'), 1)`,
	} {
		t.Run(query, func(t *testing.T) {
			tx, err := db.GetTx(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback(ctx) }()
			if _, err := tx.Exec(ctx, query); err == nil {
				t.Fatal("invalid mutation was accepted")
			}
		})
	}

	t.Run("pending messages are not recovered or counted", func(t *testing.T) {
		tx, err := db.GetTx(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		pending := lifecycleSignature(3)
		if _, err := tx.Exec(ctx, `INSERT INTO blind_messages (blinded_message, amount, keyset_id, state, created_at)
		 VALUES ($1, 1, 'keyset', 'PENDING', 123)`, pending.B_); err != nil {
			t.Fatal(err)
		}
		sigs, err := db.GetRestoreSigsFromBlindedMessages(tx, []cashu.WrappedPublicKey{pending.B_})
		if err != nil || len(sigs) != 0 {
			t.Fatalf("pending recovery=%v err=%v", sigs, err)
		}
		stats, err := db.GetBlindSigStatsRows(ctx, tx, 0, 200)
		if err != nil || len(stats) != 1 {
			t.Fatalf("pending counted: %v err=%v", stats, err)
		}
		if err := db.SaveRestoreSigs(tx, []cashu.RecoverSigDB{pending}); !errors.Is(err, cashu.ErrOutputPending) {
			t.Fatalf("pending signed through generic writer: %v", err)
		}
	})

	t.Run("rollback removes both halves", func(t *testing.T) {
		tx, err := db.GetTx(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		if err := db.SaveRestoreSigs(tx, []cashu.RecoverSigDB{lifecycleSignature(4)}); err != nil {
			t.Fatal(err)
		}
		if err := tx.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
		assertLifecycleCounts(t, ctx, db, 1)
	})

	t.Run("failed batch leaves no orphan signature", func(t *testing.T) {
		tx, err := db.GetTx(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		valid, invalid := lifecycleSignature(5), lifecycleSignature(6)
		invalid.MintQuote = "missing-quote"
		if err := db.SaveRestoreSigs(tx, []cashu.RecoverSigDB{valid, invalid}); err == nil {
			t.Fatal("missing quote accepted")
		}
		if err := tx.Commit(ctx); !errors.Is(err, pgx.ErrTxCommitRollback) {
			t.Fatalf("failed batch committed: %v", err)
		}
		assertLifecycleCounts(t, ctx, db, 1)
	})
}

func assertLifecycleCounts(t *testing.T, ctx context.Context, db Postgresql, want int) {
	t.Helper()
	var messages, signatures int
	if err := db.pool.QueryRow(ctx, "SELECT (SELECT count(*) FROM blind_messages), (SELECT count(*) FROM blind_signatures)").Scan(&messages, &signatures); err != nil {
		t.Fatal(err)
	}
	if messages != want || signatures != want {
		t.Fatalf("messages=%d signatures=%d want=%d", messages, signatures, want)
	}
}

func TestBlindMessageLifecycleConcurrentWriters(t *testing.T) {
	db, baseCtx := setupTestDB(t)
	t.Cleanup(db.Close)
	ctx, cancel := context.WithTimeout(baseCtx, 10*time.Second)
	defer cancel()
	first, second := lifecycleSignature(1), lifecycleSignature(2)
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, sigs := range [][]cashu.RecoverSigDB{{first, second}, {second, first}} {
		go func(sigs []cashu.RecoverSigDB) {
			<-start
			tx, err := db.GetTx(ctx)
			if err != nil {
				results <- err
				return
			}
			defer func() { _ = tx.Rollback(baseCtx) }()
			err = db.SaveRestoreSigs(tx, sigs)
			if err == nil {
				err = tx.Commit(ctx)
			}
			results <- err
		}(sigs)
	}
	close(start)
	successes := 0
	for range 2 {
		err := <-results
		if err == nil {
			successes++
		} else if !errors.Is(err, cashu.ErrBlindMessageAlreadySigned) {
			t.Fatalf("unexpected writer error: %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("successful writers=%d", successes)
	}
	assertLifecycleCounts(t, ctx, db, 2)
}

func lifecycleMigration(t *testing.T, db Postgresql, ctx context.Context, up bool) error {
	return runLifecycleMigration(t, db, ctx, "42_blind_message_lifecycle.sql", up)
}

func runLifecycleMigration(t *testing.T, db Postgresql, ctx context.Context, name string, up bool) error {
	t.Helper()
	sql, err := os.ReadFile("../goose/migrations/" + name)
	if err != nil {
		t.Fatal(err)
	}
	upSQL, downSQL, ok := strings.Cut(string(sql), "-- +goose Down")
	if !ok {
		t.Fatal("missing down migration")
	}
	query := upSQL
	if !up {
		query = downSQL
	}
	tx, err := db.GetTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, query); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func TestBlindMessageLifecycleMigration(t *testing.T) {
	db, ctx := setupTestDB(t)
	t.Cleanup(db.Close)
	if err := runLifecycleMigration(t, db, ctx, "43_melt_change_lifecycle.sql", false); err != nil {
		t.Fatal(err)
	}
	if err := lifecycleMigration(t, db, ctx, false); err != nil {
		t.Fatal(err)
	}
	legacy := lifecycleSignature(1)
	if _, err := db.pool.Exec(ctx, `INSERT INTO recovery_signature (id, amount, "B_", "C_", created_at, dleq_e, dleq_s)
	 VALUES ($1, 7, $2, $3, $4, decode(repeat('01',32),'hex'), decode(repeat('02',32),'hex'))`, legacy.Id, legacy.B_, legacy.C_, legacy.CreatedAt); err != nil {
		t.Fatal(err)
	}
	if err := lifecycleMigration(t, db, ctx, true); err != nil {
		t.Fatal(err)
	}
	if err := runLifecycleMigration(t, db, ctx, "43_melt_change_lifecycle.sql", true); err != nil {
		t.Fatal(err)
	}
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	sigs, err := db.GetRestoreSigsFromBlindedMessages(tx, []cashu.WrappedPublicKey{legacy.B_})
	if err != nil || len(sigs) != 1 {
		t.Fatalf("migrated recovery=%v err=%v", sigs, err)
	}
	got := sigs[0]
	if got.Amount != 7 || got.Id != legacy.Id || got.CreatedAt != legacy.CreatedAt || got.C_.ToHex() != legacy.C_.ToHex() || got.MintQuote != "" || got.Dleq == nil {
		t.Fatalf("migration lost metadata: %+v", got)
	}
	if err := db.SaveRestoreSigs(tx, []cashu.RecoverSigDB{lifecycleSignature(3)}); err != nil {
		t.Fatalf("identity sequence was not advanced: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if err := runLifecycleMigration(t, db, ctx, "43_melt_change_lifecycle.sql", false); err != nil {
		t.Fatal(err)
	}
	if err := lifecycleMigration(t, db, ctx, false); err != nil {
		t.Fatal(err)
	}
	var amount uint64
	var b, c, e, s []byte
	var created int64
	if err := db.pool.QueryRow(ctx, `SELECT amount, "B_", "C_", created_at, dleq_e, dleq_s FROM recovery_signature`).Scan(&amount, &b, &c, &created, &e, &s); err != nil {
		t.Fatal(err)
	}
	if amount != 7 || string(b) != string(legacy.B_.SerializeCompressed()) || string(c) != string(legacy.C_.SerializeCompressed()) || created != legacy.CreatedAt || string(e) != strings.Repeat("\x01", 32) || string(s) != strings.Repeat("\x02", 32) {
		t.Fatal("downgrade changed recovery data")
	}

	for _, invalid := range []string{"amount = NULL", "amount = 0", "dleq_e = NULL"} {
		t.Run(invalid, func(t *testing.T) {
			if _, err := db.pool.Exec(ctx, "UPDATE recovery_signature SET "+invalid); err != nil {
				t.Fatal(err)
			}
			if err := lifecycleMigration(t, db, ctx, true); err == nil {
				t.Fatal("invalid legacy data migrated")
			}
			var relation *string
			if err := db.pool.QueryRow(ctx, "SELECT to_regclass('blind_messages')::text").Scan(&relation); err != nil || relation != nil {
				t.Fatalf("failed migration left new schema: %v %v", relation, err)
			}
			if _, err := db.pool.Exec(ctx, "UPDATE recovery_signature SET amount = 7, dleq_e = decode(repeat('01',32),'hex')"); err != nil {
				t.Fatal(err)
			}
		})
	}
	if err := lifecycleMigration(t, db, ctx, true); err != nil {
		t.Fatal(err)
	}
	if err := runLifecycleMigration(t, db, ctx, "43_melt_change_lifecycle.sql", true); err != nil {
		t.Fatal(err)
	}
	if _, err := db.pool.Exec(ctx, `INSERT INTO blind_messages (blinded_message, amount, keyset_id, state, created_at)
	 VALUES (decode('ff','hex'), 1, 'keyset', 'PENDING', 123)`); err != nil {
		t.Fatal(err)
	}
	if err := lifecycleMigration(t, db, ctx, false); err == nil {
		t.Fatal("downgrade discarded pending message")
	}
}
