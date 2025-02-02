package postgresql

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/pressly/goose/v3"
)

var DBError = errors.New("ERROR DATABASE")

var DATABASE_URL_ENV = "DATABASE_URL"

type Postgresql struct {
	pool *pgxpool.Pool
}

func databaseError(err error) error {
	return fmt.Errorf("%w  %w", DBError, err)
}

func DatabaseSetup(ctx context.Context, migrationDir string) (Postgresql, error) {

	var postgresql Postgresql

	dbUrl := os.Getenv(DATABASE_URL_ENV)
	if dbUrl == "" {
		return postgresql, fmt.Errorf("%v enviroment variable empty", DATABASE_URL_ENV)

	}

	pool, err := pgxpool.New(context.Background(), dbUrl)

	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatalf("Error setting dialect: %v", err)
	}

	db := stdlib.OpenDBFromPool(pool)

	if err := goose.Up(db, migrationDir); err != nil {
		log.Fatalf("Error running migrations: %v", err)
	}

	if err := db.Close(); err != nil {
		panic(err)
	}

	if err != nil {
		return postgresql, databaseError(fmt.Errorf("Error connecting to database: %w", err))
	}
	postgresql.pool = pool

	return postgresql, nil
}

func (pql Postgresql) GetAllSeeds() ([]cashu.Seed, error) {
	var seeds []cashu.Seed

	rows, err := pql.pool.Query(context.Background(), `SELECT  created_at, active, version, unit, id,  "input_fee_ppk" FROM seeds ORDER BY version DESC`)
	defer rows.Close()

	if err != nil {
		if err == pgx.ErrNoRows {
			return seeds, fmt.Errorf("No rows found: %w", err)
		}

		return seeds, fmt.Errorf("Error checking for  seeds: %w", err)
	}

	seeds_collect, err := pgx.CollectRows(rows, pgx.RowToStructByName[cashu.Seed])

	if err != nil {
		return seeds_collect, fmt.Errorf("Collecting rows: %w", err)
	}

	return seeds_collect, nil
}

func (pql Postgresql) GetSeedsByUnit(unit cashu.Unit) ([]cashu.Seed, error) {
	rows, err := pql.pool.Query(context.Background(), "SELECT  created_at, active, version, unit, id, input_fee_ppk FROM seeds WHERE unit = $1", unit.String())
	defer rows.Close()
	if err != nil {
		return []cashu.Seed{}, fmt.Errorf("Error checking for Active seeds: %w", err)
	}

	seeds, err := pgx.CollectRows(rows, pgx.RowToStructByName[cashu.Seed])

	if err != nil {
		return seeds, databaseError(fmt.Errorf("pgx.CollectRows(rows, pgx.RowToStructByName[cashu.Seed]): %w", err))
	}

	return seeds, nil
}

func (pql Postgresql) SaveNewSeed(seed cashu.Seed) error {

	tries := 0

	for {
		tries += 1
		_, err := pql.pool.Exec(context.Background(), "INSERT INTO seeds ( active, created_at, unit, id, version, input_fee_ppk) VALUES ($1, $2, $3, $4, $5, $6)", seed.Active, seed.CreatedAt, seed.Unit, seed.Id, seed.Version, seed.InputFeePpk)

		switch {
		case err != nil && tries < 3:
			continue
		case err != nil && tries >= 3:
			return databaseError(fmt.Errorf("Inserting to seeds: %w", err))
		case err == nil:
			return nil
		}

	}
}

