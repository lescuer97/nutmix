package postgresql

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/database/goose"
)

var ErrDB = errors.New("ERROR DATABASE")

var DATABASE_URL_ENV = "DATABASE_URL"

type Postgresql struct {
	pool *pgxpool.Pool
}

func databaseError(err error) error {
	return errors.Join(ErrDB, err)
}

func DatabaseSetup(ctx context.Context, migrationDir string) (Postgresql, error) {

	var postgresql Postgresql

	dbUrl := os.Getenv(DATABASE_URL_ENV)
	if dbUrl == "" {
		return postgresql, fmt.Errorf("%v enviroment variable empty", DATABASE_URL_ENV)

	}

	pool, err := pgxpool.New(context.Background(), dbUrl)
	if err != nil {
		return postgresql, fmt.Errorf("pgxpool.New: %w", err)
	}

	db := stdlib.OpenDBFromPool(pool)

	err = goose.RunMigration(db, goose.POSTGRES)
	if err := db.Close(); err != nil {
		panic(err)
	}

	if err != nil {
		return postgresql, databaseError(fmt.Errorf("error connecting to database: %w", err))
	}
	postgresql.pool = pool

	return postgresql, nil
}

func (pql Postgresql) GetTx(ctx context.Context) (pgx.Tx, error) {
	return pql.pool.Begin(ctx)
}
func (pql Postgresql) Commit(ctx context.Context, tx pgx.Tx) error {
	return tx.Commit(ctx)
}
func (pql Postgresql) Rollback(ctx context.Context, tx pgx.Tx) error {
	return tx.Rollback(ctx)
}
func (pql Postgresql) SubTx(ctx context.Context, tx pgx.Tx) (pgx.Tx, error) {
	return tx.Begin(ctx)
}

func (pql Postgresql) GetAllSeeds() ([]cashu.Seed, error) {
	var seeds []cashu.Seed

	rows, err := pql.pool.Query(context.Background(), `SELECT  created_at, active, version, unit, id,  "input_fee_ppk", final_expiry FROM seeds ORDER BY version DESC`)
	if err != nil {
		if err == pgx.ErrNoRows {
			return seeds, fmt.Errorf("no rows found: %w", err)
		}

		return seeds, fmt.Errorf("error checking for seeds: %w", err)
	}
	defer rows.Close()

	seeds_collect, err := pgx.CollectRows(rows, pgx.RowToStructByName[cashu.Seed])

	if err != nil {
		return seeds_collect, fmt.Errorf("collecting rows: %w", err)
	}

	return seeds_collect, nil
}

func (pql Postgresql) GetSeedsByUnit(tx pgx.Tx, unit cashu.Unit) ([]cashu.Seed, error) {
	rows, err := tx.Query(context.Background(), "SELECT  created_at, active, version, unit, id, input_fee_ppk, final_expiry FROM seeds WHERE unit = $1", unit.String())
	if err != nil {
		return []cashu.Seed{}, fmt.Errorf("error checking for active seeds: %w", err)
	}
	defer rows.Close()

	seeds, err := pgx.CollectRows(rows, pgx.RowToStructByName[cashu.Seed])

	if err != nil {
		if err == pgx.ErrNoRows {
			return seeds, nil
		}
		return seeds, databaseError(fmt.Errorf("pgx.CollectRows(rows, pgx.RowToStructByName[cashu.Seed]): %w", err))
	}

	return seeds, nil
}

func (pql Postgresql) SaveNewSeed(tx pgx.Tx, seed cashu.Seed) error {

	tries := 0

	for {
		tries += 1
		_, err := tx.Exec(context.Background(), "INSERT INTO seeds ( active, created_at, unit, id, version, input_fee_ppk, final_expiry) VALUES ($1, $2, $3, $4, $5, $6, $7)", seed.Active, seed.CreatedAt, seed.Unit, seed.Id, seed.Version, seed.InputFeePpk, seed.FinalExpiry)

		switch {
		case err != nil && tries < 3:
			continue
		case err != nil && tries >= 3:
			return databaseError(fmt.Errorf("inserting to seeds: %w", err))
		case err == nil:
			return nil
		}

	}
}

