package postgresql

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
)

func (pql Postgresql) SaveNostrAuth(auth database.NostrLoginAuth) error {
	_, err := pql.pool.Exec(context.Background(), "INSERT INTO nostr_login (nonce, expiry , activated) VALUES ($1, $2, $3)", auth.Nonce, auth.Expiry, auth.Activated)

	if err != nil {
		return databaseError(fmt.Errorf("Inserting to nostr_login: %w", err))

	}
	return nil
}

func (pql Postgresql) UpdateNostrAuthActivation(nonce string, activated bool) error {
	// change the paid status of the quote
	_, err := pql.pool.Exec(context.Background(), "UPDATE nostr_login SET activated = $1 WHERE nonce = $2", activated, nonce)
	if err != nil {
		return databaseError(fmt.Errorf("Update to seeds: %w", err))

	}
	return nil
}

func (pql Postgresql) GetNostrAuth(nonce string) (database.NostrLoginAuth, error) {
	rows, err := pql.pool.Query(context.Background(), "SELECT nonce, activated, expiry FROM nostr_login WHERE nonce = $1", nonce)
	defer rows.Close()
	if err != nil {
		return database.NostrLoginAuth{}, fmt.Errorf("Error checking for Active seeds: %w", err)
	}

	nostrLogin, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[database.NostrLoginAuth])

	if err != nil {
		return nostrLogin, fmt.Errorf("pgx.CollectOneRow(rows, pgx.RowToStructByName[cashu.NostrLoginAuth]): %w", err)
	}

	return nostrLogin, nil

}

func (pql Postgresql) GetMintMeltBalanceByTime(time int64) (database.MintMeltBalance, error) {
	var mintMeltBalance database.MintMeltBalance
	// change the paid status of the quote
	batch := pgx.Batch{}
	batch.Queue("SELECT quote, request, request_paid, expiry, unit, minted, state, seen_at FROM mint_request WHERE seen_at >= $1 AND (state = 'ISSUED' OR state = 'PAID') ", time)
	batch.Queue("SELECT quote, request, amount, request_paid, expiry, unit, melted, fee_reserve, state, payment_preimage, seen_at, mpp FROM melt_request WHERE seen_at >= $1 AND (state = 'ISSUED' OR state = 'PAID')", time)

	results := pql.pool.SendBatch(context.Background(), &batch)

	defer results.Close()

	mintRows, err := results.Query()
	if err != nil {
		if err == pgx.ErrNoRows {
			return mintMeltBalance, err
		}
		return mintMeltBalance, databaseError(fmt.Errorf(" results.Query(): %w", err))
	}
	mintRequest, err := pgx.CollectRows(mintRows, pgx.RowToStructByName[cashu.MintRequestDB])

	if err != nil {
		if err == pgx.ErrNoRows {
			return mintMeltBalance, err
		}
		return mintMeltBalance, databaseError(fmt.Errorf("pgx.CollectRows(rows, pgx.RowToStructByName[cashu.MintRequestDB]): %w", err))
	}

	meltRows, err := results.Query()
	if err != nil {
		if err == pgx.ErrNoRows {
			return mintMeltBalance, err
		}
		return mintMeltBalance, databaseError(fmt.Errorf(" results.Query(): %w", err))
	}
	defer meltRows.Close()
	meltRequest, err := pgx.CollectRows(meltRows, pgx.RowToStructByName[cashu.MeltRequestDB])

	if err != nil {
		if err == pgx.ErrNoRows {
			return mintMeltBalance, err
		}
		return mintMeltBalance, databaseError(fmt.Errorf("pgx.CollectRows(rows, pgx.RowToStructByName[cashu.MintRequestDB]): %w", err))
	}

	mintMeltBalance.Melt = meltRequest
	mintMeltBalance.Mint = mintRequest

	defer mintRows.Close()

	return mintMeltBalance, nil

}
