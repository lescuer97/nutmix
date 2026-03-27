//nolint:exhaustruct
package admin

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	mockdb "github.com/lescuer97/nutmix/internal/database/mock_db"
	"github.com/lescuer97/nutmix/internal/lightning"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
	"github.com/lescuer97/nutmix/internal/utils"
)

func TestSummarizeStatsRowsAggregatesFees(t *testing.T) {
	rowA := testStatsRow()
	rowA.Fees = 7
	rowB := testStatsRow()
	rowB.Fees = 5
	rows := []database.StatsSnapshot{rowA, rowB}
	if got := sumFeesFromStats(rows); got != 12 {
		t.Fatalf("expected fees 12, got %d", got)
	}
}

func TestSummarizeStatsRowsPreservesWalletAndSinceInputs(t *testing.T) {
	lnBalance := cashu.Amount{Unit: cashu.Sat, Amount: 42}
	rowA := testStatsRow()
	rowA.Fees = 7
	rowB := testStatsRow()
	rowB.Fees = 5
	summary := buildSummaryFromStats([]database.StatsSnapshot{rowA, rowB}, lnBalance, true, "Jan 2, 2026")
	if summary.LnBalance.Amount != 42 || !summary.FakeWallet || summary.SinceDate != "Jan 2, 2026" || summary.Fees != 12 {
		t.Fatalf("unexpected summary: %#v", summary)
	}
}

func TestBalanceFromStatsSnapshotsAggregatesAmountsAndCounts(t *testing.T) {
	rowA := testStatsRow()
	rowA.ProofsSummary = []database.StatsSummaryItem{{Unit: "sat", Quantity: 2, Amount: 7}}
	rowA.BlindSigsSummary = []database.StatsSummaryItem{{Unit: "sat", Quantity: 3, Amount: 10}}
	rowB := testStatsRow()
	rowB.ProofsSummary = []database.StatsSummaryItem{{Unit: "usd", Quantity: 1, Amount: 4}}
	rowB.BlindSigsSummary = []database.StatsSummaryItem{{Unit: "usd", Quantity: 2, Amount: 8}}
	rows := []database.StatsSnapshot{rowA, rowB}
	balance := balanceFromStatsSnapshots(rows)
	if balance.ProofsAmount != 11 || balance.ProofsQuantity != 3 || balance.BlindSigsAmount != 18 || balance.BlindSigsQuantity != 5 || balance.NeededBalance != 7 {
		t.Fatalf("unexpected balance: %#v", balance)
	}
	if math.Abs(balance.Ratio-61.1111111111) > 0.0001 {
		t.Fatalf("unexpected ratio: %f", balance.Ratio)
	}
}

func TestBalanceFromStatsSnapshotsReturnsZeroRatioWhenBlindSigsAmountIsZero(t *testing.T) {
	row := testStatsRow()
	row.ProofsSummary = []database.StatsSummaryItem{{Unit: "sat", Quantity: 2, Amount: 7}}
	balance := balanceFromStatsSnapshots([]database.StatsSnapshot{row})
	if balance.Ratio != 0 {
		t.Fatalf("expected zero ratio, got %f", balance.Ratio)
	}
}

func TestBalanceFromStatsSnapshotsClampsNeededBalanceAtZero(t *testing.T) {
	row := testStatsRow()
	row.ProofsSummary = []database.StatsSummaryItem{{Unit: "sat", Quantity: 2, Amount: 12}}
	row.BlindSigsSummary = []database.StatsSummaryItem{{Unit: "sat", Quantity: 1, Amount: 5}}
	balance := balanceFromStatsSnapshots([]database.StatsSnapshot{row})
	if balance.NeededBalance != 0 {
		t.Fatalf("expected clamped needed balance 0, got %d", balance.NeededBalance)
	}
}

func TestBalanceFromStatsSnapshotsReturnsZeroBalanceForEmptyInput(t *testing.T) {
	balance := balanceFromStatsSnapshots(nil)
	if balance != (templates.Balance{}) { //nolint:exhaustruct
		t.Fatalf("expected zero balance, got %#v", balance)
	}
}

func summaryTestMint(db *mockdb.MockDB) *mint.Mint {
	var m mint.Mint
	m.MintDB = db
	m.LightningBackend = lightning.FakeWallet{ //nolint:exhaustruct
		UnpurposeErrors: nil,
		Network:         chaincfg.RegressionNetParams,
		InvoiceFee:      0,
	}
	m.Config = utils.Config{ //nolint:exhaustruct
		MINT_LIGHTNING_BACKEND: utils.FAKE_WALLET,
	}
	return &m
}

