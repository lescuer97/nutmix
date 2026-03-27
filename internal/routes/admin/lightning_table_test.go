//nolint:exhaustruct
package admin

import (
	"strings"
	"testing"
	"time"

	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
)

func TestLightningSnapshotRowsAggregatesMintMeltAndFees(t *testing.T) {
	rowA := testStatsRow()
	rowA.StartDate = 100
	rowA.EndDate = 200
	rowA.MintSummary = []database.StatsSummaryItem{{Unit: "sat", Quantity: 2, Amount: 20}}
	rowA.MeltSummary = []database.StatsSummaryItem{{Unit: "sat", Quantity: 1, Amount: 5}}
	rowA.Fees = 3
	rowB := testStatsRow()
	rowB.StartDate = 201
	rowB.EndDate = 300
	rowB.MintSummary = []database.StatsSummaryItem{{Unit: "usd", Quantity: 1, Amount: 2}}
	rowB.MeltSummary = []database.StatsSummaryItem{{Unit: "usd", Quantity: 3, Amount: 6}}
	rowB.Fees = 7
	rows := []database.StatsSnapshot{rowA, rowB}
	got, err := lightningSnapshotRows(rows)
	if err != nil {
		t.Fatalf("lightningSnapshotRows: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 rows, got %#v", got)
	}
	if got[0].EndDate != 300 || got[0].MintCount != 1 || got[0].MintAmount != 2 || got[0].MeltCount != 3 || got[0].MeltAmount != 6 || got[0].NetFlow != 0 || got[0].Fees != 7 {
		t.Fatalf("unexpected first row: %#v", got[0])
	}
	if got[1].EndDate != 200 || got[1].MintCount != 2 || got[1].MintAmount != 20 || got[1].MeltCount != 1 || got[1].MeltAmount != 5 || got[1].NetFlow != 15 || got[1].Fees != 3 {
		t.Fatalf("unexpected second row: %#v", got[1])
	}
}

func TestLightningSnapshotRowsOrdersNewestEndDateFirst(t *testing.T) {
	rowA := testStatsRow()
	rowA.StartDate = 0
	rowA.EndDate = 10
	rowB := testStatsRow()
	rowB.StartDate = 0
	rowB.EndDate = 30
	rowC := testStatsRow()
	rowC.StartDate = 0
	rowC.EndDate = 20
	got, err := lightningSnapshotRows([]database.StatsSnapshot{rowA, rowB, rowC})
	if err != nil {
		t.Fatalf("lightningSnapshotRows: %v", err)
	}
	if got[0].EndDate != 30 || got[1].EndDate != 20 || got[2].EndDate != 10 {
		t.Fatalf("unexpected order: %#v", got)
	}
}

func TestLightningSnapshotRowsHandlesEmptySummaries(t *testing.T) {
	row := testStatsRow()
	row.StartDate = 0
	row.EndDate = 10
	got, err := lightningSnapshotRows([]database.StatsSnapshot{row})
	if err != nil {
		t.Fatalf("lightningSnapshotRows: %v", err)
	}
	if got[0].MintCount != 0 || got[0].MeltCount != 0 || got[0].Fees != 0 {
		t.Fatalf("expected zero row, got %#v", got[0])
	}
}

func TestLightningSnapshotRowsClampsNegativeNetFlowSafely(t *testing.T) {
	row := testStatsRow()
	row.StartDate = 0
	row.EndDate = 10
	row.MintSummary = []database.StatsSummaryItem{{Unit: "sat", Quantity: 1, Amount: 2}}
	row.MeltSummary = []database.StatsSummaryItem{{Unit: "sat", Quantity: 1, Amount: 6}}
	got, err := lightningSnapshotRows([]database.StatsSnapshot{row})
	if err != nil {
		t.Fatalf("lightningSnapshotRows: %v", err)
	}
	if got[0].NetFlow != 0 {
		t.Fatalf("expected clamped net flow 0, got %#v", got[0])
	}
}

func TestLightningSnapshotRowsReturnsErrorForInvalidWindowSemantics(t *testing.T) {
	row := testStatsRow()
	row.StartDate = 20
	row.EndDate = 10
	_, err := lightningSnapshotRows([]database.StatsSnapshot{row})
	if err == nil {
		t.Fatal("expected error for invalid window semantics")
	}
}

