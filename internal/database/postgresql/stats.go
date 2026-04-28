package postgresql

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/internal/database"
)

func normalizeStatsSummary(items []database.StatsSummaryItem) []database.StatsSummaryItem {
	if items == nil {
		return []database.StatsSummaryItem{}
	}
	return items
}

func (pql Postgresql) GetLatestStatsSnapshot(ctx context.Context) (*database.StatsSnapshot, error) {
	row := pql.pool.QueryRow(ctx, `SELECT id, start_date, end_date, mint_summary, melt_summary, blind_sigs_summary, proofs_summary, fees
		FROM stats
		ORDER BY end_date DESC, id DESC
		LIMIT 1`)

	var snapshot database.StatsSnapshot
	var mintSummary []byte
	var meltSummary []byte
	var blindSigsSummary []byte
	var proofsSummary []byte

	err := row.Scan(
		&snapshot.ID,
		&snapshot.StartDate,
		&snapshot.EndDate,
		&mintSummary,
		&meltSummary,
		&blindSigsSummary,
		&proofsSummary,
		&snapshot.Fees,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, databaseError(fmt.Errorf("GetLatestStatsSnapshot scan error: %w", err))
	}

	if err := json.Unmarshal(mintSummary, &snapshot.MintSummary); err != nil {
		return nil, databaseError(fmt.Errorf("unmarshal mint_summary: %w", err))
	}
	if err := json.Unmarshal(meltSummary, &snapshot.MeltSummary); err != nil {
		return nil, databaseError(fmt.Errorf("unmarshal melt_summary: %w", err))
	}
	if err := json.Unmarshal(blindSigsSummary, &snapshot.BlindSigsSummary); err != nil {
		return nil, databaseError(fmt.Errorf("unmarshal blind_sigs_summary: %w", err))
	}
	if err := json.Unmarshal(proofsSummary, &snapshot.ProofsSummary); err != nil {
		return nil, databaseError(fmt.Errorf("unmarshal proofs_summary: %w", err))
	}

	snapshot.MintSummary = normalizeStatsSummary(snapshot.MintSummary)
	snapshot.MeltSummary = normalizeStatsSummary(snapshot.MeltSummary)
	snapshot.BlindSigsSummary = normalizeStatsSummary(snapshot.BlindSigsSummary)
	snapshot.ProofsSummary = normalizeStatsSummary(snapshot.ProofsSummary)

	return &snapshot, nil
}

func collectRows[T any](rows pgx.Rows, collect func(pgx.CollectableRow) (T, error)) ([]T, error) {
	defer rows.Close()
	items, err := pgx.CollectRows(rows, collect)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return []T{}, nil
		}
		return nil, err
	}
	return items, nil
}

func (pql Postgresql) GetMintStatsRows(ctx context.Context, tx pgx.Tx, startDate, endDate int64) ([]database.MintStatsRow, error) {
	rows, err := tx.Query(ctx, `SELECT quote, unit, amount, request
		FROM mint_request
		WHERE seen_at >= $1 AND seen_at <= $2
		  AND (state = 'PAID' OR state = 'ISSUED')`, startDate, endDate)
	if err != nil {
		return nil, databaseError(fmt.Errorf("GetMintStatsRows query error: %w", err))
	}

	items, err := collectRows(rows, func(row pgx.CollectableRow) (database.MintStatsRow, error) {
		var item database.MintStatsRow
		err := row.Scan(&item.Quote, &item.Unit, &item.Amount, &item.Request)
		return item, err
	})
	if err != nil {
		return nil, databaseError(fmt.Errorf("GetMintStatsRows collect error: %w", err))
	}
	return items, nil
}

