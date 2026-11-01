package postgresql

import (
	"context"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
)

// InsertSignedOutputs atomically creates signed messages and signatures without
// assigning quote provenance. Use InsertMintSignedOutputs for mint ownership.
// Existing pending outputs cannot be finalized here; callers must commit before
// exposing signatures.
func (pql Postgresql) InsertSignedOutputs(ctx context.Context, tx pgx.Tx, signatures []cashu.SignedOutput) error {
	return pql.insertSignedOutputs(ctx, tx, "", signatures)
}

// InsertMintSignedOutputs atomically creates signed outputs owned by quoteID.
func (pql Postgresql) InsertMintSignedOutputs(ctx context.Context, tx pgx.Tx, quoteID string, signatures []cashu.SignedOutput) error {
	if quoteID == "" {
		return cashu.ErrInvalidBlindMessage
	}
	return pql.insertSignedOutputs(ctx, tx, quoteID, signatures)
}

func (pql Postgresql) insertSignedOutputs(ctx context.Context, tx pgx.Tx, quoteID string, signatures []cashu.SignedOutput) error {
	if len(signatures) == 0 {
		return nil
	}
	points := make([]cashu.WrappedPublicKey, len(signatures))
	for i, sig := range signatures {
		if sig.B_.PublicKey == nil || sig.C_.PublicKey == nil || sig.Amount == 0 || sig.Id == "" ||
			(sig.Dleq != nil && (sig.Dleq.E == nil || sig.Dleq.S == nil)) {
			return cashu.ErrInvalidBlindMessage
		}
		points[i] = sig.B_
	}
	if err := lockBlindMessagePoints(ctx, tx, points); err != nil {
		return err
	}
	if err := pql.CheckBlindMessagesAvailable(ctx, tx, points); err != nil {
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
			sig.C_, e, s, sig.CreatedAt, sig.B_, sig.Amount, sig.Id, quoteID)
	}
	if err := tx.SendBatch(ctx, &batch).Close(); err != nil {
		return databaseError(fmt.Errorf("persist signed blinded messages: %w", err))
	}
	return nil
}

func recoveryDLEQBytes(sig cashu.SignedOutput) ([]byte, []byte) {
	if sig.Dleq == nil {
		return nil, nil
	}
	e, s := sig.Dleq.E.Key.Bytes(), sig.Dleq.S.Key.Bytes()
	return e[:], s[:]
}

func lockBlindMessagePoints(ctx context.Context, tx pgx.Tx, points []cashu.WrappedPublicKey) error {
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
	if err := tx.SendBatch(ctx, &locks).Close(); err != nil {
		return databaseError(fmt.Errorf("lock blinded messages: %w", err))
	}
	return nil
}

// Availability checks are advisory reads. Writers must also claim the sorted
// output locks and recheck before inserting or signing.
func (pql Postgresql) CheckBlindMessagesAvailable(ctx context.Context, tx pgx.Tx, points []cashu.WrappedPublicKey) error {
	if len(points) == 0 {
		return nil
	}
	for _, point := range points {
		if point.PublicKey == nil {
			return cashu.ErrInvalidBlindMessage
		}
	}
	rows, err := tx.Query(ctx, "SELECT state FROM blind_messages WHERE blinded_message = ANY($1)", points)
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