func (pql Postgresql) SaveNewSeeds(seeds []cashu.Seed) error {
	tries := 0

	entries := [][]any{}
	columns := []string{"active", "created_at", "unit", "id", "version", "input_fee_ppk"}
	tableName := "seeds"

	for _, seed := range seeds {
		entries = append(entries, []any{seed.Active, seed.CreatedAt, seed.Unit, seed.Id, seed.Version, seed.InputFeePpk})
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

func (pql Postgresql) UpdateSeedsActiveStatus(seeds []cashu.Seed) error {
	// change the paid status of the quote
	batch := pgx.Batch{}
	for _, seed := range seeds {

		batch.Queue("UPDATE seeds SET active = $1 WHERE id = $2", seed.Active, seed.Id)

	}
	results := pql.pool.SendBatch(context.Background(), &batch)
	defer results.Close()

	rows, err := results.Query()
	defer rows.Close()
	if err != nil {
		if err == pgx.ErrNoRows {
			return err
		}
		return databaseError(fmt.Errorf(" results.Query(): %w", err))
	}

	return nil

}

func (pql Postgresql) SaveMintRequest(request cashu.MintRequestDB) error {
	ctx := context.Background()

	_, err := pql.pool.Exec(ctx, "INSERT INTO mint_request (quote, request, request_paid, expiry, unit, minted, state, seen_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)", request.Quote, request.Request, request.RequestPaid, request.Expiry, request.Unit, request.Minted, request.State, request.SeenAt)
	if err != nil {
		return databaseError(fmt.Errorf("Inserting to mint_request: %w", err))

	}
	return nil
}

func (pql Postgresql) ChangeMintRequestState(quote string, paid bool, state cashu.ACTION_STATE, minted bool) error {
	// change the paid status of the quote
	_, err := pql.pool.Exec(context.Background(), "UPDATE mint_request SET request_paid = $1, state = $3, minted = $4 WHERE quote = $2", paid, quote, state, minted)
	if err != nil {
		return databaseError(fmt.Errorf("Inserting to mint_request: %w", err))

	}
	return nil
}

func (pql Postgresql) GetMintRequestById(id string) (cashu.MintRequestDB, error) {

	rows, err := pql.pool.Query(context.Background(), "SELECT quote, request, request_paid, expiry, unit, minted, state, seen_at FROM mint_request WHERE quote = $1", id)
	defer rows.Close()
	if err != nil {
		if err == pgx.ErrNoRows {
			return cashu.MintRequestDB{}, err
		}
	}

	quote, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[cashu.MintRequestDB])
	rows.Close()
	if err != nil {
		if err == pgx.ErrNoRows {
			return cashu.MintRequestDB{}, err
		}
		return quote, databaseError(fmt.Errorf("pgx.CollectOneRow(rows, pgx.RowToStructByName[cashu.PostMintQuoteBolt11Response]): %w", err))
	}

	return quote, nil
}

func (pql Postgresql) GetMeltRequestById(id string) (cashu.MeltRequestDB, error) {

	rows, err := pql.pool.Query(context.Background(), "SELECT quote, request, amount, request_paid, expiry, unit, melted, fee_reserve, state, payment_preimage, seen_at, mpp  FROM melt_request WHERE quote = $1", id)
	defer rows.Close()
	if err != nil {
		if err == pgx.ErrNoRows {
			return cashu.MeltRequestDB{}, err
		}
	}

	quote, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[cashu.MeltRequestDB])

	if err != nil {
		if err == pgx.ErrNoRows {
			return cashu.MeltRequestDB{}, err
		}

		return quote, databaseError(fmt.Errorf("pgx.CollectOneRow(rows, pgx.RowToStructByName[cashu.MeltRequestDB]): %w", err))
	}

	return quote, nil
}

func (pql Postgresql) SaveMeltRequest(request cashu.MeltRequestDB) error {

	_, err := pql.pool.Exec(context.Background(), "INSERT INTO melt_request (quote, request, fee_reserve, expiry, unit, amount, request_paid, melted, state, payment_preimage, seen_at, mpp) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)", request.Quote, request.Request, request.FeeReserve, request.Expiry, request.Unit, request.Amount, request.RequestPaid, request.Melted, request.State, request.PaymentPreimage, request.SeenAt, request.Mpp)
	if err != nil {
		return databaseError(fmt.Errorf("Inserting to mint_request: %w", err))
	}
	return nil
}

