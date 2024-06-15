package main

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/pressly/goose/v3"
	"log"
	"os"
)

func DatabaseSetup(migrationDir string) (*pgxpool.Pool, error) {
	databaseConUrl := os.Getenv("DATABASE_URL")

	pool, err := pgxpool.New(context.Background(), databaseConUrl)

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
		return nil, fmt.Errorf("Error connecting to database: %w", err)
	}

	return pool, nil
}

func GetAllSeeds(pool *pgxpool.Pool) ([]cashu.Seed, error) {
	var seeds []cashu.Seed

	rows, err := pool.Query(context.Background(), "SELECT * FROM seeds")

	if err != nil {
		if err == pgx.ErrNoRows {
			return seeds, fmt.Errorf("No rows found: %w", err)
		}

		return seeds, fmt.Errorf("Error checking for  seeds: %w", err)
	}

	defer rows.Close()

	seeds_collect, err := pgx.CollectRows(rows, pgx.RowToStructByName[cashu.Seed])

	if err != nil {
		return seeds_collect, fmt.Errorf("Collecting rows: %w", err)
	}

	return seeds_collect, nil
}

func GetActiveSeed(pool *pgxpool.Pool) (cashu.Seed, error) {
	rows, err := pool.Query(context.Background(), "SELECT * FROM seeds WHERE active")
	if err != nil {
		return cashu.Seed{}, fmt.Errorf("Error checking for Active seeds: %w", err)
	}
	defer rows.Close()

	seed, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[cashu.Seed])

	if err != nil {
		return seed, fmt.Errorf("GetActiveSeed: %w", err)
	}

	return seed, nil
}

func SaveNewSeed(pool *pgxpool.Pool, seed *cashu.Seed) error {
	_, err := pool.Exec(context.Background(), "INSERT INTO seeds (seed, active, created_at, unit, id, version) VALUES ($1, $2, $3, $4, $5, $6)", seed.Seed, seed.Active, seed.CreatedAt, seed.Unit, seed.Id, seed.Version)

	if err != nil {
		return fmt.Errorf("inserting to DB: %w", err)
	}
	return nil
}
func SaveNewSeeds(pool *pgxpool.Pool, seeds []cashu.Seed) error {

	entries := [][]any{}
	columns := []string{"seed", "active", "created_at", "unit", "id", "version"}
	tableName := "seeds"

	for _, seed := range seeds {
		entries = append(entries, []any{seed.Seed, seed.Active, seed.CreatedAt, seed.Unit, seed.Id, seed.Version})
	}

	_, err := pool.CopyFrom(context.Background(), pgx.Identifier{tableName}, columns, pgx.CopyFromRows(entries))

	if err != nil {
		return fmt.Errorf("inserting to DB: %w", err)
	}
	return nil
}

func SaveQuoteMintRequest(pool *pgxpool.Pool, request cashu.PostMintQuoteBolt11Response) error {

	_, err := pool.Exec(context.Background(), "INSERT INTO mint_request (quote, request, request_paid, expiry, unit, minted) VALUES ($1, $2, $3, $4, $5, $6)", request.Quote, request.Request, request.RequestPaid, request.Expiry, request.Unit, request.Minted)
	if err != nil {
		return fmt.Errorf("Inserting to mint_request: %w", err)

	}
	return nil
}
func ModifyQuoteMintPayStatus(pool *pgxpool.Pool, requestPaid bool, quote string) error {
	// change the paid status of the quote
	_, err := pool.Exec(context.Background(), "UPDATE mint_request SET request_paid = $1 WHERE quote = $2", requestPaid, quote)
	if err != nil {
		return fmt.Errorf("Inserting to mint_request: %w", err)

	}
	return nil
}

func ModifyQuoteMintMintedStatus(pool *pgxpool.Pool, minted bool, quote string) error {
	// change the paid status of the quote
	_, err := pool.Exec(context.Background(), "UPDATE mint_request SET minted = $1 WHERE quote = $2", minted, quote)
	if err != nil {
		return fmt.Errorf("Inserting to mint_request: %w", err)

	}
	return nil
}
func SaveQuoteMeltRequest(pool *pgxpool.Pool, request cashu.MeltRequestDB) error {

	_, err := pool.Exec(context.Background(), "INSERT INTO melt_request (quote, request, fee_reserve, expiry, unit, amount, request_paid, melted) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)", request.Quote, request.Request, request.FeeReserve, request.Expiry, request.Unit, request.Amount, request.RequestPaid, request.Melted)
	if err != nil {
		return fmt.Errorf("Inserting to mint_request: %w", err)

	}
	return nil
}
func ModifyQuoteMeltPayStatus(pool *pgxpool.Pool, paid bool, request string) error {
	// change the paid status of the quote
	_, err := pool.Exec(context.Background(), "UPDATE melt_request SET request_paid = $1 WHERE quote = $2", paid, request)
	if err != nil {
		return fmt.Errorf("Inserting to mint_request: %v", err)

	}
	return nil
}
func ModifyQuoteMeltPayStatusAndMelted(pool *pgxpool.Pool, paid bool, melted bool, request string) error {
	// change the paid status of the quote
	_, err := pool.Exec(context.Background(), "UPDATE melt_request SET request_paid = $1, melted = $3 WHERE quote = $2", paid, request, melted)
	if err != nil {
		return fmt.Errorf("Inserting to mint_request: %w", err)

	}
	return nil
}

