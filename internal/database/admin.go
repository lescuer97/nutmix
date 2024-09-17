package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lescuer97/nutmix/api/cashu"
	// "github.com/lescuer97/nutmix/api/cashu"
)

type MintMeltBalance struct {
	Mint []cashu.MintRequestDB
	Melt []cashu.MeltRequestDB
}

func GetMintMeltBalanceByTime(pool *pgxpool.Pool, time int64) (MintMeltBalance, error) {
	var mintMeltBalance MintMeltBalance
	// change the paid status of the quote
	batch := pgx.Batch{}
	batch.Queue("SELECT quote, request, request_paid, expiry, unit, minted, state, seen_at FROM mint_request WHERE seen_at >= $1 AND (state = 'ISSUED' OR state = 'PAID') ", time)
	batch.Queue("SELECT quote, request, amount, request_paid, expiry, unit, melted, fee_reserve, state, payment_preimage, seen_at FROM melt_request WHERE seen_at >= $1 AND (state = 'ISSUED' OR state = 'PAID')", time)

	results := pool.SendBatch(context.Background(), &batch)

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