func (pql Postgresql) SaveNewSeeds(seeds []cashu.Seed) error {
	tries := 0

	entries := [][]any{}
	columns := []string{"active", "created_at", "unit", "id", "version", "input_fee_ppk", "final_expiry"}
	tableName := "seeds"

	for _, seed := range seeds {
		entries = append(entries, []any{seed.Active, seed.CreatedAt, seed.Unit, seed.Id, seed.Version, seed.InputFeePpk, seed.FinalExpiry})
	}

	for {
		tries += 1
		_, err := pql.pool.CopyFrom(context.Background(), pgx.Identifier{tableName}, columns, pgx.CopyFromRows(entries))

		switch {
		case err != nil && tries < 3:
			continue
		case err != nil && tries >= 3:
			return databaseError(fmt.Errorf("inserting seeds: %w", err))
		case err == nil:
			return nil
		}

	}

}

func (pql Postgresql) UpdateSeedsActiveStatus(tx pgx.Tx, seeds []cashu.Seed) error {
	// change the paid status of the quote
	batch := pgx.Batch{}
	for _, seed := range seeds {

		batch.Queue("UPDATE seeds SET active = $1 WHERE id = $2", seed.Active, seed.Id)

	}
	results := tx.SendBatch(context.Background(), &batch)
	defer func() {
		if err := results.Close(); err != nil {
			slog.Error("failed to close results", slog.Any("error", err))
		}
	}()

	rows, err := results.Query()
	if err != nil {
		if err == pgx.ErrNoRows {
			return err
		}
		return databaseError(fmt.Errorf(" results.Query(): %w", err))
	}
	defer rows.Close()

	return nil

}

func (pql Postgresql) SaveMintRequest(tx pgx.Tx, request cashu.MintRequestDB) error {
	ctx := context.Background()

	// WARN: WrappedPubkey needs to not used it's Value function here because there are columns that are different
	// columns with string and bytea.
	var pubkeyBytes []byte
	if request.Pubkey.PublicKey != nil {
		pubkeyBytes = request.Pubkey.SerializeCompressed()
	}

	_, err := tx.Exec(ctx, "INSERT INTO mint_request (quote, request, request_paid, expiry, unit, minted, state, seen_at, amount, checking_id, pubkey, description) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)", request.Quote, request.Request, request.RequestPaid, request.Expiry, request.Unit, request.Minted, request.State, request.SeenAt, request.Amount, request.CheckingId, pubkeyBytes, request.Description)
	if err != nil {
		return databaseError(fmt.Errorf("inserting to mint_request: %w", err))

	}
	return nil
}

func (pql Postgresql) ChangeMintRequestState(tx pgx.Tx, quote string, paid bool, state cashu.ACTION_STATE, minted bool) error {
	// change the paid status of the quote
	_, err := tx.Exec(context.Background(), "UPDATE mint_request SET request_paid = $1, state = $3, minted = $4 WHERE quote = $2", paid, quote, state, minted)
	if err != nil {
		return databaseError(fmt.Errorf("inserting to mint_request: %w", err))

	}
	return nil
}

func (pql Postgresql) GetMintRequestById(tx pgx.Tx, id string) (cashu.MintRequestDB, error) {
	rows, err := tx.Query(context.Background(), "SELECT quote, request, request_paid, expiry, unit, minted, state, seen_at, amount, checking_id, pubkey, description FROM mint_request WHERE quote = $1 FOR UPDATE", id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return cashu.MintRequestDB{}, err
		}
	}
	defer rows.Close()

	var mintRequest cashu.MintRequestDB
	for rows.Next() {
		var amount *uint64
		err := rows.Scan(&mintRequest.Quote, &mintRequest.Request, &mintRequest.RequestPaid, &mintRequest.Expiry, &mintRequest.Unit, &mintRequest.Minted, &mintRequest.State, &mintRequest.SeenAt, &amount, &mintRequest.CheckingId, &mintRequest.Pubkey, &mintRequest.Description)
		if err != nil {
			return mintRequest, databaseError(fmt.Errorf("rows.Scan(&mintRequest.Quote, &mintRequest.Request, &mintRequest.RequestPaid, &mintRequest.Expiry, &mintRequest.Unit, &mintRequest.Minted, &mintRequest.State, &mintRequest.SeenAt, &amount, &mintRequest.CheckingId, pubkeyBytes, &mintRequest.Description ): %w", err))
		}

		mintRequest.Amount = amount
	}

	return mintRequest, nil
}