func ModifyQuoteMeltMeltedStatus(pool *pgxpool.Pool, melted bool, quote string) error {
	// change the paid status of the quote
	_, err := pool.Exec(context.Background(), "UPDATE melt_request SET melted = $1 WHERE quote = $2", melted, quote)
	if err != nil {
		return fmt.Errorf("Inserting to mint_request: %w", err)

	}
	return nil
}

func GetMintQuoteById(pool *pgxpool.Pool, id string) (cashu.PostMintQuoteBolt11Response, error) {

	rows, err := pool.Query(context.Background(), "SELECT * FROM mint_request WHERE quote = $1", id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return cashu.PostMintQuoteBolt11Response{}, err
		}
	}
	defer rows.Close()

	quote, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[cashu.PostMintQuoteBolt11Response])

	if err != nil {
		if err == pgx.ErrNoRows {
			return cashu.PostMintQuoteBolt11Response{}, err
		}
		return quote, fmt.Errorf("CollectOneRow: %w", err)
	}

	return quote, nil
}
func GetMeltQuoteById(pool *pgxpool.Pool, id string) (cashu.MeltRequestDB, error) {

	rows, err := pool.Query(context.Background(), "SELECT * FROM melt_request WHERE quote = $1", id)
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
		return quote, fmt.Errorf("CollectOneRow: %w", err)
	}

	return quote, nil
}

func CheckListOfProofs(pool *pgxpool.Pool, CList []string, SecretList []string) ([]cashu.Proof, error) {

	var proofList []cashu.Proof

	rows, err := pool.Query(context.Background(), "SELECT * FROM proofs WHERE C = ANY($1) OR secret = ANY($2)", CList, SecretList)

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
		return proofList, fmt.Errorf("CollectOneRow: %w", err)
	}

	proofList = proof

	return proofList, nil
}

func SaveProofs(pool *pgxpool.Pool, proofs []cashu.Proof) error {
	entries := [][]any{}
	columns := []string{"c", "secret", "amount", "id", "y", "witness"}
	tableName := "proofs"

	for _, proof := range proofs {
		entries = append(entries, []any{proof.C, proof.Secret, proof.Amount, proof.Id, proof.Y, proof.Witness})
	}

	_, err := pool.CopyFrom(context.Background(), pgx.Identifier{tableName}, columns, pgx.CopyFromRows(entries))

	if err != nil {
		return fmt.Errorf("inserting to DB: %w", err)
	}
	return nil
}

func CheckListOfProofsBySecretCurve(pool *pgxpool.Pool, Ys []string) ([]cashu.Proof, error) {

	var proofList []cashu.Proof

	rows, err := pool.Query(context.Background(), "SELECT * FROM proofs WHERE Y = ANY($1)", Ys)

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
		return proofList, fmt.Errorf("CollectOneRow: %w", err)
	}

	proofList = proof

	return proofList, nil
}

func GetRestoreSigsFromBlindedMessages(pool *pgxpool.Pool, B_ []string) ([]cashu.RecoverSigDB, error) {

	var signaturesList []cashu.RecoverSigDB

	rows, err := pool.Query(context.Background(), `SELECT id, amount, "C_", "B_", created_at, witness  FROM recovery_signature WHERE "B_" = ANY($1)`, B_)

	if err != nil {
		if err == pgx.ErrNoRows {
			return signaturesList, nil
		}
		return signaturesList, fmt.Errorf("pool.Query: %w", err)
	}

	signatures, err := pgx.CollectRows(rows, pgx.RowToStructByName[cashu.RecoverSigDB])

	if err != nil {
		if err == pgx.ErrNoRows {
			return signaturesList, nil
		}
		return signaturesList, fmt.Errorf("CollectOneRow: %w", err)
	}

	signaturesList = signatures

	return signaturesList, nil
}

func SetRestoreSigs(pool *pgxpool.Pool, recover_sigs []cashu.RecoverSigDB) error {
	entries := [][]any{}
	columns := []string{"id", "amount", "B_", "C_", "created_at", "witness"}
	tableName := "recovery_signature"

	for _, sig := range recover_sigs {
		entries = append(entries, []any{sig.Id, sig.Amount, sig.B_, sig.C_, sig.CreatedAt, sig.Witness})
	}

	_, err := pool.CopyFrom(context.Background(), pgx.Identifier{tableName}, columns, pgx.CopyFromRows(entries))

	if err != nil {
		return fmt.Errorf("inserting to DB: %w", err)
	}
	return nil
}
