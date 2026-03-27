package mockdb

import (
	"context"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/utils"
)

func (m *MockDB) SaveNostrAuth(auth database.NostrLoginAuth) error {
	return nil

}

func (m *MockDB) UpdateNostrAuthActivation(tx pgx.Tx, nonce string, activated bool) error {
	return nil
}

func (m *MockDB) GetNostrAuth(tx pgx.Tx, nonce string) (database.NostrLoginAuth, error) {
	var seeds []database.NostrLoginAuth
	for i := 0; i < len(m.NostrAuth); i++ {

		if m.Seeds[i].Unit == nonce {
			seeds = append(seeds, m.NostrAuth[i])

		}

	}
	return seeds[0], nil

}

func (m *MockDB) AddLiquiditySwap(tx pgx.Tx, swap utils.LiquiditySwap) error {
	m.LiquiditySwap = append(m.LiquiditySwap, swap)
	return nil

}
func (m *MockDB) ChangeLiquiditySwapState(tx pgx.Tx, id string, state utils.SwapState) error {
	for i := 0; i < len(m.LiquiditySwap); i++ {
		if m.LiquiditySwap[i].Id == id {
			m.LiquiditySwap[i].State = state
		}

	}

	return nil
}

func (m *MockDB) GetLiquiditySwapById(tx pgx.Tx, id string) (utils.LiquiditySwap, error) {
	var liquiditySwaps []utils.LiquiditySwap
	for i := 0; i < len(m.LiquiditySwap); i++ {

		if m.LiquiditySwap[i].Id == id {
			liquiditySwaps = append(liquiditySwaps, m.LiquiditySwap[i])

		}

	}

	return liquiditySwaps[0], nil
}

func (m *MockDB) GetAllLiquiditySwaps() ([]utils.LiquiditySwap, error) {
	return m.LiquiditySwap, nil
}

func (m *MockDB) GetLiquiditySwapsByStates(tx pgx.Tx, states []utils.SwapState) ([]string, error) {
	liquiditySwaps := make([]string, 0)
	for i := 0; i < len(m.LiquiditySwap); i++ {
		if slices.Contains(states, m.LiquiditySwap[i].State) {
			liquiditySwaps = append(liquiditySwaps, m.LiquiditySwap[i].Id)
		}

	}

	return liquiditySwaps, nil

}

func (m *MockDB) GetLatestStatsSnapshot(ctx context.Context, tx pgx.Tx) (*database.StatsSnapshot, error) {
	if m.ReturnError != 0 {
		return nil, database.ErrDB
	}
	if len(m.Stats) == 0 {
		return nil, nil
	}
	latest := m.Stats[0]
	for i := 1; i < len(m.Stats); i++ {
		candidate := m.Stats[i]
		if candidate.EndDate > latest.EndDate || (candidate.EndDate == latest.EndDate && candidate.ID > latest.ID) {
			latest = candidate
		}
	}
	return &latest, nil
}

func (m *MockDB) GetMintStatsRows(ctx context.Context, tx pgx.Tx, startDate, endDate int64) ([]database.MintStatsRow, error) {
	rows := make([]database.MintStatsRow, 0)
	for _, request := range m.MintRequest {
		if request.SeenAt >= startDate && request.SeenAt <= endDate && (request.State == cashu.PAID || request.State == cashu.ISSUED) {
			rows = append(rows, database.MintStatsRow{
				Quote:   request.Quote,
				Unit:    request.Unit,
				Amount:  request.Amount,
				Request: request.Request,
			})
		}
	}
	return rows, nil
}

func (m *MockDB) GetMeltStatsRows(ctx context.Context, tx pgx.Tx, startDate, endDate int64) ([]database.MeltStatsRow, error) {
	rows := make([]database.MeltStatsRow, 0)
	for _, request := range m.MeltRequest {
		if request.SeenAt >= startDate && request.SeenAt <= endDate && (request.State == cashu.PAID || request.State == cashu.ISSUED) {
			rows = append(rows, database.MeltStatsRow{
				Quote:  request.Quote,
				Unit:   request.Unit,
				Amount: request.Amount,
			})
		}
	}
	return rows, nil
}