func (pql Postgresql) AddPreimageMeltRequest(quote string, preimage string) error {
	// change the paid status of the quote
	_, err := pql.pool.Exec(context.Background(), "UPDATE melt_request SET payment_preimage = $1 WHERE quote = $2", preimage, quote)
	if err != nil {
		return databaseError(fmt.Errorf("updating melt_request with preimage: %w", err))

	}
	return nil
}
func (pql Postgresql) ChangeMeltRequestState(quote string, paid bool, state cashu.ACTION_STATE, melted bool) error {
	// change the paid status of the quote
	_, err := pql.pool.Exec(context.Background(), "UPDATE melt_request SET request_paid = $1, state = $3, melted = $4 WHERE quote = $2", paid, quote, state, melted)
	if err != nil {
		return databaseError(fmt.Errorf("updating mint_request: %w", err))

	}
	return nil
}

func (pql Postgresql) GetProofsFromSecret(SecretList []string) ([]cashu.Proof, error) {

	var proofList []cashu.Proof

	ctx := context.Background()
	rows, err := pql.pool.Query(ctx, "SELECT amount, id, secret, c, y, witness, seen_at, state, quote FROM proofs WHERE secret = ANY($1)", SecretList)

	defer rows.Close()

	if err != nil {
		if err == pgx.ErrNoRows {
			return proofList, nil
		}
	}

	proof, err := pgx.CollectRows(rows, pgx.RowToStructByName[cashu.Proof])
	rows.Close()

	if err != nil {
		if err == pgx.ErrNoRows {
			return proofList, nil
		}
		return proofList, databaseError(fmt.Errorf("pgx.CollectRows(rows, pgx.RowToStructByName[cashu.Proof]): %w", err))
	}

	proofList = proof

	return proofList, nil
}

func (pql Postgresql) SaveProof(proofs []cashu.Proof) error {
	entries := [][]any{}
	columns := []string{"c", "secret", "amount", "id", "y", "witness", "seen_at", "state", "quote"}
	tableName := "proofs"

	tries := 0

	for _, proof := range proofs {
		entries = append(entries, []any{proof.C, proof.Secret, proof.Amount, proof.Id, proof.Y, proof.Witness, proof.SeenAt, proof.State, proof.Quote})
	}

	for {
		tries += 1
		_, err := pql.pool.CopyFrom(context.Background(), pgx.Identifier{tableName}, columns, pgx.CopyFromRows(entries))

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

func (pql Postgresql) GetProofsFromSecretCurve(Ys []string) ([]cashu.Proof, error) {

	var proofList []cashu.Proof

	rows, err := pql.pool.Query(context.Background(), `SELECT amount, id, secret, c, y, witness, seen_at, state, quote FROM proofs WHERE y = ANY($1)`, Ys)
	defer rows.Close()

	if err != nil {

		if err == pgx.ErrNoRows {
			return proofList, nil
		}
	}

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

func (pql Postgresql) GetRestoreSigsFromBlindedMessages(B_ []string) ([]cashu.RecoverSigDB, error) {

	var signaturesList []cashu.RecoverSigDB

	rows, err := pql.pool.Query(context.Background(), `SELECT id, amount, "C_", "B_", created_at, dleq_e, dleq_s FROM recovery_signature WHERE "B_" = ANY($1)`, B_)
	defer rows.Close()

	if err != nil {
		if err == pgx.ErrNoRows {
			return signaturesList, nil
		}
		return signaturesList, databaseError(fmt.Errorf("Error checking for  recovery_signature: %w", err))
	}

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

func (pql Postgresql) SaveRestoreSigs(recover_sigs []cashu.RecoverSigDB) error {
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
		_, err := pql.pool.CopyFrom(context.Background(), pgx.Identifier{tableName}, columns, pgx.CopyFromRows(entries))

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

func (pql Postgresql) Close() {
	pql.pool.Close()
}
