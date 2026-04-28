package stats

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
)

type Store interface {
	GetTx(ctx context.Context) (pgx.Tx, error)
	Rollback(ctx context.Context, tx pgx.Tx) error
	GetLatestStatsSnapshot(ctx context.Context) (*database.StatsSnapshot, error)
	GetMintStatsRows(ctx context.Context, tx pgx.Tx, startDate, endDate int64) ([]database.MintStatsRow, error)
	GetMeltStatsRows(ctx context.Context, tx pgx.Tx, startDate, endDate int64) ([]database.MeltStatsRow, error)
	GetProofStatsRows(ctx context.Context, tx pgx.Tx, startDate, endDate int64) ([]database.KeysetStatsRow, error)
	GetBlindSigStatsRows(ctx context.Context, tx pgx.Tx, startDate, endDate int64) ([]database.KeysetStatsRow, error)
	GetStatsFeeRows(ctx context.Context, tx pgx.Tx, startDate, endDate int64) ([]database.KeysetFeeRow, error)
	GetStatsSnapshotsBySince(ctx context.Context, since int64) ([]database.StatsSnapshot, error)
	InsertStatsSnapshot(ctx context.Context, snapshot database.StatsSnapshot) error
}

type SnapshotOutcome string

const (
	SnapshotInserted SnapshotOutcome = "inserted"
	SnapshotSkipped  SnapshotOutcome = "skipped"
)

type SnapshotResult struct {
	Outcome   SnapshotOutcome
	StartDate int64
	EndDate   int64
}

type ticker interface {
	C() <-chan time.Time
	Stop()
}

type realTicker struct {
	ticker *time.Ticker
}

func (t realTicker) C() <-chan time.Time { return t.ticker.C }
func (t realTicker) Stop()               { t.ticker.Stop() }

type Service struct {
	DB               Store
	Now              func() time.Time
	DecodeMintAmount func(request string) (uint64, error)
	Logger           *slog.Logger
	NewTicker        func(interval time.Duration) ticker
	runSnapshot      func(ctx context.Context) (SnapshotResult, error)
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s Service) logger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

func (s Service) newTicker(interval time.Duration) ticker {
	if s.NewTicker != nil {
		return s.NewTicker(interval)
	}
	return realTicker{ticker: time.NewTicker(interval)}
}

func (s Service) snapshotRunner(ctx context.Context) (SnapshotResult, error) {
	if s.runSnapshot != nil {
		return s.runSnapshot(ctx)
	}
	return s.CreateSnapshot(ctx)
}

func sortSummary(items []database.StatsSummaryItem) {
	sort.Slice(items, func(i, j int) bool {
		return items[i].Unit < items[j].Unit
	})
}

func aggregateSummary(items map[string]*database.StatsSummaryItem) []database.StatsSummaryItem {
	if len(items) == 0 {
		return []database.StatsSummaryItem{}
	}
	result := make([]database.StatsSummaryItem, 0, len(items))
	for _, item := range items {
		result = append(result, *item)
	}
	sortSummary(result)
	return result
}

func addSummaryItem(items map[string]*database.StatsSummaryItem, unit string, amount uint64) {
	if item, ok := items[unit]; ok {
		item.Quantity++
		item.Amount += amount
		return
	}
	items[unit] = &database.StatsSummaryItem{Unit: unit, Quantity: 1, Amount: amount}
}

func calculateSnapshotFees(rows []database.KeysetFeeRow) uint64 {
	total := uint64(0)
	for _, row := range rows {
		if row.Unit == cashu.AUTH.String() {
			continue
		}
		total += row.Quantity * row.InputFeePpk
	}
	return (total + 999) / 1000
}