func (pql Postgresql) GetMintRequestByRequest(tx pgx.Tx, request string) (cashu.MintRequestDB, error) {
	rows, err := tx.Query(context.Background(), "SELECT quote, request, request_paid, expiry, unit, minted, state, seen_at, amount, checking_id, pubkey, description FROM mint_request WHERE request = $1 FOR UPDATE", request)
	if err != nil {
		if err == pgx.ErrNoRows {
			return cashu.MintRequestDB{}, err
		}
	}
	defer rows.Close()

	var mintRequest cashu.MintRequestDB
	for rows.Next() {
		var amount *uint64
		err := rows.Scan(&mintRequest.Quote, &mintRequest.Request, &mintRequest.RequestPaid, &mintRequest.Expiry, &mintRequest.Unit, &mintRequest.Minted, &mintRequest.State, &mintRequest.SeenAt, &amount, &mintRequest.CheckingId, &mintRequest.Pubkey, &mintRequest.Description)
		if err != nil {
			return mintRequest, databaseError(fmt.Errorf("row.Scan(&sig.Amount, &sig.Id, &sig.B_, &sig.C_, &sig.CreatedAt, &sig.Dleq.E, &sig.Dleq.S): %w", err))
		}

		mintRequest.Amount = amount

	}

	return mintRequest, nil
}

func (pql Postgresql) GetMeltRequestById(tx pgx.Tx, id string) (cashu.MeltRequestDB, error) {
	rows, err := tx.Query(context.Background(), "SELECT quote, request, amount, request_paid, expiry, unit, melted, fee_reserve, state, payment_preimage, seen_at, mpp, fee_paid, checking_id  FROM melt_request WHERE quote = $1 FOR UPDATE NOWAIT", id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return cashu.MeltRequestDB{}, err
		}
	}
	defer rows.Close()

	quote, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[cashu.MeltRequestDB])

	if err != nil {
		if err == pgx.ErrNoRows {
			return cashu.MeltRequestDB{}, err
		}

		return quote, databaseError(fmt.Errorf("pgx.CollectOneRow(rows, pgx.RowToStructByName[cashu.MeltRequestDB]): %w", err))
	}

	return quote, nil
}

func (pql Postgresql) GetMeltQuotesByState(state cashu.ACTION_STATE) ([]cashu.MeltRequestDB, error) {

	rows, err := pql.pool.Query(context.Background(), "SELECT quote, request, amount, request_paid, expiry, unit, melted, fee_reserve, state, payment_preimage, seen_at, mpp, fee_paid, checking_id  FROM melt_request WHERE state = $1", state)
	if err != nil {
		if err == pgx.ErrNoRows {
			return []cashu.MeltRequestDB{}, err
		}
	}
	defer rows.Close()

	quote, err := pgx.CollectRows(rows, pgx.RowToStructByName[cashu.MeltRequestDB])

	if err != nil {
		if err == pgx.ErrNoRows {
			return []cashu.MeltRequestDB{}, err
		}

		return quote, databaseError(fmt.Errorf("pgx.CollectOneRow(rows, pgx.RowToStructByName[cashu.MeltRequestDB]): %w", err))
	}

	return quote, nil
}

