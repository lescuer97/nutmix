//nolint:exhaustruct
package admin

import (
	"strings"
	"testing"
	"time"

	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
)

func uint64Ptr(value uint64) *uint64 {
	return &value
}

func TestLightningActivityTableRendersOperationColumns(t *testing.T) {
	ctx, recorder := adminTestContext("/admin/ln-table?since=all")
	row := templates.LightningInvoiceVisual{
		Id:      "mint-123",
		Type:    "mint",
		Invoice: "lnbc123",
		Status:  "PAID",
		Unit:    "sat",
		Time:    200,
	}
	err := templates.LightningActivityTable([]templates.LightningInvoiceVisual{row}).Render(ctx.Request.Context(), recorder)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	body := recorder.Body.String()
	for _, want := range []string{"ID", "Type", "Invoice", "Status", "Unit", "Time", "mint-123", "lnbc123", "PAID"} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected body to contain %q, got %s", want, body)
		}
	}
	for _, unwanted := range []string{"Mint Count", "Net Flow", "Fees"} {
		if strings.Contains(body, unwanted) {
			t.Fatalf("did not expect body to contain %q, got %s", unwanted, body)
		}
	}
}

func TestLightningTableRendersRawMintAndMeltRowsSortedNewestFirst(t *testing.T) {
	db := testMockDB()
	now := time.Now().Unix()
	db.MintRequest = []cashu.MintRequestDB{{
		Quote:   "mint-new",
		Request: "lnbc-new",
		Unit:    "sat",
		State:   cashu.PAID,
		SeenAt:  now,
		Amount:  uint64Ptr(21),
	}}
	db.MeltRequest = []cashu.MeltRequestDB{{
		Quote:   "melt-mid",
		Request: "lnbc-mid",
		Unit:    "sat",
		State:   cashu.ISSUED,
		SeenAt:  now - 60,
		Amount:  10,
	}}
	ctx, recorder := adminTestContext("/admin/ln-table?since=all")
	LightningTable(&adminHandler{mint: adminTestMint(db)})(ctx)
	body := recorder.Body.String()
	for _, want := range []string{"mint-new", "melt-mid", "lnbc-new", "lnbc-mid", "pill-mint", "pill-melt"} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected body to contain %q, got %s", want, body)
		}
	}
	if strings.Index(body, "mint-new") > strings.Index(body, "melt-mid") {
		t.Fatalf("expected newest row first, got %s", body)
	}
}

func TestLightningTablePassesSinceUnixToMintAndMeltQueries(t *testing.T) {
	db := testMockDB()
	ctx, _ := adminTestContext("/admin/ln-table?since=all")
	LightningTable(&adminHandler{mint: adminTestMint(db)})(ctx)
	if db.LastMintSince != time.Unix(0, 0).Unix() {
		t.Fatalf("expected mint since %d, got %d", time.Unix(0, 0).Unix(), db.LastMintSince)
	}
	if db.LastMeltSince != time.Unix(0, 0).Unix() {
		t.Fatalf("expected melt since %d, got %d", time.Unix(0, 0).Unix(), db.LastMeltSince)
	}
	if db.LastLightningSearch != nil {
		t.Fatalf("expected no search query for date fetch, got %v", db.LastLightningSearch)
	}
}

func TestLightningTableFiltersByTimeWhenSearchEmpty(t *testing.T) {
	db := testMockDB()
	now := time.Now().Unix()
	db.MintRequest = []cashu.MintRequestDB{
		{
			Quote:   "mint-recent",
			Request: "lnbc-recent",
			Unit:    "sat",
			State:   cashu.PAID,
			SeenAt:  now - 24*60*60,
			Amount:  uint64Ptr(5),
		},
		{
			Quote:   "mint-old",
			Request: "lnbc-old",
			Unit:    "sat",
			State:   cashu.PAID,
			SeenAt:  now - 40*24*60*60,
			Amount:  uint64Ptr(7),
		},
	}
	ctx, recorder := adminTestContext("/admin/ln-table?since=1w")
	LightningTable(&adminHandler{mint: adminTestMint(db)})(ctx)
	body := recorder.Body.String()
	if !strings.Contains(body, "mint-recent") {
		t.Fatalf("expected recent row in body, got %s", body)
	}
	if strings.Contains(body, "mint-old") {
		t.Fatalf("did not expect old row in body, got %s", body)
	}
	if db.LastLightningSearch != nil {
		t.Fatalf("expected no search query, got %v", db.LastLightningSearch)
	}
}