func (s Service) CreateSnapshot(ctx context.Context) (SnapshotResult, error) {
	result := SnapshotResult{
		Outcome:   "",
		StartDate: 0,
		EndDate:   0,
	}
	if s.DB == nil {
		return result, fmt.Errorf("stats store is nil")
	}
	if s.DecodeMintAmount == nil {
		return result, fmt.Errorf("mint amount decoder is nil")
	}

	latest, err := s.DB.GetLatestStatsSnapshot(ctx)
	if err != nil {
		return result, fmt.Errorf("GetLatestStatsSnapshot: %w", err)
	}
	if latest == nil {
		result.StartDate = 0
	} else {
		result.StartDate = latest.EndDate + 1
	}
	result.EndDate = s.now().Unix() - 10
	if result.EndDate < result.StartDate {
		result.Outcome = SnapshotSkipped
		return result, nil
	}

	mintRows, meltRows, proofRows, blindSigRows, feeRows, err := func() ([]database.MintStatsRow, []database.MeltStatsRow, []database.KeysetStatsRow, []database.KeysetStatsRow, []database.KeysetFeeRow, error) {
		tx, err := s.DB.GetTx(ctx)
		if err != nil {
			return nil, nil, nil, nil, nil, fmt.Errorf("GetTx: %w", err)
		}
		defer func() {
			_ = s.DB.Rollback(ctx, tx)
		}()

		mintRows, err := s.DB.GetMintStatsRows(ctx, tx, result.StartDate, result.EndDate)
		if err != nil {
			return nil, nil, nil, nil, nil, fmt.Errorf("GetMintStatsRows: %w", err)
		}
		meltRows, err := s.DB.GetMeltStatsRows(ctx, tx, result.StartDate, result.EndDate)
		if err != nil {
			return nil, nil, nil, nil, nil, fmt.Errorf("GetMeltStatsRows: %w", err)
		}
		proofRows, err := s.DB.GetProofStatsRows(ctx, tx, result.StartDate, result.EndDate)
		if err != nil {
			return nil, nil, nil, nil, nil, fmt.Errorf("GetProofStatsRows: %w", err)
		}
		blindSigRows, err := s.DB.GetBlindSigStatsRows(ctx, tx, result.StartDate, result.EndDate)
		if err != nil {
			return nil, nil, nil, nil, nil, fmt.Errorf("GetBlindSigStatsRows: %w", err)
		}
		feeRows, err := s.DB.GetStatsFeeRows(ctx, tx, result.StartDate, result.EndDate)
		if err != nil {
			return nil, nil, nil, nil, nil, fmt.Errorf("GetStatsFeeRows: %w", err)
		}

		return mintRows, meltRows, proofRows, blindSigRows, feeRows, nil
	}()
	if err != nil {
		return result, err
	}

	mintSummaryMap := make(map[string]*database.StatsSummaryItem)
	for _, row := range mintRows {
		var amount uint64
		if row.Amount != nil {
			amount = *row.Amount
		} else {
			if row.Unit != "sat" {
				return result, fmt.Errorf("mint quote %s missing amount for unit %s", row.Quote, row.Unit)
			}
			amount, err = s.DecodeMintAmount(row.Request)
			if err != nil {
				return result, fmt.Errorf("decode mint quote %s: %w", row.Quote, err)
			}
		}
		addSummaryItem(mintSummaryMap, row.Unit, amount)
	}

	meltSummaryMap := make(map[string]*database.StatsSummaryItem)
	for _, row := range meltRows {
		addSummaryItem(meltSummaryMap, row.Unit, row.Amount)
	}

	proofSummaryMap := make(map[string]*database.StatsSummaryItem)
	for _, row := range proofRows {
		if row.Unit == "" {
			return result, fmt.Errorf("proof keyset_id %s has no resolved unit", row.KeysetID)
		}
		addSummaryItem(proofSummaryMap, row.Unit, row.Amount)
	}

	blindSigSummaryMap := make(map[string]*database.StatsSummaryItem)
	for _, row := range blindSigRows {
		if row.Unit == "" {
			return result, fmt.Errorf("blind signature keyset_id %s has no resolved unit", row.KeysetID)
		}
		addSummaryItem(blindSigSummaryMap, row.Unit, row.Amount)
	}

	snapshot := database.StatsSnapshot{
		ID:               0,
		StartDate:        result.StartDate,
		EndDate:          result.EndDate,
		MintSummary:      aggregateSummary(mintSummaryMap),
		MeltSummary:      aggregateSummary(meltSummaryMap),
		BlindSigsSummary: aggregateSummary(blindSigSummaryMap),
		ProofsSummary:    aggregateSummary(proofSummaryMap),
		Fees:             calculateSnapshotFees(feeRows),
	}

	if len(snapshot.MintSummary) == 0 && len(snapshot.MeltSummary) == 0 && len(snapshot.BlindSigsSummary) == 0 && len(snapshot.ProofsSummary) == 0 {
		result.Outcome = SnapshotSkipped
		return result, nil
	}

	if err := s.DB.InsertStatsSnapshot(ctx, snapshot); err != nil {
		return result, fmt.Errorf("InsertStatsSnapshot: %w", err)
	}

	result.Outcome = SnapshotInserted
	return result, nil
}

func (s Service) logResult(result SnapshotResult, err error) {
	logger := s.logger()
	attrs := []any{"start_date", result.StartDate, "end_date", result.EndDate}
	if err != nil {
		logger.Error("stats snapshot failed", append(attrs, "error", err)...)
		return
	}
	if result.Outcome == SnapshotSkipped {
		logger.Info("stats snapshot skipped: zero new activity", attrs...)
		return
	}
	logger.Info("stats snapshot inserted", attrs...)
}

func (s Service) Run(ctx context.Context, interval time.Duration) {
	result, err := s.snapshotRunner(ctx)
	s.logResult(result, err)

	t := s.newTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C():
			result, err := s.snapshotRunner(ctx)
			s.logResult(result, err)
		}
	}
}