func (pql Postgresql) GetMeltStatsRows(ctx context.Context, tx pgx.Tx, startDate, endDate int64) ([]database.MeltStatsRow, error) {
	rows, err := tx.Query(ctx, `SELECT quote, unit, amount
		FROM melt_request
		WHERE seen_at >= $1 AND seen_at <= $2
		  AND (state = 'PAID' OR state = 'ISSUED')`, startDate, endDate)
	if err != nil {
		return nil, databaseError(fmt.Errorf("GetMeltStatsRows query error: %w", err))
	}

	items, err := collectRows(rows, func(row pgx.CollectableRow) (database.MeltStatsRow, error) {
		var item database.MeltStatsRow
		err := row.Scan(&item.Quote, &item.Unit, &item.Amount)
		return item, err
	})
	if err != nil {
		return nil, databaseError(fmt.Errorf("GetMeltStatsRows collect error: %w", err))
	}
	return items, nil
}

func (pql Postgresql) GetProofStatsRows(ctx context.Context, tx pgx.Tx, startDate, endDate int64) ([]database.KeysetStatsRow, error) {
	rows, err := tx.Query(ctx, `SELECT proofs.id AS keyset_id, proofs.amount, COALESCE(seeds.unit, '') AS unit
		FROM proofs
		LEFT JOIN seeds ON seeds.id = proofs.id
		WHERE proofs.seen_at >= $1 AND proofs.seen_at <= $2
		  AND proofs.state = 'SPENT'`, startDate, endDate)
	if err != nil {
		return nil, databaseError(fmt.Errorf("GetProofStatsRows query error: %w", err))
	}

	items, err := collectRows(rows, func(row pgx.CollectableRow) (database.KeysetStatsRow, error) {
		var item database.KeysetStatsRow
		err := row.Scan(&item.KeysetID, &item.Amount, &item.Unit)
		return item, err
	})
	if err != nil {
		return nil, databaseError(fmt.Errorf("GetProofStatsRows collect error: %w", err))
	}
	return items, nil
}

func (pql Postgresql) GetBlindSigStatsRows(ctx context.Context, tx pgx.Tx, startDate, endDate int64) ([]database.KeysetStatsRow, error) {
	rows, err := tx.Query(ctx, `SELECT recovery_signature.id AS keyset_id, recovery_signature.amount, COALESCE(seeds.unit, '') AS unit
		FROM recovery_signature
		LEFT JOIN seeds ON seeds.id = recovery_signature.id
		WHERE recovery_signature.created_at >= $1 AND recovery_signature.created_at <= $2`, startDate, endDate)
	if err != nil {
		return nil, databaseError(fmt.Errorf("GetBlindSigStatsRows query error: %w", err))
	}

	items, err := collectRows(rows, func(row pgx.CollectableRow) (database.KeysetStatsRow, error) {
		var item database.KeysetStatsRow
		err := row.Scan(&item.KeysetID, &item.Amount, &item.Unit)
		return item, err
	})
	if err != nil {
		return nil, databaseError(fmt.Errorf("GetBlindSigStatsRows collect error: %w", err))
	}
	return items, nil
}

func (pql Postgresql) GetStatsFeeRows(ctx context.Context, tx pgx.Tx, startDate, endDate int64) ([]database.KeysetFeeRow, error) {
	rows, err := tx.Query(ctx, `SELECT proofs.id AS keyset_id, seeds.unit, COUNT(*) AS quantity, seeds.input_fee_ppk
		FROM proofs
		JOIN seeds ON seeds.id = proofs.id
		WHERE proofs.seen_at >= $1 AND proofs.seen_at <= $2
		  AND proofs.state = 'SPENT'
		GROUP BY proofs.id, seeds.unit, seeds.input_fee_ppk
		ORDER BY proofs.id`, startDate, endDate)
	if err != nil {
		return nil, databaseError(fmt.Errorf("GetStatsFeeRows query error: %w", err))
	}

	items, err := collectRows(rows, func(row pgx.CollectableRow) (database.KeysetFeeRow, error) {
		var item database.KeysetFeeRow
		err := row.Scan(&item.KeysetID, &item.Unit, &item.Quantity, &item.InputFeePpk)
		return item, err
	})
	if err != nil {
		return nil, databaseError(fmt.Errorf("GetStatsFeeRows collect error: %w", err))
	}
	return items, nil
}