func (m *MockDB) GetProofStatsRows(ctx context.Context, tx pgx.Tx, startDate, endDate int64) ([]database.KeysetStatsRow, error) {
	rows := make([]database.KeysetStatsRow, 0)
	for _, proof := range m.Proofs {
		if proof.SeenAt < startDate || proof.SeenAt > endDate || proof.State != cashu.PROOF_SPENT {
			continue
		}
		unit := ""
		for _, seed := range m.Seeds {
			if seed.Id == proof.Id {
				unit = seed.Unit
				break
			}
		}
		rows = append(rows, database.KeysetStatsRow{KeysetID: proof.Id, Unit: unit, Amount: proof.Amount})
	}
	return rows, nil
}

func (m *MockDB) GetBlindSigStatsRows(ctx context.Context, tx pgx.Tx, startDate, endDate int64) ([]database.KeysetStatsRow, error) {
	rows := make([]database.KeysetStatsRow, 0)
	for _, sig := range m.RecoverSigDB {
		if sig.CreatedAt < startDate || sig.CreatedAt > endDate {
			continue
		}
		unit := ""
		for _, seed := range m.Seeds {
			if seed.Id == sig.Id {
				unit = seed.Unit
				break
			}
		}
		rows = append(rows, database.KeysetStatsRow{KeysetID: sig.Id, Unit: unit, Amount: sig.Amount})
	}
	return rows, nil
}

func (m *MockDB) GetStatsFeeRows(ctx context.Context, tx pgx.Tx, startDate, endDate int64) ([]database.KeysetFeeRow, error) {
	if m.ReturnError != 0 {
		return nil, database.ErrDB
	}
	counts := map[string]*database.KeysetFeeRow{}
	for _, proof := range m.Proofs {
		if proof.SeenAt < startDate || proof.SeenAt > endDate || proof.State != cashu.PROOF_SPENT {
			continue
		}
		row, ok := counts[proof.Id]
		if !ok {
			unit := ""
			inputFeePpk := uint64(0)
			for _, seed := range m.Seeds {
				if seed.Id == proof.Id {
					unit = seed.Unit
					inputFeePpk = uint64(seed.InputFeePpk)
					break
				}
			}
			row = &database.KeysetFeeRow{KeysetID: proof.Id, Unit: unit, Quantity: 0, InputFeePpk: inputFeePpk}
			counts[proof.Id] = row
		}
		row.Quantity++
	}
	rows := make([]database.KeysetFeeRow, 0, len(counts))
	for _, row := range counts {
		rows = append(rows, *row)
	}
	slices.SortFunc(rows, func(a, b database.KeysetFeeRow) int {
		if a.KeysetID < b.KeysetID {
			return -1
		}
		if a.KeysetID > b.KeysetID {
			return 1
		}
		return 0
	})
	return rows, nil
}

func (m *MockDB) GetStatsSnapshotsBySince(ctx context.Context, since int64) ([]database.StatsSnapshot, error) {
	m.LastStatsSince = since
	if m.ReturnError != 0 {
		return nil, database.ErrDB
	}
	rows := make([]database.StatsSnapshot, 0)
	for _, snapshot := range m.Stats {
		if snapshot.EndDate >= since {
			rows = append(rows, snapshot)
		}
	}
	slices.SortFunc(rows, func(a, b database.StatsSnapshot) int {
		if a.EndDate < b.EndDate {
			return -1
		}
		if a.EndDate > b.EndDate {
			return 1
		}
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
	return rows, nil
}

func (m *MockDB) InsertStatsSnapshot(ctx context.Context, snapshot database.StatsSnapshot) error {
	snapshot.ID = int64(len(m.Stats) + 1)
	m.Stats = append(m.Stats, snapshot)
	return nil
}