func (pql Postgresql) SaveMeltRequest(tx pgx.Tx, request cashu.MeltRequestDB) error {

	_, err := tx.Exec(context.Background(),
		"INSERT INTO melt_request (quote, request, fee_reserve, expiry, unit, amount, request_paid, melted, state, payment_preimage, seen_at, mpp, fee_paid, checking_id) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)",
		request.Quote, request.Request, request.FeeReserve, request.Expiry, request.Unit, request.Amount, request.RequestPaid, request.Melted, request.State, request.PaymentPreimage, request.SeenAt, request.Mpp, request.FeePaid, request.CheckingId)
	if err != nil {
		return databaseError(fmt.Errorf("inserting to mint_request: %w", err))
	}
	return nil
}

func (pql Postgresql) AddPreimageMeltRequest(tx pgx.Tx, quote string, preimage string) error {
	// change the paid status of the quote
	_, err := tx.Exec(context.Background(), "UPDATE melt_request SET payment_preimage = $1 WHERE quote = $2", preimage, quote)
	if err != nil {
		return databaseError(fmt.Errorf("updating melt_request with preimage: %w", err))

	}
	return nil
}
func (pql Postgresql) ChangeMeltRequestState(tx pgx.Tx, quote string, paid bool, state cashu.ACTION_STATE, melted bool, fee_paid uint64) error {
	// change the paid status of the quote
	_, err := tx.Exec(context.Background(), "UPDATE melt_request SET request_paid = $1, state = $3, melted = $4, fee_paid = $5 WHERE quote = $2", paid, quote, state, melted, fee_paid)
	if err != nil {
		return databaseError(fmt.Errorf("updating mint_request: %w", err))

	}
	return nil
}
func (pql Postgresql) ChangeCheckingId(tx pgx.Tx, quote string, checking_id string) error {
	// change the paid status of the quote
	_, err := tx.Exec(context.Background(), "UPDATE melt_request SET checking_id = $1 WHERE quote = $2", checking_id, quote)
	if err != nil {
		return databaseError(fmt.Errorf("updating mint_request: %w", err))

	}
	return nil
}

func (pql Postgresql) GetProofsFromSecret(tx pgx.Tx, SecretList []string) (cashu.Proofs, error) {

	var proofList cashu.Proofs

	ctx := context.Background()
	rows, err := tx.Query(ctx, "SELECT amount, id, secret, c, y, witness, seen_at, state, quote FROM proofs WHERE secret = ANY($1) FOR UPDATE NOWAIT", SecretList)

	if err != nil {
		if err == pgx.ErrNoRows {
			return proofList, nil
		}
		return proofList, databaseError(fmt.Errorf("query error: %w", err))
	}
	defer rows.Close()

	proof, err := pgx.CollectRows(rows, pgx.RowToStructByName[cashu.Proof])

	if err != nil {
		if err == pgx.ErrNoRows {
			return proofList, nil
		}
		return proofList, databaseError(fmt.Errorf("pgx.CollectRows(rows, pgx.RowToStructByName[cashu.Proof]): %w", err))
	}

	proofList = proof

	return proofList, nil
}

func (pql Postgresql) SaveProof(tx pgx.Tx, proofs []cashu.Proof) error {
	entries := [][]any{}
	columns := []string{"c", "secret", "amount", "id", "y", "witness", "seen_at", "state", "quote"}
	tableName := "proofs"

	for _, proof := range proofs {
		C := proof.C.String()
		entries = append(entries, []any{C, proof.Secret, proof.Amount, proof.Id, proof.Y, proof.Witness, proof.SeenAt, proof.State, proof.Quote})
	}

	_, err := tx.CopyFrom(context.Background(), pgx.Identifier{tableName}, columns, pgx.CopyFromRows(entries))

	if err != nil {

		return databaseError(fmt.Errorf("inserting to DB: %w", err))
	}
	return nil
}

