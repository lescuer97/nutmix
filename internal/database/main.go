package database

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/pressly/goose/v3"
	"log"
	"os"
)

var DBError = errors.New("ERROR DATABASE")

var DATABASE_URL_ENV = "DATABASE_URL"

const (
	DOCKERDATABASE = "DOCKERDATABASE"
	CUSTOMDATABASE = "CUSTOMDATABASE"
)

func databaseError(err error) error {
	return fmt.Errorf("%w  %w", DBError, err)
}

func DatabaseSetup(ctx context.Context, migrationDir string) (*pgxpool.Pool, error) {

	dbUrl := os.Getenv(DATABASE_URL_ENV)
	if dbUrl == "" {
		return &pgxpool.Pool{}, fmt.Errorf("%v enviroment variable empty", DATABASE_URL_ENV)

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
		return nil, databaseError(fmt.Errorf("Error connecting to database: %w", err))
	}

	return pool, nil
}

func GetAllSeeds(pool *pgxpool.Pool) ([]cashu.Seed, error) {
	var seeds []cashu.Seed

	rows, err := pool.Query(context.Background(), `SELECT  created_at, active, version, unit, id,  "input_fee_ppk" FROM seeds ORDER BY version DESC`)
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

func GetActiveSeed(pool *pgxpool.Pool) (cashu.Seed, error) {
	rows, err := pool.Query(context.Background(), "SELECT seed, created_at, active, version, unit, id, encrypted, input_fee_ppk FROM seeds WHERE active")
	defer rows.Close()
	if err != nil {
		return cashu.Seed{}, fmt.Errorf("Error checking for Active seeds: %w", err)
	}

	seed, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[cashu.Seed])

	if err != nil {
		return seed, databaseError(fmt.Errorf("pgx.CollectOneRow(rows, pgx.RowToStructByName[cashu.Seed]): %w", err))
	}

	return seed, nil
}