func (pql Postgresql) GetStatsSnapshotsBySince(ctx context.Context, since int64) ([]database.StatsSnapshot, error) {
	rows, err := pql.pool.Query(ctx, `SELECT id, start_date, end_date, mint_summary, melt_summary, blind_sigs_summary, proofs_summary, fees
		FROM stats
		WHERE end_date >= $1
		ORDER BY end_date ASC, id ASC`, since)
	if err != nil {
		return nil, databaseError(fmt.Errorf("GetStatsSnapshotsBySince query error: %w", err))
	}
	defer rows.Close()

	snapshots := make([]database.StatsSnapshot, 0)
	for rows.Next() {
		var snapshot database.StatsSnapshot
		var mintSummary []byte
		var meltSummary []byte
		var blindSigsSummary []byte
		var proofsSummary []byte
		if err := rows.Scan(&snapshot.ID, &snapshot.StartDate, &snapshot.EndDate, &mintSummary, &meltSummary, &blindSigsSummary, &proofsSummary, &snapshot.Fees); err != nil {
			return nil, databaseError(fmt.Errorf("GetStatsSnapshotsBySince scan error: %w", err))
		}
		if err := json.Unmarshal(mintSummary, &snapshot.MintSummary); err != nil {
			return nil, databaseError(fmt.Errorf("GetStatsSnapshotsBySince unmarshal mint_summary: %w", err))
		}
		if err := json.Unmarshal(meltSummary, &snapshot.MeltSummary); err != nil {
			return nil, databaseError(fmt.Errorf("GetStatsSnapshotsBySince unmarshal melt_summary: %w", err))
		}
		if err := json.Unmarshal(blindSigsSummary, &snapshot.BlindSigsSummary); err != nil {
			return nil, databaseError(fmt.Errorf("GetStatsSnapshotsBySince unmarshal blind_sigs_summary: %w", err))
		}
		if err := json.Unmarshal(proofsSummary, &snapshot.ProofsSummary); err != nil {
			return nil, databaseError(fmt.Errorf("GetStatsSnapshotsBySince unmarshal proofs_summary: %w", err))
		}
		snapshot.MintSummary = normalizeStatsSummary(snapshot.MintSummary)
		snapshot.MeltSummary = normalizeStatsSummary(snapshot.MeltSummary)
		snapshot.BlindSigsSummary = normalizeStatsSummary(snapshot.BlindSigsSummary)
		snapshot.ProofsSummary = normalizeStatsSummary(snapshot.ProofsSummary)
		snapshots = append(snapshots, snapshot)
	}
	if err := rows.Err(); err != nil {
		return nil, databaseError(fmt.Errorf("GetStatsSnapshotsBySince rows error: %w", err))
	}
	return snapshots, nil
}

func (pql Postgresql) InsertStatsSnapshot(ctx context.Context, snapshot database.StatsSnapshot) error {
	mintSummary, err := json.Marshal(normalizeStatsSummary(snapshot.MintSummary))
	if err != nil {
		return databaseError(fmt.Errorf("marshal mint summary: %w", err))
	}
	meltSummary, err := json.Marshal(normalizeStatsSummary(snapshot.MeltSummary))
	if err != nil {
		return databaseError(fmt.Errorf("marshal melt summary: %w", err))
	}
	blindSigsSummary, err := json.Marshal(normalizeStatsSummary(snapshot.BlindSigsSummary))
	if err != nil {
		return databaseError(fmt.Errorf("marshal blind sigs summary: %w", err))
	}
	proofsSummary, err := json.Marshal(normalizeStatsSummary(snapshot.ProofsSummary))
	if err != nil {
		return databaseError(fmt.Errorf("marshal proofs summary: %w", err))
	}

	_, err = pql.pool.Exec(ctx, `INSERT INTO stats (start_date, end_date, mint_summary, melt_summary, blind_sigs_summary, proofs_summary, fees)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		snapshot.StartDate,
		snapshot.EndDate,
		mintSummary,
		meltSummary,
		blindSigsSummary,
		proofsSummary,
		snapshot.Fees,
	)
	if err != nil {
		return databaseError(fmt.Errorf("InsertStatsSnapshot exec error: %w", err))
	}

	return nil
}
