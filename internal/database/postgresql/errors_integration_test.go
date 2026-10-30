//nolint:exhaustruct // Fixtures specify only fields relevant to the constraint under test.
package postgresql

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lescuer97/nutmix/api/cashu"
)

func TestLifecycleConstraintAndTriggerErrorMetadata(t *testing.T) {
	db, ctx := setupTestDB(t)
	t.Cleanup(db.Close)
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := db.SaveMeltRequest(tx, cashu.MeltRequestDB{Quote: "q", Request: "invoice", Unit: "sat", State: cashu.PENDING}); err != nil {
		t.Fatal(err)
	}
	signed, blank := lifecycleSignature(1), lifecycleSignature(2)
	if err := db.SaveRestoreSigs(tx, []cashu.RecoverSigDB{signed}); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveMeltChange(tx, []cashu.BlindedMessage{blank.GetBlindedMessage()}, "q"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct { //nolint:govet // Table layout favors readable fixtures over packing.
		name, sql, constraint string
		want                  error
	}{
		{"duplicate point", `INSERT INTO blind_messages (blinded_message, amount, keyset_id, state, created_at)
		 SELECT blinded_message, 1, 'keyset', 'PENDING', 1 FROM blind_messages WHERE state = 'SIGNED'`, "blind_messages_pkey", ErrOutputConflict},
		{"duplicate change position", `INSERT INTO blind_messages (blinded_message, keyset_id, state, created_at, melt_quote_id, output_position)
		 VALUES (decode('ff','hex'), 'keyset', 'PENDING', 1, 'q', 0)`, "blind_messages_melt_position", ErrOutputConflict},
		{"shared signature", `INSERT INTO blind_messages (blinded_message, amount, keyset_id, state, signature_id, created_at)
		 SELECT decode('ff','hex'), 1, 'keyset', 'SIGNED', signature_id, 1 FROM blind_messages WHERE state = 'SIGNED'`, "blind_messages_signature_id_key", ErrOutputInvariant},
		{"missing signature", `INSERT INTO blind_messages (blinded_message, amount, keyset_id, state, signature_id, created_at)
		 VALUES (decode('ff','hex'), 1, 'keyset', 'SIGNED', -1, 1)`, "blind_messages_signature_id_fkey", ErrOutputReference},
		{"missing mint quote", `INSERT INTO blind_messages (blinded_message, amount, keyset_id, state, mint_quote_id, created_at)
		 VALUES (decode('ff','hex'), 1, 'keyset', 'PENDING', 'missing', 1)`, "blind_messages_mint_quote_id_fkey", ErrOutputReference},
		{"missing melt quote", `INSERT INTO blind_messages (blinded_message, keyset_id, state, melt_quote_id, output_position, created_at)
		 VALUES (decode('ff','hex'), 'keyset', 'PENDING', 'missing', 1, 1)`, "blind_messages_melt_quote_id_fkey", ErrOutputReference},
		{"invalid amount", `INSERT INTO blind_messages (blinded_message, amount, keyset_id, state, created_at)
		 VALUES (decode('ff','hex'), 0, 'keyset', 'PENDING', 1)`, "blind_messages_amount_check", cashu.ErrInvalidBlindMessage},
		{"invalid keyset", `INSERT INTO blind_messages (blinded_message, amount, keyset_id, state, created_at)
		 VALUES (decode('ff','hex'), 1, '', 'PENDING', 1)`, "blind_messages_keyset_id_check", cashu.ErrInvalidBlindMessage},
		{"incomplete DLEQ", `INSERT INTO blind_signatures (signature, dleq_e, created_at)
		 VALUES (decode('ff','hex'), decode('01','hex'), 1)`, "blind_signatures_dleq_pair", cashu.ErrInvalidBlindMessage},
		{"signed without signature", `INSERT INTO blind_messages (blinded_message, amount, keyset_id, state, created_at)
		 VALUES (decode('ff','hex'), 1, 'keyset', 'SIGNED', 1)`, "blind_messages_signature_state", ErrOutputInvariant},
		{"invalid change owner", `INSERT INTO blind_messages (blinded_message, keyset_id, state, melt_quote_id, created_at)
		 VALUES (decode('ff','hex'), 'keyset', 'PENDING', 'q', 1)`, "blind_messages_change_owner", ErrOutputInvariant},
		{"known blank amount", `INSERT INTO blind_messages (blinded_message, amount, keyset_id, state, melt_quote_id, output_position, created_at)
		 VALUES (decode('ff','hex'), 1, 'keyset', 'PENDING', 'q', 1, 1)`, "blind_messages_known_amount", ErrOutputInvariant},
		{"update signed message", "UPDATE blind_messages SET amount = 2 WHERE state = 'SIGNED'", "blind_messages_signed_immutable", ErrSignedOutputImmutable},
		{"delete signed message", "DELETE FROM blind_messages WHERE state = 'SIGNED'", "blind_messages_signed_immutable", ErrSignedOutputImmutable},
		{"update signature", "UPDATE blind_signatures SET signature = decode('ff','hex')", "blind_signatures_immutable", ErrSignedOutputImmutable},
		{"truncate recovery", "TRUNCATE blind_signatures, blind_messages", "blind_signatures_immutable", ErrSignedOutputImmutable},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, cause := db.pool.Exec(ctx, tt.sql)
			err := databaseError(cause)
			if cause == nil || !errors.Is(err, tt.want) {
				t.Fatalf("error=%v want=%v", err, tt.want)
			}
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.ConstraintName != tt.constraint {
				t.Fatalf("missing stable constraint metadata: %+v", pgErr)
			}
		})
	}
	for _, name := range []string{"42_blind_message_lifecycle.sql", "43_melt_change_lifecycle.sql"} {
		err := databaseError(runLifecycleMigration(t, db, ctx, name, false))
		if !errors.Is(err, ErrUnsafeMigration) {
			t.Fatalf("downgrade guard not classified: %v", err)
		}
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "55000" || pgErr.ConstraintName == "" {
			t.Fatalf("missing downgrade diagnostics: %+v", pgErr)
		}
	}
}
