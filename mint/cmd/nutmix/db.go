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

func DatabaseSetup() (*pgxpool.Pool, error) {
	databaseConUrl := os.Getenv("DATABASE_URL")

	pool, err := pgxpool.New(context.Background(), databaseConUrl)

	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatalf("Error setting dialect: %v", err)
	}

	db := stdlib.OpenDBFromPool(pool)

	if err := goose.Up(db, "migrations"); err != nil {
		log.Fatalf("Error running migrations: %v", err)
	}

	if err := db.Close(); err != nil {
		panic(err)
	}

	if err != nil {
		return nil, fmt.Errorf("Error connecting to database: %v", err)
	}

	return pool, nil
}

func GetAllSeeds(pool *pgxpool.Pool) ([]cashu.Seed, error) {
	var seeds []cashu.Seed

	rows, err := pool.Query(context.Background(), "SELECT * FROM seeds")

	if err != nil {
		if err == pgx.ErrNoRows {
			return seeds, fmt.Errorf("No rows found: %v", err)
		}

		return seeds, fmt.Errorf("Error checking for  seeds: %+v", err)
	}

	defer rows.Close()

	keysets_collect, err := pgx.CollectRows(rows, pgx.RowToStructByName[cashu.Seed])

	if err != nil {
		return keysets_collect, fmt.Errorf("Collecting rows: %v", err)
	}

	return keysets_collect, nil
}

func GetActiveSeed(pool *pgxpool.Pool) (cashu.Seed, error) {
	rows, err := pool.Query(context.Background(), "SELECT * FROM seeds WHERE active")
	if err != nil {
		return cashu.Seed{}, fmt.Errorf("Error checking for Active seeds: %+v", err)
	}
	defer rows.Close()

	seed, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[cashu.Seed])

	if err != nil {
		return seed, fmt.Errorf("GetActiveSeed: %v", err)
	}

	return seed, nil
}

func SaveNewSeed(pool *pgxpool.Pool, seed *cashu.Seed) error {
	_, err := pool.Exec(context.Background(), "INSERT INTO seeds (seed, active, created_at, unit, id) VALUES ($1, $2, $3, $4, $5)", seed.Seed, seed.Active, seed.CreatedAt, seed.Unit, seed.Id)

	if err != nil {
		return fmt.Errorf("inserting to DB: %v", err)
	}
	return nil
}

func SaveQuoteMintRequest(pool *pgxpool.Pool, request cashu.PostMintQuoteBolt11Response) error {

	_, err := pool.Exec(context.Background(), "INSERT INTO mint_request (quote, request, paid, expiry) VALUES ($1, $2, $3, $4)", request.Quote, request.Request, request.Paid, request.Expiry)
	if err != nil {
		return fmt.Errorf("Inserting to mint_request: %v", err)

	}
	return nil
}
func ModifyQuoteMintPayStatus(pool *pgxpool.Pool, request cashu.PostMintQuoteBolt11Response) error {

	// change the paid status of the quote
	_, err := pool.Exec(context.Background(), "UPDATE mint_request SET paid = $1 WHERE quote = $2", request.Paid, request.Quote)
	if err != nil {
		return fmt.Errorf("Inserting to mint_request: %v", err)

	}
	return nil
}
func SaveQuoteMeltRequest(pool *pgxpool.Pool, request cashu.MeltRequestDB) error {

	_, err := pool.Exec(context.Background(), "INSERT INTO melt_request (quote, request, fee_reserve, expiry, unit, amount, paid) VALUES ($1, $2, $3, $4, $5, $6, $7)", request.Quote, request.Request, request.FeeReserve, request.Expiry, request.Unit, request.Amount, request.Paid)
	if err != nil {
		return fmt.Errorf("Inserting to mint_request: %v", err)

	}
	return nil
}
func ModifyQuoteMeltPayStatus(pool *pgxpool.Pool, paid bool, request string) error {
	// change the paid status of the quote
	_, err := pool.Exec(context.Background(), "UPDATE melt_request SET paid = $1 WHERE quote = $2", paid, request)
	if err != nil {
		return fmt.Errorf("Inserting to mint_request: %v", err)

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
		return quote, fmt.Errorf("CollectOneRow: %v", err)
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
		return quote, fmt.Errorf("CollectOneRow: %v", err)
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
		return proofList, fmt.Errorf("CollectOneRow: %v", err)
	}

	proofList = proof

	return proofList, nil
}

func SaveProofs(pool *pgxpool.Pool, proofs []cashu.Proof) error {
	for _, proof := range proofs {
		_, err := pool.Exec(context.Background(), "INSERT INTO proofs (C, secret, amount, id, Y) VALUES ($1, $2, $3, $4, $5)", proof.C, proof.Secret, proof.Amount, proof.Id, proof.Y)
		if err != nil {
			return fmt.Errorf("Inserting to proofs: %v", err)
		}
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
		return proofList, fmt.Errorf("CollectOneRow: %v", err)
	}

	proofList = proof

	return proofList, nil
}
