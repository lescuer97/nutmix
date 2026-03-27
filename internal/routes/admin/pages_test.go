//nolint:exhaustruct
package admin

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/internal/database"
	mockdb "github.com/lescuer97/nutmix/internal/database/mock_db"
	"github.com/lescuer97/nutmix/internal/lightning"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/utils"
)

func adminTestMint(db *mockdb.MockDB) *mint.Mint {
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

func testStatsRow() database.StatsSnapshot {
	return database.StatsSnapshot{} //nolint:exhaustruct
}

func testMockDB() *mockdb.MockDB { return new(mockdb.MockDB) }

func adminTestContext(path string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("GET", path, nil)
	return ctx, recorder
}

func TestBuildLnChartFromStatsRowsBucketsByEndDate(t *testing.T) {
	rowA := testStatsRow()
	rowA.EndDate = 3600
	rowA.MintSummary = []database.StatsSummaryItem{{Unit: "sat", Quantity: 2, Amount: 20}}
	rowA.MeltSummary = []database.StatsSummaryItem{{Unit: "sat", Quantity: 1, Amount: 5}}
	rowB := testStatsRow()
	rowB.EndDate = 3610
	rowB.MintSummary = []database.StatsSummaryItem{{Unit: "usd", Quantity: 1, Amount: 2}}
	rowB.MeltSummary = []database.StatsSummaryItem{{Unit: "usd", Quantity: 3, Amount: 6}}
	rows := []database.StatsSnapshot{rowA, rowB}
	data := buildMintMeltTimeSeriesFromStats(rows, 60)
	if len(data) != 1 {
		t.Fatalf("expected one bucket, got %#v", data)
	}
	if data[0].Timestamp != 3600 || data[0].MintAmount != 22 || data[0].MeltAmount != 11 || data[0].MintCount != 3 || data[0].MeltCount != 4 {
		t.Fatalf("unexpected bucket: %#v", data[0])
	}
}

func TestBuildProofsChartFromStatsRowsBucketsByEndDate(t *testing.T) {
	rowA := testStatsRow()
	rowA.EndDate = 3600
	rowA.ProofsSummary = []database.StatsSummaryItem{{Unit: "sat", Quantity: 2, Amount: 9}}
	rowB := testStatsRow()
	rowB.EndDate = 3610
	rowB.ProofsSummary = []database.StatsSummaryItem{{Unit: "usd", Quantity: 1, Amount: 4}}
	rows := []database.StatsSnapshot{rowA, rowB}
	data := buildProofTimeSeriesFromStats(rows, func(row database.StatsSnapshot) []database.StatsSummaryItem { return row.ProofsSummary }, 60)
	if len(data) != 1 || data[0].Timestamp != 3600 || data[0].TotalAmount != 13 || data[0].Count != 3 {
		t.Fatalf("unexpected proof data: %#v", data)
	}
}

func TestBuildBlindSigsChartFromStatsRowsBucketsByEndDate(t *testing.T) {
	rowA := testStatsRow()
	rowA.EndDate = 3600
	rowA.BlindSigsSummary = []database.StatsSummaryItem{{Unit: "sat", Quantity: 4, Amount: 12}}
	rowB := testStatsRow()
	rowB.EndDate = 7200
	rowB.BlindSigsSummary = []database.StatsSummaryItem{{Unit: "usd", Quantity: 1, Amount: 5}}
	rows := []database.StatsSnapshot{rowA, rowB}
	data := buildProofTimeSeriesFromStats(rows, func(row database.StatsSnapshot) []database.StatsSummaryItem { return row.BlindSigsSummary }, 60)
	if len(data) != 2 || data[0].TotalAmount != 12 || data[1].TotalAmount != 5 {
		t.Fatalf("unexpected blind sig data: %#v", data)
	}
}

func TestLnChartCardUsesStatsRowsOnly(t *testing.T) {
	db := testMockDB()
	row := testStatsRow()
	row.EndDate = 3600
	row.MintSummary = []database.StatsSummaryItem{{Unit: "sat", Quantity: 2, Amount: 20}}
	row.MeltSummary = []database.StatsSummaryItem{{Unit: "sat", Quantity: 1, Amount: 5}}
	db.Stats = []database.StatsSnapshot{row}
	ctx, recorder := adminTestContext("/admin/ln-chart?since=all")
	LnChartCard(adminTestMint(db))(ctx)
	if !strings.Contains(recorder.Body.String(), "Inflows (Mint)") || !strings.Contains(recorder.Body.String(), ">20 ") || !strings.Contains(recorder.Body.String(), ">5 ") {
		t.Fatalf("expected stats-based LN chart output, got %s", recorder.Body.String())
	}
}

func TestProofsChartCardUsesStatsRowsOnly(t *testing.T) {
	db := testMockDB()
	row := testStatsRow()
	row.EndDate = 3600
	row.ProofsSummary = []database.StatsSummaryItem{{Unit: "sat", Quantity: 3, Amount: 9}}
	db.Stats = []database.StatsSnapshot{row}
	ctx, recorder := adminTestContext("/admin/proofs-chart?since=all")
	ProofsChartCard(adminTestMint(db))(ctx)
	if !strings.Contains(recorder.Body.String(), "Proofs Count") || !strings.Contains(recorder.Body.String(), ">9 ") {
		t.Fatalf("expected stats-based proofs chart output, got %s", recorder.Body.String())
	}
}

func TestBlindSigsChartCardUsesStatsRowsOnly(t *testing.T) {
	db := testMockDB()
	row := testStatsRow()
	row.EndDate = 3600
	row.BlindSigsSummary = []database.StatsSummaryItem{{Unit: "sat", Quantity: 4, Amount: 12}}
	db.Stats = []database.StatsSnapshot{row}
	ctx, recorder := adminTestContext("/admin/blindsigs-chart?since=all")
	BlindSigsChartCard(adminTestMint(db))(ctx)
	if !strings.Contains(recorder.Body.String(), "Blind Signatures") || !strings.Contains(recorder.Body.String(), ">12 ") {
		t.Fatalf("expected stats-based blind sig chart output, got %s", recorder.Body.String())
	}
}

func TestChartHandlersReturnEmptyStateWhenStatsReadFails(t *testing.T) {
	db := testMockDB()
	db.ReturnError = 1
	ctx, recorder := adminTestContext("/admin/proofs-chart?since=all")
	ProofsChartCard(adminTestMint(db))(ctx)
	if !strings.Contains(recorder.Body.String(), "proofs-chart-card") {
		t.Fatalf("expected empty chart card on stats read failure, got %s", recorder.Body.String())
	}
}

func TestChartHandlersFailCleanlyOnMalformedStatsData(t *testing.T) {
	db := testMockDB()
	db.ReturnError = 1
	ctx, recorder := adminTestContext("/admin/api/proofs-chart-data?since=all")
	ProofsChartDataAPI(adminTestMint(db))(ctx)
	if !strings.Contains(recorder.Body.String(), "proofsChart") {
		t.Fatalf("expected chart content on malformed stats failure, got %s", recorder.Body.String())
	}
}
