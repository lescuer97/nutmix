package mockdb

import (
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

func (m *MockDB) GetMintMeltBalanceByTime(time int64) (database.MintMeltBalance, error) {
	var mintmeltbalance database.MintMeltBalance

	for i := 0; i < len(m.MeltRequest); i++ {
		if m.MeltRequest[i].State == cashu.ISSUED || m.MeltRequest[i].State == cashu.PAID {
			mintmeltbalance.Melt = append(mintmeltbalance.Melt, m.MeltRequest[i])

		}

	}

	for j := 0; j < len(m.MeltRequest); j++ {
		if m.MintRequest[j].State == cashu.ISSUED || m.MintRequest[j].State == cashu.PAID {
			mintmeltbalance.Mint = append(mintmeltbalance.Mint, m.MintRequest[j])

		}

	}
	return mintmeltbalance, nil
}
func (m *MockDB) AddLiquiditySwap(tx pgx.Tx, swap utils.LiquiditySwap) error {
	m.LiquiditySwap = append(m.LiquiditySwap, swap)
	return nil

}
func (m *MockDB) ChangeLiquiditySwapState(tx pgx.Tx, id string, state utils.SwapState) error {
	var liquiditySwaps []utils.LiquiditySwap
	for i := 0; i < len(m.LiquiditySwap); i++ {
		if m.LiquiditySwap[i].Id == id {
			liquiditySwaps[i].State = state
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

func (m *MockDB) GetLiquiditySwapsByStates(states []utils.SwapState) ([]utils.LiquiditySwap, error) {
	var liquiditySwaps []utils.LiquiditySwap
	for i := 0; i < len(m.LiquiditySwap); i++ {
		if slices.Contains(states, m.LiquiditySwap[i].State) {
			liquiditySwaps = append(liquiditySwaps, m.LiquiditySwap[i])
		}

	}

	return liquiditySwaps, nil

}
