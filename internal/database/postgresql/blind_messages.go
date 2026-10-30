package postgresql

import (
	"context"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
)

// SaveRestoreSigs persists newly signed outputs for every signing path. It does
// not finalize existing PENDING messages: mint-owner finalization is separate.
// The caller must commit the transaction before returning signatures.
func (pql Postgresql) SaveRestoreSigs(tx pgx.Tx, signatures []cashu.RecoverSigDB) error {
	if len(signatures) == 0 {
		return nil
	}
	ctx := context.Background()
	points := make([]cashu.WrappedPublicKey, len(signatures))
	for i, sig := range signatures {
		if sig.B_.PublicKey == nil || sig.C_.PublicKey == nil || sig.Amount == 0 || sig.Id == "" || sig.MeltQuote != "" ||
			(sig.Dleq != nil && (sig.Dleq.E == nil || sig.Dleq.S == nil)) {
			return cashu.ErrInvalidBlindMessage
		}
		points[i] = sig.B_
	}
	if err := lockBlindMessagePoints(tx, points); err != nil {
		return err
	}
	if err := pql.CheckBlindMessagesAvailable(tx, points); err != nil {
		return err
	}

	var batch pgx.Batch
	for _, sig := range signatures {
		e, s := recoveryDLEQBytes(sig)
		// The CTE creates and links the pair in one statement. A uniqueness or
		// foreign-key error aborts the transaction rather than leaving an orphan.
		batch.Queue(`WITH signature AS (
			INSERT INTO blind_signatures (signature, dleq_e, dleq_s, created_at)
			VALUES ($1, $2, $3, $4) RETURNING id
		)
		INSERT INTO blind_messages (blinded_message, amount, keyset_id, state, signature_id, mint_quote_id, created_at)
		SELECT $5, $6, $7, 'SIGNED', id, NULLIF($8, ''), $4 FROM signature`,
			sig.C_, e, s, sig.CreatedAt, sig.B_, sig.Amount, sig.Id, sig.MintQuote)
	}
	if err := tx.SendBatch(ctx, &batch).Close(); err != nil {
		return databaseError(fmt.Errorf("persist signed blinded messages: %w", err))
	}
	return nil
}

func recoveryDLEQBytes(sig cashu.RecoverSigDB) ([]byte, []byte) {
	if sig.Dleq == nil {
		return nil, nil
	}
	e, s := sig.Dleq.E.Key.Bytes(), sig.Dleq.S.Key.Bytes()
	return e[:], s[:]
}

func lockBlindMessagePoints(tx pgx.Tx, points []cashu.WrappedPublicKey) error {
	if len(points) == 0 {
		return nil
	}
	keys := make([]string, len(points))
	for i, point := range points {
		if point.PublicKey == nil {
			return cashu.ErrInvalidBlindMessage
		}
		keys[i] = point.ToHex()
	}
	sort.Strings(keys)
	for i, key := range keys {
		if i > 0 && key == keys[i-1] {
			return cashu.ErrRepeatedOutput
		}
	}
	// Missing rows cannot be row-locked. All signing paths use the same sorted
	// transaction-scoped keys, also usable by the later mint-reservation change.
	var locks pgx.Batch
	for _, key := range keys {
		locks.Queue("SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", key)
	}
	if err := tx.SendBatch(context.Background(), &locks).Close(); err != nil {
		return databaseError(fmt.Errorf("lock blinded messages: %w", err))
	}
	return nil
}

// Availability checks are advisory reads. Writers must also claim the sorted
// output locks and recheck before inserting or signing.
func (pql Postgresql) CheckBlindMessagesAvailable(tx pgx.Tx, points []cashu.WrappedPublicKey) error {
	if len(points) == 0 {
		return nil
	}
	for _, point := range points {
		if point.PublicKey == nil {
			return cashu.ErrInvalidBlindMessage
		}
	}
	rows, err := tx.Query(context.Background(), "SELECT state FROM blind_messages WHERE blinded_message = ANY($1)", points)
	if err != nil {
		return databaseError(err)
	}
	var conflict error
	for rows.Next() {
		var state string
		if err := rows.Scan(&state); err != nil {
			rows.Close()
			return databaseError(err)
		}
		if state == "SIGNED" {
			conflict = cashu.ErrBlindMessageAlreadySigned
		} else if conflict == nil {
			conflict = cashu.ErrOutputPending
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return databaseError(err)
	}
	return conflict
}
