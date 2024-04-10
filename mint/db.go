package main

import (
	"context"
	"log"
	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/cashu-v4v/cashu"
)

func GetAllSeeds(conn *pgx.Conn) []cashu.Seed {
	var seeds []cashu.Seed

	rows, err := conn.Query(context.Background(), "SELECT * FROM seeds")

	if err != nil {
		if err == pgx.ErrNoRows {
			return seeds
		}
		log.Fatal("Error checking for  seeds: ", err)
	}

	defer rows.Close()

	keysets_collect, err := pgx.CollectRows(rows, pgx.RowToStructByName[cashu.Seed])

	if err != nil {
		log.Fatal("Error checking for seeds: ", err)
	}

	return keysets_collect
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

func CheckForActiveKeyset(conn *pgx.Conn) []cashu.Keyset {
	var keysets []cashu.Keyset

	rows, err := conn.Query(context.Background(), "SELECT * FROM keysets WHERE active")
	if err != nil {
		if err == pgx.ErrNoRows {
			return keysets
		}
		log.Fatal("Error checking for active keyset: ", err)
	}
	defer rows.Close()

	keysets_collect, err := pgx.CollectRows(rows, pgx.RowToStructByName[cashu.Keyset])

	if err != nil {
		log.Fatal("Error checking for active keyset: ", err)
	}

	return keysets_collect
}

func CheckForKeysetById(conn *pgx.Conn, id string) []cashu.Keyset {
	var keysets []cashu.Keyset

	rows, err := conn.Query(context.Background(), "SELECT * FROM keysets WHERE id = $1", id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return keysets
		}
		log.Fatal("Error checking for active keyset: ", err)
	}
	defer rows.Close()

	keysets_collect, err := pgx.CollectRows(rows, pgx.RowToStructByName[cashu.Keyset])

	if err != nil {
		log.Fatal("Error checking for active keyset: ", err)
	}

	return keysets_collect
}

func SaveNewSeed(conn *pgx.Conn, seed *cashu.Seed) {
	_, err := conn.Exec(context.Background(), "INSERT INTO seeds (seed, active, created_at, unit, id) VALUES ($1, $2, $3, $4, $5)", seed.Seed, seed.Active, seed.CreatedAt, seed.Unit, seed.Id)
	if err != nil {
		log.Fatal("Error saving new seed: ", err)
	}
}

func SaveNewKeysets(conn *pgx.Conn, keyset []cashu.Keyset) {
	for _, key := range keyset {
		_, err := conn.Exec(context.Background(), "INSERT INTO keysets (id, active, unit, amount, pubkey, created_at) VALUES ($1, $2, $3, $4, $5, $6)", key.Id, key.Active, key.Unit, key.Amount, key.PubKey, key.CreatedAt)
		if err != nil {
			log.Fatal("Error saving new keyset: ", err)
		}
	}
}
