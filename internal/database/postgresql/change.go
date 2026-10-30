package postgresql

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lescuer97/nutmix/api/cashu"
)

func lockMeltChangeQuote(tx pgx.Tx, quote string) (cashu.ACTION_STATE, error) {
	var state cashu.ACTION_STATE
	err := tx.QueryRow(context.Background(), "SELECT state FROM melt_request WHERE quote = $1 FOR UPDATE NOWAIT", quote).Scan(&state)
	if err != nil {
		return state, databaseError(err)
	}
	return state, nil
}

// SaveMeltChange claims blank outputs in the same transaction as melt inputs.
// A blank's denomination is unknown until settlement, so its stored amount is NULL.
func (pql Postgresql) SaveMeltChange(tx pgx.Tx, change []cashu.BlindedMessage, quote string) error {
	if len(change) == 0 {
		return nil
	}
	state, err := lockMeltChangeQuote(tx, quote)
	if err != nil {
		return err
	}
	if state != cashu.PENDING {
		return cashu.ErrQuoteIsPending
	}
	points := make([]cashu.WrappedPublicKey, len(change))
	for i, output := range change {
		if output.B_.PublicKey == nil || output.Id == "" {
			return cashu.ErrInvalidBlindMessage
		}
		points[i] = output.B_
	}
	if err := lockBlindMessagePoints(tx, points); err != nil {
		return err
	}
	if err := pql.CheckBlindMessagesAvailable(tx, points); err != nil {
		return err
	}
	entries := make([][]any, len(change))
	now := time.Now().Unix()
	for i, output := range change {
		entries[i] = []any{output.B_, output.Id, "PENDING", quote, i, now}
	}
	_, err = tx.CopyFrom(context.Background(), pgx.Identifier{"blind_messages"},
		[]string{"blinded_message", "keyset_id", "state", "melt_quote_id", "output_position", "created_at"}, pgx.CopyFromRows(entries))
	if err != nil {
		return databaseError(fmt.Errorf("reserve melt change messages: %w", err))
	}
	return nil
}

func (pql Postgresql) GetMeltChangeByQuote(tx pgx.Tx, quote string) ([]cashu.MeltChange, error) {
	if _, err := lockMeltChangeQuote(tx, quote); err != nil {
		return nil, err
	}
	rows, err := tx.Query(context.Background(), `SELECT blinded_message AS "B_", keyset_id AS id,
		melt_quote_id AS quote, created_at FROM blind_messages
		WHERE melt_quote_id = $1 AND state = 'PENDING'
		ORDER BY output_position FOR UPDATE`, quote)
	if err != nil {
		return nil, databaseError(err)
	}
	change, err := pgx.CollectRows(rows, pgx.RowToStructByName[cashu.MeltChange])
	if err != nil {
		return nil, databaseError(err)
	}
	return change, nil
}

// DeleteChangeByQuote releases only unsigned blanks. Signed change remains
// linked to the melt quote permanently, for recovery and repeated quote reads.
func (pql Postgresql) DeleteChangeByQuote(tx pgx.Tx, quote string) error {
	if _, err := lockMeltChangeQuote(tx, quote); err != nil {
		return err
	}
	_, err := tx.Exec(context.Background(), `DELETE FROM blind_messages
		WHERE melt_quote_id = $1 AND state = 'PENDING' AND signature_id IS NULL`, quote)
	if err != nil {
		return databaseError(err)
	}
	return nil
}

// FinalizeMeltChange signs only the selected prefix of this quote's ordered
// blanks and releases the unused suffix, including all blanks for zero refund.
// Quote/proof settlement and this operation must share the caller's transaction.
func (pql Postgresql) FinalizeMeltChange(tx pgx.Tx, quote string, signatures []cashu.RecoverSigDB) error {
	state, err := lockMeltChangeQuote(tx, quote)
	if err != nil {
		return err
	}
	if state != cashu.PENDING && state != cashu.PAID {
		return cashu.ErrQuoteIsPending
	}
	pending, err := pql.GetMeltChangeByQuote(tx, quote)
	if err != nil {
		return err
	}
	if len(signatures) > len(pending) {
		return cashu.ErrQuoteIsPending
	}
	points := make([]cashu.WrappedPublicKey, len(signatures))
	for i, sig := range signatures {
		if sig.B_.PublicKey == nil || sig.C_.PublicKey == nil || sig.Amount == 0 || sig.MintQuote != "" ||
			(sig.MeltQuote != "" && sig.MeltQuote != quote) ||
			(sig.Dleq != nil && (sig.Dleq.E == nil || sig.Dleq.S == nil)) {
			return cashu.ErrInvalidBlindMessage
		}
		if sig.B_.ToHex() != pending[i].B_.ToHex() || sig.Id != pending[i].Id {
			return cashu.ErrInvalidBlindMessage
		}
		points[i] = sig.B_
	}
	if err := lockBlindMessagePoints(tx, points); err != nil {
		return err
	}
	var batch pgx.Batch
	for _, sig := range signatures {
		e, s := recoveryDLEQBytes(sig)
		batch.Queue(`WITH candidate AS (
			SELECT blinded_message FROM blind_messages
			WHERE blinded_message = $5 AND melt_quote_id = $7 AND keyset_id = $8 AND state = 'PENDING'
			FOR UPDATE
		), signature AS (
			INSERT INTO blind_signatures (signature, dleq_e, dleq_s, created_at)
			SELECT $1, $2, $3, $4 FROM candidate RETURNING id
		)
		UPDATE blind_messages m SET amount = $6, state = 'SIGNED', signature_id = signature.id
		FROM signature, candidate WHERE m.blinded_message = candidate.blinded_message AND m.state = 'PENDING'`,
			sig.C_, e, s, sig.CreatedAt, sig.B_, sig.Amount, quote, sig.Id).Exec(func(tag pgconn.CommandTag) error {
			if tag.RowsAffected() != 1 {
				return cashu.ErrQuoteIsPending
			}
			return nil
		})
	}
	if len(signatures) > 0 {
		if err := tx.SendBatch(context.Background(), &batch).Close(); err != nil {
			return databaseError(err)
		}
	}
	return pql.DeleteChangeByQuote(tx, quote)
}
