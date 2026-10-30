//nolint:exhaustruct // Fixtures specify only the PostgreSQL diagnostics under test.
package postgresql

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/utils"
)

func TestLifecycleDatabaseErrorClassification(t *testing.T) {
	for _, tt := range []struct { //nolint:govet // Table layout favors readable fixtures over packing.
		code, constraint, table string
		want                    error
	}{
		{"23505", "blind_messages_pkey", "blind_messages", ErrOutputConflict},
		{"23505", "blind_messages_melt_position", "blind_messages", ErrOutputConflict},
		{"23505", "blind_messages_signature_id_key", "blind_messages", ErrOutputInvariant},
		{"23505", "blind_signatures_pkey", "blind_signatures", ErrOutputInvariant},
		{"23503", "blind_messages_signature_id_fkey", "blind_messages", ErrOutputReference},
		{"23503", "blind_messages_mint_quote_id_fkey", "blind_messages", ErrOutputReference},
		{"23503", "blind_messages_melt_quote_id_fkey", "blind_messages", ErrOutputReference},
		{"23514", "blind_messages_amount_check", "blind_messages", cashu.ErrInvalidBlindMessage},
		{"23514", "blind_messages_keyset_id_check", "blind_messages", cashu.ErrInvalidBlindMessage},
		{"23514", "blind_signatures_dleq_pair", "blind_signatures", cashu.ErrInvalidBlindMessage},
		{"23514", "blind_messages_state_check", "blind_messages", ErrOutputInvariant},
		{"23514", "blind_messages_signature_state", "blind_messages", ErrOutputInvariant},
		{"23514", "blind_messages_change_owner", "blind_messages", ErrOutputInvariant},
		{"23514", "blind_messages_known_amount", "blind_messages", ErrOutputInvariant},
		{"23514", "blind_messages_signed_immutable", "blind_messages", ErrSignedOutputImmutable},
		{"23514", "blind_signatures_immutable", "blind_signatures", ErrSignedOutputImmutable},
		{"23514", "blind_messages_check", "blind_messages", ErrOutputInvariant},
		{"23514", "blind_signatures_check", "blind_signatures", cashu.ErrInvalidBlindMessage},
		{"23502", "", "blind_messages", cashu.ErrInvalidBlindMessage},
		{"23502", "", "blind_signatures", cashu.ErrInvalidBlindMessage},
		{"55000", "blind_message_lifecycle_lossless_downgrade", "", ErrUnsafeMigration},
		{"55000", "melt_change_lifecycle_lossless_downgrade", "", ErrUnsafeMigration},
		{"23514", "unrelated_check", "other_table", nil},
		{"23502", "", "proofs", nil},
		{"23505", "unrelated_key", "other_table", nil},
		{"23503", "unrelated_fkey", "other_table", nil},
		{"55000", "unrelated_guard", "", nil},
		{"P0001", "blind_messages_signed_immutable", "blind_messages", nil},
	} {
		t.Run(tt.code+"/"+tt.constraint+"/"+tt.table, func(t *testing.T) {
			original := &pgconn.PgError{Code: tt.code, ConstraintName: tt.constraint, TableName: tt.table, Message: "localized message must not affect classification"}
			err := fmt.Errorf("application: %w", databaseError(fmt.Errorf("batch or migration: %w", original)))
			if !errors.Is(err, ErrDB) {
				t.Fatal("lost database error identity")
			}
			var recovered *pgconn.PgError
			if !errors.As(err, &recovered) || recovered != original {
				t.Fatal("lost original PostgreSQL diagnostics")
			}
			if tt.want != nil && !errors.Is(err, tt.want) {
				t.Fatalf("error=%v want=%v", err, tt.want)
			}
			if tt.want == nil {
				for _, kind := range []error{ErrOutputConflict, ErrOutputReference, ErrOutputInvariant, ErrSignedOutputImmutable, ErrUnsafeMigration, cashu.ErrInvalidBlindMessage} {
					if errors.Is(err, kind) {
						t.Fatalf("unrelated SQL error classified as %v", kind)
					}
				}
			}
			if errors.Is(err, cashu.ErrBlindMessageAlreadySigned) || errors.Is(err, cashu.ErrQuoteIsPending) || errors.Is(err, cashu.ErrOutputPending) {
				t.Fatal("guessed existing output state from a constraint violation")
			}
		})
	}
	if databaseError(nil) != nil {
		t.Fatal("nil error became a database failure")
	}
}

func TestLifecycleDatabaseErrorsReachApplicationMapping(t *testing.T) {
	invalid := databaseError(&pgconn.PgError{Code: "23514", ConstraintName: "blind_messages_amount_check", Message: "private SQL diagnostic"})
	code, detail := utils.ParseErrorToCashuErrorCode(fmt.Errorf("mint operation: %w", invalid))
	if code != cashu.PROOF_VERIFICATION_FAILED || detail == nil || *detail != cashu.ErrInvalidBlindMessage.Error() {
		t.Fatalf("invalid output mapping=%v detail=%v", code, detail)
	}
	for _, constraint := range []string{"blind_messages_signed_immutable", "blind_messages_signature_state"} {
		err := databaseError(&pgconn.PgError{Code: "23514", ConstraintName: constraint, Message: "private SQL diagnostic"})
		code, detail := utils.ParseErrorToCashuErrorCode(err)
		if code != cashu.UNKNOWN || detail != nil {
			t.Fatalf("server invariant exposed as client fault: %v %v", code, detail)
		}
	}
}

type lifecycleCommitErrorTx struct {
	pgx.Tx
	err error
}

func (tx lifecycleCommitErrorTx) Commit(context.Context) error { return tx.err }

func TestLifecycleCommitAndBatchErrorsKeepClassification(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "23503", ConstraintName: "blind_messages_signature_id_fkey"}
	err := (Postgresql{}).Commit(context.Background(), lifecycleCommitErrorTx{err: pgErr})
	if !errors.Is(err, ErrOutputReference) {
		t.Fatalf("commit error was not classified: %v", err)
	}
	var recovered *pgconn.PgError
	if !errors.As(err, &recovered) || recovered != pgErr {
		t.Fatal("commit lost PostgreSQL error")
	}
	if err := (Postgresql{}).Commit(context.Background(), lifecycleCommitErrorTx{}); err != nil {
		t.Fatal(err)
	}
	tx := &lifecycleTx{batchErr: pgErr}
	err = (Postgresql{}).SaveRestoreSigs(tx, []cashu.RecoverSigDB{lifecycleSignature(1)})
	if !errors.Is(err, ErrOutputReference) || !errors.As(err, &recovered) || recovered != pgErr {
		t.Fatalf("batch error was not classified: %v", err)
	}
	if !errors.Is(databaseError(context.Canceled), context.Canceled) {
		t.Fatal("database wrapping lost cancellation")
	}
}
