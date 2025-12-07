package postgresql

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/utils"
)

func (pql Postgresql) SaveNostrAuth(auth database.NostrLoginAuth) error {
	_, err := pql.pool.Exec(context.Background(), "INSERT INTO nostr_login (nonce, expiry , activated) VALUES ($1, $2, $3)", auth.Nonce, auth.Expiry, auth.Activated)

	if err != nil {
		return databaseError(fmt.Errorf("Inserting to nostr_login: %w", err))

	}
	return nil
}

func (pql Postgresql) UpdateNostrAuthActivation(tx pgx.Tx, nonce string, activated bool) error {
	// change the paid status of the quote
	_, err := tx.Exec(context.Background(), "UPDATE nostr_login SET activated = $1 WHERE nonce = $2", activated, nonce)
	if err != nil {
		return databaseError(fmt.Errorf("Update to seeds: %w", err))

	}
	return nil
}

func (pql Postgresql) GetNostrAuth(tx pgx.Tx, nonce string) (database.NostrLoginAuth, error) {
	rows, err := tx.Query(context.Background(), "SELECT nonce, activated, expiry FROM nostr_login WHERE nonce = $1 FOR UPDATE", nonce)
	if err != nil {
		return database.NostrLoginAuth{}, fmt.Errorf("Error checking for Active seeds: %w", err)
	}
	defer rows.Close()

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
	batch.Queue("SELECT quote, request, request_paid, expiry, unit, minted, state, seen_at, amount, checking_id, pubkey, description FROM mint_request WHERE seen_at >= $1 AND (state = 'ISSUED' OR state = 'PAID') ", time)
	batch.Queue("SELECT quote, request, amount, request_paid, expiry, unit, melted, fee_reserve, state, payment_preimage, seen_at, mpp, fee_paid, checking_id FROM melt_request WHERE seen_at >= $1 AND (state = 'ISSUED' OR state = 'PAID')", time)

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

func (pql Postgresql) AddLiquiditySwap(tx pgx.Tx, swap utils.LiquiditySwap) error {
	_, err := tx.Exec(context.Background(), "INSERT INTO liquidity_swaps (amount, id , lightning_invoice, state, type, expiration, checking_id) VALUES ($1, $2, $3, $4, $5, $6, $7)", swap.Amount, swap.Id, swap.LightningInvoice, swap.State, swap.Type, swap.Expiration, swap.CheckingId)

	if err != nil {
		return databaseError(fmt.Errorf("INSERT INTO swap_request: %w", err))

	}
	return nil
}
func (pql Postgresql) ChangeLiquiditySwapState(tx pgx.Tx, id string, state utils.SwapState) error {
	_, err := tx.Exec(context.Background(), "UPDATE liquidity_swaps SET state = $1 WHERE id = $2", state, id)

	if err != nil {
		return databaseError(fmt.Errorf("Update liquidity_swaps: %w", err))

	}
	return nil
}

func (pql Postgresql) GetLiquiditySwaps(swap utils.LiquiditySwap) ([]utils.LiquiditySwap, error) {

	var swaps []utils.LiquiditySwap
	rows, err := pql.pool.Query(context.Background(), "SELECT amount, id, lightning_invoice, state,type, expiration, checking_id FROM liquidity_swaps ")
	if err != nil {
		return swaps, fmt.Errorf("Error checking for Active seeds: %w", err)
	}
	defer rows.Close()

	swaps, err = pgx.CollectRows(rows, pgx.RowToStructByName[utils.LiquiditySwap])

	if err != nil {
		return swaps, fmt.Errorf("pgx.CollectOneRow(rows, pgx.RowToStructByName[cashu.NostrLoginAuth]): %w", err)
	}

	return swaps, nil
}

func (pql Postgresql) GetLiquiditySwapById(tx pgx.Tx, id string) (utils.LiquiditySwap, error) {

	var swaps utils.LiquiditySwap
	rows, err := tx.Query(context.Background(), "SELECT amount, id, lightning_invoice, state,type, expiration, checking_id FROM liquidity_swaps WHERE id = $1 FOR SHARE", id)
	if err != nil {
		return swaps, fmt.Errorf("Error checking for Active seeds: %w", err)
	}
	defer rows.Close()

	swaps, err = pgx.CollectOneRow(rows, pgx.RowToStructByName[utils.LiquiditySwap])

	if err != nil {
		return swaps, fmt.Errorf("pgx.CollectOneRow(rows, pgx.RowToStructByName[cashu.NostrLoginAuth]): %w", err)
	}

	return swaps, nil
}

func (pql Postgresql) GetAllLiquiditySwaps() ([]utils.LiquiditySwap, error) {

	var swaps []utils.LiquiditySwap
	rows, err := pql.pool.Query(context.Background(), "SELECT amount, id, lightning_invoice, state,type,expiration, checking_id FROM liquidity_swaps ORDER BY expiration DESC")
	if err != nil {
		return swaps, fmt.Errorf("Error checking for Active seeds: %w", err)
	}
	defer rows.Close()

	swaps, err = pgx.CollectRows(rows, pgx.RowToStructByName[utils.LiquiditySwap])

	if err != nil {
		return swaps, fmt.Errorf("pgx.CollectOneRow(rows, pgx.RowToStructByName[cashu.NostrLoginAuth]): %w", err)
	}

	return swaps, nil
}

func (pql Postgresql) GetLiquiditySwapsByStates(states []utils.SwapState) ([]utils.LiquiditySwap, error) {

	var swaps []utils.LiquiditySwap
	rows, err := pql.pool.Query(context.Background(), "SELECT amount, id, lightning_invoice, state,type,expiration, checking_id FROM liquidity_swaps WHERE state = ANY($1) ORDER BY expiration DESC FOR UPDATE NOWAIT", states)
	if err != nil {
		return swaps, fmt.Errorf("Error checking for liquidity swaps: %w", err)
	}
	defer rows.Close()

	swaps, err = pgx.CollectRows(rows, pgx.RowToStructByName[utils.LiquiditySwap])

	if err != nil {
		return swaps, fmt.Errorf("pgx.CollectOneRow(rows, pgx.RowToStructByName[cashu.NostrLoginAuth]): %w", err)
	}

	return swaps, nil
}