func GetSeedsByUnit(pool *pgxpool.Pool, unit cashu.Unit) ([]cashu.Seed, error) {
	rows, err := pool.Query(context.Background(), "SELECT  created_at, active, version, unit, id, input_fee_ppk FROM seeds WHERE unit = $1", unit.String())
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

func SaveNewSeed(pool *pgxpool.Pool, seed *cashu.Seed) error {

	tries := 0

	for {
		tries += 1
		_, err := pool.Exec(context.Background(), "INSERT INTO seeds ( active, created_at, unit, id, version, input_fee_ppk) VALUES ($1, $2, $3, $4, $5, $6)", seed.Active, seed.CreatedAt, seed.Unit, seed.Id, seed.Version, seed.InputFeePpk)

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

func SaveNewSeeds(pool *pgxpool.Pool, seeds []cashu.Seed) error {
	tries := 0

	entries := [][]any{}
	columns := []string{"active", "created_at", "unit", "id", "version", "input_fee_ppk"}
	tableName := "seeds"

	for _, seed := range seeds {
		entries = append(entries, []any{seed.Active, seed.CreatedAt, seed.Unit, seed.Id, seed.Version, seed.InputFeePpk})
	}

	for {
		tries += 1
		_, err := pool.CopyFrom(context.Background(), pgx.Identifier{tableName}, columns, pgx.CopyFromRows(entries))

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

func UpdateActiveStatusSeeds(pool *pgxpool.Pool, seeds []cashu.Seed) error {
	// change the paid status of the quote
	batch := pgx.Batch{}
	for _, seed := range seeds {

		batch.Queue("UPDATE seeds SET active = $1 WHERE id = $2", seed.Active, seed.Id)

	}
	results := pool.SendBatch(context.Background(), &batch)
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

func SaveMintRequestDB(pool *pgxpool.Pool, request cashu.MintRequestDB) error {
	ctx := context.Background()

	_, err := pool.Exec(ctx, "INSERT INTO mint_request (quote, request, request_paid, expiry, unit, minted, state, seen_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)", request.Quote, request.Request, request.RequestPaid, request.Expiry, request.Unit, request.Minted, request.State, request.SeenAt)
	if err != nil {
		return databaseError(fmt.Errorf("Inserting to mint_request: %w", err))

	}
	return nil
}
func ModifyQuoteMintPayStatus(pool *pgxpool.Pool, requestPaid bool, state cashu.ACTION_STATE, quote string) error {
	// change the paid status of the quote
	_, err := pool.Exec(context.Background(), "UPDATE mint_request SET request_paid = $1, state = $3 WHERE quote = $2", requestPaid, quote, state)
	if err != nil {
		return databaseError(fmt.Errorf("Inserting to mint_request: %w", err))

	}
	return nil
}

func ModifyQuoteMintMintedStatus(pool *pgxpool.Pool, minted bool, state cashu.ACTION_STATE, quote string) error {

	args := pgx.NamedArgs{
		"state":  state,
		"minted": minted,
		"quote":  quote,
	}

	query := `UPDATE mint_request SET minted = @minted, state = @state WHERE quote = @quote`

	// change the paid status of the quote
	_, err := pool.Exec(context.Background(), query, args)

	if err != nil {
		return databaseError(fmt.Errorf("Inserting to mint_request: %w", err))

	}
	return nil
}
func SaveQuoteMeltRequest(pool *pgxpool.Pool, request cashu.MeltRequestDB) error {

	_, err := pool.Exec(context.Background(), "INSERT INTO melt_request (quote, request, fee_reserve, expiry, unit, amount, request_paid, melted, state, payment_preimage, seen_at, mpp) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)", request.Quote, request.Request, request.FeeReserve, request.Expiry, request.Unit, request.Amount, request.RequestPaid, request.Melted, request.State, request.PaymentPreimage, request.SeenAt, request.Mpp)
	if err != nil {
		return databaseError(fmt.Errorf("Inserting to mint_request: %w", err))
	}
	return nil
}

func AddPaymentPreimageToMeltRequest(pool *pgxpool.Pool, preimage string, quote string) error {
	// change the paid status of the quote
	_, err := pool.Exec(context.Background(), "UPDATE melt_request SET payment_preimage = $1 WHERE quote = $2", preimage, quote)
	if err != nil {
		return databaseError(fmt.Errorf("updating melt_request with preimage: %w", err))

	}
	return nil
}
func ModifyQuoteMeltPayStatus(pool *pgxpool.Pool, paid bool, state cashu.ACTION_STATE, request string) error {
	// change the paid status of the quote
	_, err := pool.Exec(context.Background(), "UPDATE melt_request SET request_paid = $1, state = $3 WHERE quote = $2", paid, request, state)
	if err != nil {
		return databaseError(fmt.Errorf("updating mint_request: %w", err))

	}
	return nil
}
func ModifyQuoteMeltPayStatusAndMelted(pool *pgxpool.Pool, paid bool, melted bool, state cashu.ACTION_STATE, request string) error {
	// change the paid status of the quote
	_, err := pool.Exec(context.Background(), "UPDATE melt_request SET request_paid = $1, melted = $3, state = $4 WHERE quote = $2", paid, request, melted, state)
	if err != nil {
		return databaseError(fmt.Errorf("updating mint_request: %w", err))

	}
	return nil
}

func ModifyQuoteMeltMeltedStatus(pool *pgxpool.Pool, melted bool, quote string) error {
	// change the paid status of the quote
	_, err := pool.Exec(context.Background(), "UPDATE melt_request SET melted = $1 WHERE quote = $2", melted, quote)
	if err != nil {
		return databaseError(fmt.Errorf("updating mint_request: %w", err))

	}
	return nil
}

func GetMintQuoteById(pool *pgxpool.Pool, id string) (cashu.MintRequestDB, error) {

	rows, err := pool.Query(context.Background(), "SELECT quote, request, request_paid, expiry, unit, minted, state, seen_at FROM mint_request WHERE quote = $1", id)
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
func GetMeltQuoteById(pool *pgxpool.Pool, id string) (cashu.MeltRequestDB, error) {

	rows, err := pool.Query(context.Background(), "SELECT quote, request, amount, request_paid, expiry, unit, melted, fee_reserve, state, payment_preimage, seen_at, mpp  FROM melt_request WHERE quote = $1", id)
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

func CheckListOfProofs(pool *pgxpool.Pool, CList []string, SecretList []string) ([]cashu.Proof, error) {

	var proofList []cashu.Proof

	ctx := context.Background()
	rows, err := pool.Query(ctx, "SELECT amount, id, secret, c, y, witness, seen_at  FROM proofs WHERE C = ANY($1) OR secret = ANY($2)", CList, SecretList)

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

func SaveProofs(pool *pgxpool.Pool, proofs []cashu.Proof) error {
	entries := [][]any{}
	columns := []string{"c", "secret", "amount", "id", "y", "witness", "seen_at"}
	tableName := "proofs"

	tries := 0

	for _, proof := range proofs {
		entries = append(entries, []any{proof.C, proof.Secret, proof.Amount, proof.Id, proof.Y, proof.Witness, proof.SeenAt})
	}

	for {
		tries += 1
		_, err := pool.CopyFrom(context.Background(), pgx.Identifier{tableName}, columns, pgx.CopyFromRows(entries))

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

func CheckListOfProofsBySecretCurve(pool *pgxpool.Pool, Ys []string) ([]cashu.Proof, error) {

	var proofList []cashu.Proof

	rows, err := pool.Query(context.Background(), `SELECT amount, id, secret, c, y, witness, seen_at FROM proofs WHERE y = ANY($1)`, Ys)
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

func GetRestoreSigsFromBlindedMessages(pool *pgxpool.Pool, B_ []string) ([]cashu.RecoverSigDB, error) {

	var signaturesList []cashu.RecoverSigDB

	rows, err := pool.Query(context.Background(), `SELECT id, amount, "C_", "B_", created_at  FROM recovery_signature WHERE "B_" = ANY($1)`, B_)
	defer rows.Close()

	if err != nil {
		if err == pgx.ErrNoRows {
			return signaturesList, nil
		}
		return signaturesList, databaseError(fmt.Errorf("Error checking for  recovery_signature: %w", err))
	}

	signatures, err := pgx.CollectRows(rows, pgx.RowToStructByName[cashu.RecoverSigDB])

	if err != nil {
		if err == pgx.ErrNoRows {
			return signaturesList, nil
		}
		return signaturesList, databaseError(fmt.Errorf("pgx.CollectRows(rows, pgx.RowToStructByName[cashu.RecoverSigDB]): %w", err))
	}

	signaturesList = signatures

	return signaturesList, nil
}

func SetRestoreSigs(pool *pgxpool.Pool, recover_sigs []cashu.RecoverSigDB) error {
	entries := [][]any{}
	columns := []string{"id", "amount", "B_", "C_", "created_at"}
	tableName := "recovery_signature"
	tries := 0

	for _, sig := range recover_sigs {
		entries = append(entries, []any{sig.Id, sig.Amount, sig.B_, sig.C_, sig.CreatedAt})
	}

	for {
		tries += 1
		_, err := pool.CopyFrom(context.Background(), pgx.Identifier{tableName}, columns, pgx.CopyFromRows(entries))

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
