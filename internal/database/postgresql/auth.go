package postgresql

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/internal/database"
)

func (pql Postgresql) MakeAuthUser(tx pgx.Tx, auth database.AuthUser) error {

	_, err := tx.Exec(context.Background(), "INSERT INTO user_auth (sub, aud , last_logged_in) VALUES ($1, $2, $3)", auth.Sub, auth.Aud, auth.LastLoggedIn)

	if err != nil {
		return databaseError(fmt.Errorf("inserting to auth user login: %w", err))

	}
	return nil

}

func (pql Postgresql) GetAuthUser(tx pgx.Tx, sub string) (database.AuthUser, error) {
	rows, err := tx.Query(context.Background(), "SELECT sub, aud , last_logged_in FROM user_auth WHERE sub = $1 FOR UPDATE", sub)
	if err != nil {
		return database.AuthUser{}, fmt.Errorf("error checking for active seeds: %w", err)
	}
	defer rows.Close()

	nostrLogin, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[database.AuthUser])

	if err != nil {
		return nostrLogin, fmt.Errorf("pgx.CollectOneRow(rows, pgx.RowToStructByName[cashu.NostrLoginAuth]): %w", err)
	}

	return nostrLogin, nil
}

func (pql Postgresql) UpdateLastLoggedIn(tx pgx.Tx, sub string, lastLoggedIn uint64) error {
	// change the paid status of the quote
	_, err := tx.Exec(context.Background(), "UPDATE user_auth SET last_logged_in = $1 WHERE sub = $2", lastLoggedIn, sub)
	if err != nil {
		return databaseError(fmt.Errorf("update to seeds: %w", err))

	}
	return nil
}
