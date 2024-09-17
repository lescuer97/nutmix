package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lescuer97/nutmix/api/cashu"
)

func SaveNostrLoginAuth(pool *pgxpool.Pool, auth cashu.NostrLoginAuth) error {
	_, err := pool.Exec(context.Background(), "INSERT INTO nostr_login (nonce, expiry , activated) VALUES ($1, $2, $3)", auth.Nonce, auth.Expiry, auth.Activated)

	if err != nil {
		return databaseError(fmt.Errorf("Inserting to nostr_login: %w", err))

	}
	return nil
}

func UpdateNostrLoginActivation(pool *pgxpool.Pool, auth cashu.NostrLoginAuth) error {
	// change the paid status of the quote
	_, err := pool.Exec(context.Background(), "UPDATE nostr_login SET activated = $1 WHERE nonce = $2", auth.Activated, auth.Nonce)
	if err != nil {
		return databaseError(fmt.Errorf("Update to seeds: %w", err))

	}
	return nil
}

func GetNostrLogin(pool *pgxpool.Pool, nonce string) (cashu.NostrLoginAuth, error) {
	rows, err := pool.Query(context.Background(), "SELECT nonce, activated, expiry FROM nostr_login WHERE nonce = $1", nonce)
	defer rows.Close()
	if err != nil {
		return cashu.NostrLoginAuth{}, fmt.Errorf("Error checking for Active seeds: %w", err)
	}

	nostrLogin, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[cashu.NostrLoginAuth])

	if err != nil {
		return nostrLogin, fmt.Errorf("pgx.CollectOneRow(rows, pgx.RowToStructByName[cashu.NostrLoginAuth]): %w", err)
	}

	return nostrLogin, nil

}