func (pql Postgresql) GetProofsFromSecretCurve(tx pgx.Tx, Ys []cashu.WrappedPublicKey) (cashu.Proofs, error) {

	var proofList cashu.Proofs

	rows, err := tx.Query(context.Background(), `SELECT amount, id, secret, c, y, witness, seen_at, state, quote FROM proofs WHERE y = ANY($1) FOR UPDATE NOWAIT`, Ys)

	if err != nil {

		if err == pgx.ErrNoRows {
			return proofList, nil
		}
	}
	defer rows.Close()

	proof, err := pgx.CollectRows(rows, pgx.RowToStructByName[cashu.Proof])

	if err != nil {
		if err == pgx.ErrNoRows {
			return proofList, nil
		}
		return proofList, fmt.Errorf("pgx.CollectRows(rows, pgx.RowToStructByName[cashu.Proof]): %w", err)
	}

	proofList = proof

	return proofList, nil
}

func (pql Postgresql) GetProofsFromQuote(tx pgx.Tx, quote string) (cashu.Proofs, error) {

	var proofList cashu.Proofs

	rows, err := tx.Query(context.Background(), `SELECT amount, id, secret, c, y, witness, seen_at, state, quote FROM proofs WHERE quote = $1 FOR UPDATE NOWAIT`, quote)
	if err != nil {
		if err == pgx.ErrNoRows {
			return proofList, nil
		}
		return proofList, fmt.Errorf("query error: %w", err)
	}
	defer rows.Close()

	proof, err := pgx.CollectRows(rows, pgx.RowToStructByName[cashu.Proof])
	if err != nil {
		if err == pgx.ErrNoRows {
			return proofList, nil
		}
		return proofList, fmt.Errorf("pgx.CollectRows(rows, pgx.RowToStructByName[cashu.Proof]): %w", err)
	}

	proofList = proof

	return proofList, nil
}
func (pql Postgresql) SetProofsState(tx pgx.Tx, proofs cashu.Proofs, state cashu.ProofState) error {
	// change the paid status of the quote
	batch := pgx.Batch{}
	for _, proof := range proofs {
		batch.Queue(`UPDATE proofs SET state = $1  WHERE y = $2`, state, proof.Y)
	}

	results := tx.SendBatch(context.Background(), &batch)
	defer func() {
		if err := results.Close(); err != nil {
			slog.Error("failed to close results", slog.Any("error", err))
		}
	}()

	rows, err := results.Query()
	if err != nil {
		if err == pgx.ErrNoRows {
			return err
		}
		return databaseError(fmt.Errorf(" results.Query(): %w", err))
	}
	defer rows.Close()

	return nil
}

func (pql Postgresql) DeleteProofs(tx pgx.Tx, proofs cashu.Proofs) error {
	// change the paid status of the quote
	batch := pgx.Batch{}
	for _, proof := range proofs {
		batch.Queue(`DELETE FROM proofs WHERE y = $1`, proof.Y)
	}

	results := tx.SendBatch(context.Background(), &batch)
	defer func() {
		if err := results.Close(); err != nil {
			slog.Error("failed to close results", slog.Any("error", err))
		}
	}()

	rows, err := results.Query()
	if err != nil {
		if err == pgx.ErrNoRows {
			return err
		}
		return databaseError(fmt.Errorf(" results.Query(): %w", err))
	}
	defer rows.Close()

	return nil

}

func privateKeysToDleq(s_key *string, e_key *string, sig *cashu.RecoverSigDB) error {
	if s_key == nil || e_key == nil {
		return nil
	}
	if *s_key == "" || *e_key == "" {
		return nil
	}

	sBytes, err := hex.DecodeString(*s_key)
	if err != nil {
		return errors.New("failed to decode 's' field")
	}
	dleqTmp := &cashu.BlindSignatureDLEQ{
		S: nil,
		E: nil,
	}

	dleqTmp.S = secp256k1.PrivKeyFromBytes(sBytes)

	eBytes, err := hex.DecodeString(*e_key)
	if err != nil {
		return errors.New("failed to decode '' field")
	}
	dleqTmp.E = secp256k1.PrivKeyFromBytes(eBytes)

	sig.Dleq = dleqTmp
	return nil
}

