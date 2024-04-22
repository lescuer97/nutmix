package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/cashu"
)

func GetAllSeeds(conn *pgx.Conn) ([]cashu.Seed, error) {
	var seeds []cashu.Seed

	rows, err := conn.Query(context.Background(), "SELECT * FROM seeds")

	if err != nil {
		if err == pgx.ErrNoRows {
			return seeds, fmt.Errorf("No rows found: %v", err)
		}
		log.Fatal("Error checking for  seeds: ", err)
	}

	defer rows.Close()

	keysets_collect, err := pgx.CollectRows(rows, pgx.RowToStructByName[cashu.Seed])

	if err != nil {
		return keysets_collect, fmt.Errorf("Collecting rows: %v", err)
	}

	return keysets_collect, nil
}

func GetActiveSeed(conn *pgx.Conn) (cashu.Seed, error) {

	var err error
	rows, err := conn.Query(context.Background(), "SELECT * FROM seeds WHERE active")
	if err != nil {
		log.Fatal("Error checking for active keyset: ", err)
	}
	defer rows.Close()

	seed, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[cashu.Seed])

	if err != nil {
		log.Fatal("Error checking for active keyset: ", err)
		return seed, err
	}

	return seed, nil
}

func CheckForActiveKeyset(conn *pgx.Conn) ([]cashu.Keyset, error) {
	var keysets []cashu.Keyset

	rows, err := conn.Query(context.Background(), "SELECT * FROM keysets WHERE active")
	if err != nil {
		if err == pgx.ErrNoRows {
			return keysets, fmt.Errorf("No rows found: %v", err)
		}
		log.Fatal("Error checking for active keyset: ", err)
	}
	defer rows.Close()

	keysets_collect, err := pgx.CollectRows(rows, pgx.RowToStructByName[cashu.Keyset])

	if err != nil {

		return keysets_collect, fmt.Errorf("Collecting rows: %v", err)
	}

	return keysets_collect, nil
}

func CheckForKeysetById(conn *pgx.Conn, id string) ([]cashu.Keyset, error) {
	var keysets []cashu.Keyset

	rows, err := conn.Query(context.Background(), "SELECT * FROM keysets WHERE id = $1", id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return keysets, fmt.Errorf("No rows found: %v", err)
		}
	}
	defer rows.Close()

	keysets_collect, err := pgx.CollectRows(rows, pgx.RowToStructByName[cashu.Keyset])

	if err != nil {
		return keysets_collect, fmt.Errorf("Collecting rows: %v", err)
	}

	return keysets_collect, nil
}

func SaveNewSeed(conn *pgx.Conn, seed *cashu.Seed) error {
	_, err := conn.Exec(context.Background(), "INSERT INTO seeds (seed, active, created_at, unit, id) VALUES ($1, $2, $3, $4, $5)", seed.Seed, seed.Active, seed.CreatedAt, seed.Unit, seed.Id)
	if err != nil {
		log.Fatal("Error saving new seed: ", err)
		return fmt.Errorf("inserting to DB: %v", err)
	}
	return nil
}

func SaveNewKeysets(conn *pgx.Conn, keyset []cashu.Keyset) error {
	for _, key := range keyset {
		_, err := conn.Exec(context.Background(), "INSERT INTO keysets (id, active, unit, amount, privkey, created_at) VALUES ($1, $2, $3, $4, $5, $6)", key.Id, key.Active, key.Unit, key.Amount, key.PrivKey, key.CreatedAt)
		if err != nil {
			return fmt.Errorf("Inserting to keysets: %v", err)
		}
	}
	return nil
}

func SaveQuoteRequest(conn *pgx.Conn, request cashu.PostMintQuoteBolt11Response) error {
	_, err := conn.Exec(context.Background(), "INSERT INTO mint_request (quote, request, paid, expiry) VALUES ($1, $2, $3, $4)", request.Quote, request.Request, request.Paid, request.Expiry)
	if err != nil {
		return fmt.Errorf("Inserting to mint_request: %v", err)

	}
	return nil
}

func GetQuoteById(conn *pgx.Conn, id string) (cashu.PostMintQuoteBolt11Response, error) {

	rows, err := conn.Query(context.Background(), "SELECT * FROM mint_request WHERE quote = $1", id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return cashu.PostMintQuoteBolt11Response{}, fmt.Errorf("No rows found: %v", err)
		}
	}
	defer rows.Close()

	quote, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[cashu.PostMintQuoteBolt11Response])

	if err != nil {
		return quote, fmt.Errorf("CollectOneRow: %v", err)
	}

	return quote, nil
}

func GetKeysetsByAmountList(conn *pgx.Conn, keyAmounts []int32) (map[int]cashu.Keyset, error) {
    var keysetMap = make(map[int]cashu.Keyset)

	rows, err := conn.Query(context.Background(), "SELECT * FROM keysets WHERE amount = ANY($1)", keyAmounts)
	if err != nil {
		if err == pgx.ErrNoRows {
			return keysetMap, fmt.Errorf("No rows found: %v", err)
		}
		log.Fatal("Error checking for active keyset: ", err)
	}
	defer rows.Close()

	keysets_collect, err := pgx.CollectRows(rows, pgx.RowToStructByName[cashu.Keyset])

	if err != nil {

		return keysetMap, fmt.Errorf("Collecting rows: %v", err)
	}

    for _, keyset := range keysets_collect {
        keysetMap[keyset.Amount] = keyset
    }

	return keysetMap, nil
}
