package main

import (
	"context"
	"fmt"
	"log"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lescuer97/nutmix/cashu"
)

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

func CheckForActiveKeyset(pool *pgxpool.Pool) ([]cashu.Keyset, error) {
	var keysets []cashu.Keyset

	rows, err := pool.Query(context.Background(), "SELECT * FROM keysets WHERE active")
	if err != nil {
		if err == pgx.ErrNoRows {
			return keysets, fmt.Errorf("No rows found: %v", err)
		}
		return keysets, fmt.Errorf("Error checking for active keyset: %+v", err)
	}
	defer rows.Close()

	var keysets_collect []cashu.Keyset

	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			log.Printf("error while iterating dataset, %+v", err)
			return keysets_collect, err
		}

		privKey := secp256k1.PrivKeyFromBytes(values[4].([]byte))
		amount := values[3].(int32)

		keyset := cashu.Keyset{
			Id:        values[0].(string),
			Active:    values[1].(bool),
			Unit:      values[2].(string),
			Amount:    int(amount),
			PrivKey:   privKey,
			CreatedAt: values[5].(int64),
		}

		keysets_collect = append(keysets_collect, keyset)

	}

	return keysets_collect, nil
}

func CheckForKeysetById(pool *pgxpool.Pool, id string) ([]cashu.Keyset, error) {
	var keysets []cashu.Keyset

	rows, err := pool.Query(context.Background(), "SELECT * FROM keysets WHERE id = $1", id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return keysets, fmt.Errorf("No rows found: %v", err)
		}
	}
	defer rows.Close()
	var keysets_collect []cashu.Keyset
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			log.Printf("error while iterating dataset, %+v", err)
			return keysets_collect, err
		}

		privKey := secp256k1.PrivKeyFromBytes(values[4].([]byte))
		amount := values[3].(int32)

		keyset := cashu.Keyset{
			Id:        values[0].(string),
			Active:    values[1].(bool),
			Unit:      values[2].(string),
			Amount:    int(amount),
			PrivKey:   privKey,
			CreatedAt: values[5].(int64),
		}
		keysets_collect = append(keysets_collect, keyset)

	}

	return keysets_collect, nil
}

func SaveNewSeed(pool *pgxpool.Pool, seed *cashu.Seed) error {
    _, err := pool.Exec(context.Background(), "INSERT INTO seeds (seed, active, created_at, unit, id) VALUES ($1, $2, $3, $4, $5)", seed.Seed, seed.Active, seed.CreatedAt, seed.Unit, seed.Id)

	if err != nil {
		return fmt.Errorf("inserting to DB: %v", err)
	}
	return nil
}

func SaveNewKeysets(pool *pgxpool.Pool, keyset []cashu.Keyset) error {
	for _, key := range keyset {
		conn, err := pool.Acquire(context.Background())

		if err != nil {
			return fmt.Errorf("Error acquiring connection: %v", err)
		}

		_, err = conn.Exec(context.Background(), "INSERT INTO keysets (id, active, unit, amount, privkey, created_at) VALUES ($1, $2, $3, $4, $5, $6)", key.Id, key.Active, key.Unit, key.Amount, key.PrivKey.Serialize(), key.CreatedAt)
		if err != nil {
			return fmt.Errorf("Inserting to keysets: %v", err)
		}
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
func ModifyQuoteMeltPayStatus(pool *pgxpool.Pool, request cashu.MeltRequestDB) error {
	// change the paid status of the quote
    _, err := pool.Exec(context.Background(), "UPDATE melt_request SET paid = $1 WHERE quote = $2", request.Paid, request.Quote)
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

func GetKeysetsByAmountList(pool *pgxpool.Pool, keyAmounts []int32) (map[int]cashu.Keyset, error) {
	var keysetMap = make(map[int]cashu.Keyset)

	rows, err := pool.Query(context.Background(), "SELECT * FROM keysets WHERE amount = ANY($1)", keyAmounts)
	if err != nil {
		if err == pgx.ErrNoRows {
			return keysetMap, fmt.Errorf("No rows found: %v", err)
		}
		return keysetMap, fmt.Errorf("Error checking for keysets: %+v", err)
	}
	defer rows.Close()

	var keysets_collect []cashu.Keyset
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return keysetMap, fmt.Errorf("Error while iterating dataset: %v", err)
		}

		privKey := secp256k1.PrivKeyFromBytes(values[4].([]byte))
		amount := values[3].(int32)

		keyset := cashu.Keyset{
			Id:        values[0].(string),
			Active:    values[1].(bool),
			Unit:      values[2].(string),
			Amount:    int(amount),
			PrivKey:   privKey,
			CreatedAt: values[5].(int64),
		}
		keysets_collect = append(keysets_collect, keyset)

	}

	for _, keyset := range keysets_collect {
		keysetMap[keyset.Amount] = keyset
	}

	return keysetMap, nil
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

	proof, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[cashu.Proof])

	if err != nil {
		if err == pgx.ErrNoRows {
			return proofList, nil
		}
		return proofList, fmt.Errorf("CollectOneRow: %v", err)
	}

	proofList = append(proofList, proof)

	return proofList, nil
}

func SaveProofs(pool *pgxpool.Pool, proofs []cashu.Proof) error {
	for _, proof := range proofs {
        _, err := pool.Exec(context.Background(), "INSERT INTO proofs (C, secret, amount, id) VALUES ($1, $2, $3, $4)", proof.C, proof.Secret, proof.Amount, proof.Id)
		if err != nil {
			return fmt.Errorf("Inserting to proofs: %v", err)
		}
	}
	return nil
}