func (pql Postgresql) GetRestoreSigsFromBlindedMessages(tx pgx.Tx, B_ []string) ([]cashu.RecoverSigDB, error) {

	var signaturesList []cashu.RecoverSigDB

	rows, err := tx.Query(context.Background(), `SELECT id, amount, "C_", "B_", created_at, dleq_e, dleq_s FROM recovery_signature WHERE "B_" = ANY($1)`, B_)
	if err != nil {
		if err == pgx.ErrNoRows {
			return signaturesList, nil
		}
		return signaturesList, databaseError(fmt.Errorf("error checking for recovery_signature: %w", err))
	}
	defer rows.Close()

	signatures := make([]cashu.RecoverSigDB, 0)
	for rows.Next() {
		var sig cashu.RecoverSigDB
		sig.Dleq = nil

		var dleq_s_str *string
		var dleq_e_str *string
		err := rows.Scan(&sig.Id, &sig.Amount, &sig.C_, &sig.B_, &sig.CreatedAt, &dleq_e_str, &dleq_s_str)
		if err != nil {
			return nil, databaseError(fmt.Errorf("row.Scan(&sig.Amount, &sig.Id, &sig.B_, &sig.C_, &sig.CreatedAt, &sig.Dleq.E, &sig.Dleq.S): %w", err))
		}

		err = privateKeysToDleq(dleq_s_str, dleq_e_str, &sig)
		if err != nil {
			return nil, databaseError(fmt.Errorf("privateKeysToDleq(dleq_s_str, dleq_e_str, sig.Dleq). %w", err))
		}

		signatures = append(signatures, sig)
	}

	signaturesList = signatures

	return signaturesList, nil
}

func (pql Postgresql) SaveRestoreSigs(tx pgx.Tx, recover_sigs []cashu.RecoverSigDB) error {
	entries := [][]any{}
	columns := []string{"id", "amount", "B_", "C_", "created_at", "dleq_e", "dleq_s"}
	tableName := "recovery_signature"
	tries := 0

	for _, sig := range recover_sigs {
		dleq_e_bytes := sig.Dleq.E.Key.Bytes()
		dleq_s_bytes := sig.Dleq.S.Key.Bytes()
		entries = append(entries, []any{sig.Id, sig.Amount, sig.B_, sig.C_, sig.CreatedAt, hex.EncodeToString(dleq_e_bytes[:]), hex.EncodeToString(dleq_s_bytes[:])})
	}

	for {
		tries += 1
		_, err := tx.CopyFrom(context.Background(), pgx.Identifier{tableName}, columns, pgx.CopyFromRows(entries))

		switch {
		case err != nil && tries < 3:
			continue
		case err != nil && tries >= 3:
			return databaseError(fmt.Errorf("inserting to DB: %w", err))
		case err == nil:
			return nil
		}

	}
}

func (pql Postgresql) GetProofsTimeSeries(since int64, bucketMinutes int) ([]database.ProofTimeSeriesPoint, error) {
	var points []database.ProofTimeSeriesPoint

	bucketSeconds := int64(bucketMinutes * 60)

	var query string
	var args []any

	// Use floor division to group proofs into time buckets
	// (seen_at / bucket_seconds) * bucket_seconds gives us the bucket start timestamp

	// Use current time as upper bound
	now := time.Now().Unix()
	query = `SELECT 
				(seen_at / $3) * $3 as bucket_timestamp,
				COALESCE(SUM(amount), 0) as total_amount,
				COUNT(*) as count
			 FROM proofs 
			 WHERE seen_at >= $1 AND seen_at < $2
			 GROUP BY bucket_timestamp
			 ORDER BY bucket_timestamp ASC`
	args = []any{since, now, bucketSeconds}

	rows, err := pql.pool.Query(context.Background(), query, args...)
	if err != nil {
		if err == pgx.ErrNoRows {
			return points, nil
		}
		return points, databaseError(fmt.Errorf("GetProofsTimeSeries query error: %w", err))
	}
	defer rows.Close()

	for rows.Next() {
		var point database.ProofTimeSeriesPoint
		err := rows.Scan(&point.Timestamp, &point.TotalAmount, &point.Count)
		if err != nil {
			return points, databaseError(fmt.Errorf("GetProofsTimeSeries scan error: %w", err))
		}
		points = append(points, point)
	}

	return points, nil
}