func TestSummaryComponentUsesStatsRowsOnlyForFees(t *testing.T) {
	db := testMockDB()
	rowA := testStatsRow()
	rowA.EndDate = time.Now().Unix()
	rowA.Fees = 7
	rowB := testStatsRow()
	rowB.EndDate = time.Now().Unix()
	rowB.Fees = 5
	db.Stats = []database.StatsSnapshot{rowA, rowB}
	ctx, recorder := adminTestContext("/admin/summary?since=all")
	SummaryComponent(summaryTestMint(db), &adminHandler{mint: summaryTestMint(db)})(ctx)
	if !strings.Contains(recorder.Body.String(), "Fees") || !strings.Contains(recorder.Body.String(), ">12") {
		t.Fatalf("expected summary to render fees from stats, got %s", recorder.Body.String())
	}
}

func TestSummaryComponentFailsWithoutRawFallbackWhenStatsReadFails(t *testing.T) {
	db := testMockDB()
	db.ReturnError = 1
	ctx, _ := adminTestContext("/admin/summary?since=all")
	SummaryComponent(summaryTestMint(db), &adminHandler{mint: summaryTestMint(db)})(ctx)
	if len(ctx.Errors) == 0 {
		t.Fatal("expected summary error when stats read fails")
	}
}

func TestSummaryComponentFailsWithoutRawFallbackOnMalformedStatsData(t *testing.T) {
	db := testMockDB()
	db.ReturnError = 1
	ctx, _ := adminTestContext("/admin/summary?since=all")
	SummaryComponent(summaryTestMint(db), &adminHandler{mint: summaryTestMint(db)})(ctx)
	if len(ctx.Errors) == 0 {
		t.Fatal("expected summary error on malformed stats data")
	}
}

func TestChartHandlersReturnEmptyStateWhenStatsReadFailsBlindSigs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testMockDB()
	db.ReturnError = 1
	ctx, recorder := adminTestContext("/admin/blindsigs-chart?since=all")
	BlindSigsChartCard(summaryTestMint(db))(ctx)
	if !strings.Contains(recorder.Body.String(), "blindsigs-chart-card") {
		t.Fatalf("expected empty blind sig chart card on stats read failure, got %s", recorder.Body.String())
	}
}

func TestEcashBalanceUsesStatsRowsOnly(t *testing.T) {
	db := testMockDB()
	row := testStatsRow()
	row.EndDate = 3600
	row.ProofsSummary = []database.StatsSummaryItem{{Unit: "sat", Quantity: 2, Amount: 7}}
	row.BlindSigsSummary = []database.StatsSummaryItem{{Unit: "sat", Quantity: 3, Amount: 10}}
	db.Stats = []database.StatsSnapshot{row}
	handler := adminHandler{mint: summaryTestMint(db)}
	balance, err := handler.EcashBalance(time.Unix(3600, 0))
	if err != nil {
		t.Fatalf("EcashBalance: %v", err)
	}
	if balance.ProofsAmount != 7 || balance.BlindSigsAmount != 10 || balance.NeededBalance != 3 {
		t.Fatalf("unexpected balance: %#v", balance)
	}
}

func TestEcashBalancePassesSinceUnixToStatsQuery(t *testing.T) {
	db := testMockDB()
	db.Stats = []database.StatsSnapshot{}
	handler := adminHandler{mint: summaryTestMint(db)}
	since := time.Unix(12345, 0)
	_, err := handler.EcashBalance(since)
	if err != nil {
		t.Fatalf("EcashBalance: %v", err)
	}
	if db.LastStatsSince != since.Unix() {
		t.Fatalf("expected since %d, got %d", since.Unix(), db.LastStatsSince)
	}
}

func TestEcashBalanceReturnsClampedNeededBalanceFromStats(t *testing.T) {
	db := testMockDB()
	row := testStatsRow()
	row.ProofsSummary = []database.StatsSummaryItem{{Unit: "sat", Quantity: 2, Amount: 12}}
	row.BlindSigsSummary = []database.StatsSummaryItem{{Unit: "sat", Quantity: 1, Amount: 5}}
	db.Stats = []database.StatsSnapshot{row}
	handler := adminHandler{mint: summaryTestMint(db)}
	balance, err := handler.EcashBalance(time.Unix(0, 0))
	if err != nil {
		t.Fatalf("EcashBalance: %v", err)
	}
	if balance.NeededBalance != 0 {
		t.Fatalf("expected clamped needed balance 0, got %d", balance.NeededBalance)
	}
}

func TestEcashBalanceReturnsErrorWhenStatsReadFails(t *testing.T) {
	db := testMockDB()
	db.ReturnError = 1
	handler := adminHandler{mint: summaryTestMint(db)}
	_, err := handler.EcashBalance(time.Unix(0, 0))
	if err == nil {
		t.Fatal("expected error")
	}
}