func TestLightningTableSearchUsesSameTimeRangeAndMatchesQuoteOrRequest(t *testing.T) {
	db := testMockDB()
	now := time.Now().Unix()
	db.MintRequest = []cashu.MintRequestDB{
		{
			Quote:   "old-hit",
			Request: "lnbc-old-hit",
			Unit:    "sat",
			State:   cashu.PAID,
			SeenAt:  now - 40*24*60*60,
			Amount:  uint64Ptr(5),
		},
		{
			Quote:   "recent-miss",
			Request: "invoice-hit-recent",
			Unit:    "sat",
			State:   cashu.PAID,
			SeenAt:  now - 24*60*60,
			Amount:  uint64Ptr(7),
		},
		{
			Quote:   "recent-other",
			Request: "lnbc-recent-other",
			Unit:    "sat",
			State:   cashu.PAID,
			SeenAt:  now - 24*60*60,
			Amount:  uint64Ptr(9),
		},
	}
	db.MeltRequest = []cashu.MeltRequestDB{{
		Quote:   "MELT-hit-recent",
		Request: "lnbc-melt",
		Unit:    "sat",
		State:   cashu.ISSUED,
		SeenAt:  now - 2*24*60*60,
		Amount:  11,
	}}
	ctx, recorder := adminTestContext("/admin/ln-table?since=1w&search=HIT")
	LightningTable(&adminHandler{mint: adminTestMint(db)})(ctx)
	body := recorder.Body.String()
	for _, want := range []string{"invoice-hit-recent", "MELT-hit-recent"} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected matching recent search hit %q in body, got %s", want, body)
		}
	}
	for _, unwanted := range []string{"old-hit", "lnbc-old-hit", "recent-other", "lnbc-recent-other"} {
		if strings.Contains(body, unwanted) {
			t.Fatalf("did not expect %q in body, got %s", unwanted, body)
		}
	}
	if db.LastLightningSearch == nil || *db.LastLightningSearch != "HIT" {
		t.Fatalf("expected lightning search to be recorded, got %v", db.LastLightningSearch)
	}
	if db.LastMintSince != 0 || db.LastMeltSince != 0 {
		t.Fatalf("expected search path to avoid date query, got mint since=%d melt since=%d", db.LastMintSince, db.LastMeltSince)
	}
	if db.LastSearchSince < now-8*24*60*60 || db.LastSearchSince > now-6*24*60*60 {
		t.Fatalf("expected search since to be about one week ago, got %d", db.LastSearchSince)
	}
	if db.LastSearchLimit != lightningSearchLimit {
		t.Fatalf("expected search limit %d, got %d", lightningSearchLimit, db.LastSearchLimit)
	}
}

func TestLightningTableShortSearchFallsBackToDateQuery(t *testing.T) {
	db := testMockDB()
	now := time.Now().Unix()
	db.MintRequest = []cashu.MintRequestDB{{
		Quote:   "mint-recent",
		Request: "lnbc-recent",
		Unit:    "sat",
		State:   cashu.PAID,
		SeenAt:  now - 24*60*60,
		Amount:  uint64Ptr(5),
	}}
	ctx, recorder := adminTestContext("/admin/ln-table?since=1w&search=h")
	LightningTable(&adminHandler{mint: adminTestMint(db)})(ctx)
	body := recorder.Body.String()
	if !strings.Contains(body, "mint-recent") {
		t.Fatalf("expected fallback date query row in body, got %s", body)
	}
	if db.LastLightningSearch != nil {
		t.Fatalf("expected short search to skip search query, got %v", db.LastLightningSearch)
	}
	if db.LastMintSince == 0 || db.LastMeltSince == 0 {
		t.Fatalf("expected short search to use date queries, got mint=%d melt=%d", db.LastMintSince, db.LastMeltSince)
	}
}

func TestLightningTableReturnsErrorWhenRequestReadFails(t *testing.T) {
	db := testMockDB()
	db.ReturnError = 1
	ctx, _ := adminTestContext("/admin/ln-table?since=all")
	LightningTable(&adminHandler{mint: adminTestMint(db)})(ctx)
	if len(ctx.Errors) == 0 {
		t.Fatal("expected error when request read fails")
	}
}

func TestLightningPageRendersInvoiceSearchUI(t *testing.T) {
	ctx, recorder := adminTestContext("/admin/ln?since=all&search=abc")
	LnPage(adminTestMint(testMockDB()))(ctx)
	body := recorder.Body.String()
	for _, want := range []string{"ln-search-input", "Search by ID", "input changed delay:300ms from:#ln-search-input", "value=\"abc\""} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected body to contain %q, got %s", want, body)
		}
	}
}