func (pql Postgresql) GetBlindSigsTimeSeries(since int64, bucketMinutes int) ([]database.ProofTimeSeriesPoint, error) {
	var points []database.ProofTimeSeriesPoint

	bucketSeconds := int64(bucketMinutes * 60)

	var query string
	var args []any

	// Use floor division to group blind sigs into time buckets
	// (created_at / bucket_seconds) * bucket_seconds gives us the bucket start timestamp

	// Use current time as upper bound
	now := time.Now().Unix()
	query = `SELECT 
				(created_at / $3) * $3 as bucket_timestamp,
				COALESCE(SUM(amount), 0) as total_amount,
				COUNT(*) as count
			 FROM recovery_signature 
			 WHERE created_at >= $1 AND created_at < $2
			 GROUP BY bucket_timestamp
			 ORDER BY bucket_timestamp ASC`
	args = []any{since, now, bucketSeconds}

	rows, err := pql.pool.Query(context.Background(), query, args...)
	if err != nil {
		if err == pgx.ErrNoRows {
			return points, nil
		}
		return points, databaseError(fmt.Errorf("GetBlindSigsTimeSeries query error: %w", err))
	}
	defer rows.Close()

	for rows.Next() {
		var point database.ProofTimeSeriesPoint
		err := rows.Scan(&point.Timestamp, &point.TotalAmount, &point.Count)
		if err != nil {
			return points, databaseError(fmt.Errorf("GetBlindSigsTimeSeries scan error: %w", err))
		}
		points = append(points, point)
	}

	return points, nil
}

func (pql Postgresql) GetProofsCountByKeyset(since time.Time) (map[string]database.ProofsCountByKeyset, error) {
	results := make(map[string]database.ProofsCountByKeyset)

	var query string
	var args []any

	// Only since is provided
	query = `SELECT id, COALESCE(SUM(amount), 0), COUNT(*) 
			 FROM proofs 
			 WHERE seen_at >= $1
			 GROUP BY id`
	args = []any{since.Unix()}

	rows, err := pql.pool.Query(context.Background(), query, args...)
	if err != nil {
		if err == pgx.ErrNoRows {
			return results, nil
		}
		return results, databaseError(fmt.Errorf("GetProofsCountByKeyset query error: %w", err))
	}
	defer rows.Close()

	for rows.Next() {
		var item database.ProofsCountByKeyset
		err := rows.Scan(&item.KeysetId, &item.TotalAmount, &item.Count)
		if err != nil {
			return results, databaseError(fmt.Errorf("GetProofsCountByKeyset scan error: %w", err))
		}
		results[item.KeysetId] = item
	}

	return results, nil
}

func (pql Postgresql) GetBlindSigsCountByKeyset(since time.Time) (map[string]database.BlindSigsCountByKeyset, error) {
	results := make(map[string]database.BlindSigsCountByKeyset)

	var query string
	var args []any

	// Only since is provided
	query = `SELECT id, COALESCE(SUM(amount), 0), COUNT(*) 
			 FROM recovery_signature 
			 WHERE created_at >= $1
			 GROUP BY id`
	args = []any{since.Unix()}

	rows, err := pql.pool.Query(context.Background(), query, args...)
	if err != nil {
		if err == pgx.ErrNoRows {
			return results, nil
		}
		return results, databaseError(fmt.Errorf("GetBlindSigsCountByKeyset query error: %w", err))
	}
	defer rows.Close()

	for rows.Next() {
		var item database.BlindSigsCountByKeyset
		err := rows.Scan(&item.KeysetId, &item.TotalAmount, &item.Count)
		if err != nil {
			return results, databaseError(fmt.Errorf("GetBlindSigsCountByKeyset scan error: %w", err))
		}
		results[item.KeysetId] = item
	}

	return results, nil
}

func (pql Postgresql) Close() {
	pql.pool.Close()
}
