package postgresql

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/internal/utils"
)

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
		return databaseError(fmt.Errorf("update liquidity_swaps: %w", err))
	}
	return nil
}

func (pql Postgresql) GetLiquiditySwaps(swap utils.LiquiditySwap) ([]utils.LiquiditySwap, error) {
	var swaps []utils.LiquiditySwap
	rows, err := pql.pool.Query(context.Background(), "SELECT amount, id, lightning_invoice, state,type, expiration, checking_id FROM liquidity_swaps ")
	if err != nil {
		return swaps, fmt.Errorf("error checking for active seeds: %w", err)
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
		return swaps, fmt.Errorf("error checking for active seeds: %w", err)
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
		return swaps, fmt.Errorf("error checking for active seeds: %w", err)
	}
	defer rows.Close()

	swaps, err = pgx.CollectRows(rows, pgx.RowToStructByName[utils.LiquiditySwap])
	if err != nil {
		return swaps, fmt.Errorf("pgx.CollectOneRow(rows, pgx.RowToStructByName[cashu.NostrLoginAuth]): %w", err)
	}

	return swaps, nil
}

func (pql Postgresql) GetLiquiditySwapsByStates(tx pgx.Tx, states []utils.SwapState) ([]string, error) {
	swapIDs := make([]string, 0)
	rows, err := tx.Query(context.Background(), "SELECT id FROM liquidity_swaps WHERE state = ANY($1) ORDER BY expiration DESC FOR UPDATE", states)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return swapIDs, nil
		}
		return nil, fmt.Errorf("error checking for liquidity swaps: %w", err)
	}
	defer rows.Close()

	swapIDs, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (string, error) {
		var id string
		err := row.Scan(&id)
		return id, err
	})
	if err != nil {
		return swapIDs, fmt.Errorf("pgx.CollectRows(rows, func(row pgx.CollectableRow) : %w", err)
	}

	return swapIDs, nil
}
