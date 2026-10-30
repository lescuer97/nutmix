package postgresql

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lescuer97/nutmix/api/cashu"
)

// Lifecycle invariant failures are distinct from client errors. In particular,
// a primary-key conflict does not tell us whether the existing output is pending
// or signed, and must not be mislabeled as either without an availability read.
var (
	ErrOutputConflict        = errors.New("blinded output storage conflict")
	ErrOutputReference       = errors.New("invalid blinded output reference")
	ErrOutputInvariant       = errors.New("blinded output lifecycle invariant violated")
	ErrSignedOutputImmutable = errors.New("signed output recovery data is immutable")
	ErrUnsafeMigration       = errors.New("migration would discard lifecycle data")
)

// Keep both the application classification (errors.Is) and the original
// *pgconn.PgError (errors.As), including through batches and migration wrappers.
// Never classify by human-readable Message or Detail, which PostgreSQL can localize.
func databaseError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return errors.Join(ErrDB, lifecycleErrorKind(pgErr), err)
	}
	return errors.Join(ErrDB, err)
}

func lifecycleErrorKind(err *pgconn.PgError) error {
	switch err.Code {
	case "23505": // unique_violation
		switch err.ConstraintName {
		case "blind_messages_pkey", "blind_messages_melt_position":
			return ErrOutputConflict
		case "blind_messages_signature_id_key", "blind_signatures_pkey":
			return ErrOutputInvariant
		}
	case "23503": // foreign_key_violation
		switch err.ConstraintName {
		case "blind_messages_signature_id_fkey", "blind_messages_mint_quote_id_fkey", "blind_messages_melt_quote_id_fkey":
			return ErrOutputReference
		}
	case "23514": // check_violation, including named trigger exceptions
		switch err.ConstraintName {
		case "blind_messages_signed_immutable", "blind_signatures_immutable":
			return ErrSignedOutputImmutable
		case "blind_messages_amount_check", "blind_messages_keyset_id_check", "blind_signatures_dleq_pair", "blind_signatures_check":
			return cashu.ErrInvalidBlindMessage
		case "blind_messages_state_check", "blind_messages_signature_state", "blind_messages_check", "blind_messages_change_owner", "blind_messages_known_amount":
			return ErrOutputInvariant
		}
	case "23502": // not_null_violation
		if err.TableName == "blind_messages" || err.TableName == "blind_signatures" {
			return cashu.ErrInvalidBlindMessage
		}
	case "55000": // object_not_in_prerequisite_state
		switch err.ConstraintName {
		case "blind_message_lifecycle_lossless_downgrade", "melt_change_lifecycle_lossless_downgrade":
			return ErrUnsafeMigration
		}
	}
	return nil
}
