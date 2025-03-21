package mockdb

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/internal/database"
)

func (m *MockDB) MakeAuthUser(tx pgx.Tx,auth database.AuthUser)  error{

	_, err := tx.Exec(context.Background(), "INSERT INTO user_auth (sub, aud , last_logged_in) VALUES ($1, $2, $3)", auth.Sub, auth.Aud, auth.LastLoggedIn)

	if err != nil {
		return databaseError(fmt.Errorf("Inserting to auth user login: %w", err))

	}
	return nil

}

func (m *MockDB) GetAuthUser(tx pgx.Tx,sub string) (database.AuthUser, error){
	rows, err := tx.Query(context.Background(), "SELECT sub, aud , last_logged_in FROM user_auth WHERE nonce = $1 FOR UPDATE", sub)
	defer rows.Close()
	if err != nil {
		return database.AuthUser{}, fmt.Errorf("Error checking for Active seeds: %w", err)
	}

	nostrLogin, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[database.AuthUser])

	if err != nil {
		return nostrLogin, fmt.Errorf("pgx.CollectOneRow(rows, pgx.RowToStructByName[cashu.NostrLoginAuth]): %w", err)
	}

	return nostrLogin, nil

}

func (m *MockDB) UpdateLastLoggedIn(tx pgx.Tx, sub string, lastLoggedIn uint64)  error{
	// change the paid status of the quote
	_, err := tx.Exec(context.Background(), "UPDATE user_auth SET last_logged_in = $1 WHERE sub = $2", lastLoggedIn, sub)
	if err != nil {
		return databaseError(fmt.Errorf("Update to seeds: %w", err))

	}
	return nil
}