func TestLightningActivityTableRendersSnapshotColumns(t *testing.T) {
	ctx, recorder := adminTestContext("/admin/ln-table?since=all")
	row := templates.LightningSnapshotRow{ //nolint:exhaustruct
		StartDate:  100,
		EndDate:    200,
		MintCount:  2,
		MintAmount: 20,
		MeltCount:  1,
		MeltAmount: 5,
		NetFlow:    15,
		Fees:       3,
	}
	err := templates.LightningActivityTable([]templates.LightningSnapshotRow{row}).Render(ctx.Request.Context(), recorder)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	body := recorder.Body.String()
	for _, want := range []string{"Start", "End", "Mint Count", "Mint Amount", "Melt Count", "Melt Amount", "Net Flow", "Fees"} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected body to contain %q, got %s", want, body)
		}
	}
	for _, unwanted := range []string{"Invoice", "Status"} {
		if strings.Contains(body, unwanted) {
			t.Fatalf("did not expect body to contain %q, got %s", unwanted, body)
		}
	}
}

func TestLightningTableUsesStatsRowsOnly(t *testing.T) {
	db := testMockDB()
	row := testStatsRow()
	row.StartDate = 100
	row.EndDate = 200
	row.MintSummary = []database.StatsSummaryItem{{Unit: "sat", Quantity: 2, Amount: 20}}
	row.MeltSummary = []database.StatsSummaryItem{{Unit: "sat", Quantity: 1, Amount: 5}}
	row.Fees = 3
	db.Stats = []database.StatsSnapshot{row}
	ctx, recorder := adminTestContext("/admin/ln-table?since=all")
	LightningTable(&adminHandler{mint: adminTestMint(db)})(ctx)
	body := recorder.Body.String()
	for _, want := range []string{"Mint Count", ">20<", ">5<", ">15<", ">3<"} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected body to contain %q, got %s", want, body)
		}
	}
}

func TestLightningTablePassesSinceUnixToStatsQuery(t *testing.T) {
	db := testMockDB()
	db.Stats = []database.StatsSnapshot{}
	ctx, _ := adminTestContext("/admin/ln-table?since=all")
	LightningTable(&adminHandler{mint: adminTestMint(db)})(ctx)
	if db.LastStatsSince != time.Unix(0, 0).Unix() {
		t.Fatalf("expected since %d, got %d", time.Unix(0, 0).Unix(), db.LastStatsSince)
	}
}

func TestLightningTableIgnoresSearchParameter(t *testing.T) {
	db := testMockDB()
	row := testStatsRow()
	row.StartDate = 100
	row.EndDate = 200
	row.MintSummary = []database.StatsSummaryItem{{Unit: "sat", Quantity: 2, Amount: 20}}
	row.Fees = 1
	db.Stats = []database.StatsSnapshot{row}
	ctxA, recA := adminTestContext("/admin/ln-table?since=all")
	LightningTable(&adminHandler{mint: adminTestMint(db)})(ctxA)
	ctxB, recB := adminTestContext("/admin/ln-table?since=all&search=abc")
	LightningTable(&adminHandler{mint: adminTestMint(db)})(ctxB)
	if recA.Body.String() != recB.Body.String() {
		t.Fatalf("expected search to be ignored, got %q vs %q", recA.Body.String(), recB.Body.String())
	}
}

func TestLightningTableReturnsErrorWhenStatsReadFails(t *testing.T) {
	db := testMockDB()
	db.ReturnError = 1
	ctx, _ := adminTestContext("/admin/ln-table?since=all")
	LightningTable(&adminHandler{mint: adminTestMint(db)})(ctx)
	if len(ctx.Errors) == 0 {
		t.Fatal("expected error when stats read fails")
	}
}

func TestLightningTableReturnsErrorWhenStatsTransformationFails(t *testing.T) {
	db := testMockDB()
	row := testStatsRow()
	row.StartDate = 20
	row.EndDate = 10
	db.Stats = []database.StatsSnapshot{row}
	ctx, _ := adminTestContext("/admin/ln-table?since=all")
	LightningTable(&adminHandler{mint: adminTestMint(db)})(ctx)
	if len(ctx.Errors) == 0 {
		t.Fatal("expected error when stats transformation fails")
	}
}

func TestLightningPageDoesNotRenderInvoiceSearchUI(t *testing.T) {
	ctx, recorder := adminTestContext("/admin/ln?since=all")
	LnPage(adminTestMint(testMockDB()))(ctx)
	body := recorder.Body.String()
	for _, unwanted := range []string{"ln-search-input", "Search by ID", "input from:#ln-search-input"} {
		if strings.Contains(body, unwanted) {
			t.Fatalf("did not expect body to contain %q, got %s", unwanted, body)
		}
	}
}
