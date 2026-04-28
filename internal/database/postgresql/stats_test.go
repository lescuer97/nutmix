//nolint:exhaustruct,contextcheck
package postgresql

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupTestDatabase(t *testing.T, ctx context.Context) Postgresql {
	t.Helper()
	container, err := postgres.Run(ctx, "postgres:16.2",
		postgres.WithDatabase("postgres"),
		postgres.WithUsername("user"),
		postgres.WithPassword("password"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(15*time.Second)),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			t.Fatalf("Terminate: %v", err)
		}
	})

	connURI, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("ConnectionString: %v", err)
	}
	t.Setenv("DATABASE_URL", connURI)
	db, err := DatabaseSetup(ctx, "migrations")
	if err != nil {
		t.Fatalf("DatabaseSetup: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func saveSeed(t *testing.T, ctx context.Context, db Postgresql, id, unit string) {
	t.Helper()
	saveSeedWithFee(t, ctx, db, id, unit, 0)
}

func saveSeedWithFee(t *testing.T, ctx context.Context, db Postgresql, id, unit string, inputFeePpk uint) {
	t.Helper()
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatalf("GetTx: %v", err)
	}
	defer func() { _ = db.Rollback(ctx, tx) }()
	//nolint:contextcheck,exhaustruct // helper inserts minimal seed fixture shape
	if err := db.SaveNewSeed(tx, cashu.Seed{Active: true, CreatedAt: 1, Unit: unit, Id: id, Version: 1, InputFeePpk: inputFeePpk, DerivationPath: "m/0", Amounts: []uint64{1}, Legacy: false}); err != nil {
		t.Fatalf("SaveNewSeed: %v", err)
	}
	if err := db.Commit(ctx, tx); err != nil {
		t.Fatalf("Commit: %v", err)
	}
}

func TestStatsTableMigrationCreatesSchemaOnly(t *testing.T) {
	ctx := context.Background()
	db := setupTestDatabase(t, ctx)

	var count int
	if err := db.pool.QueryRow(ctx, `SELECT COUNT(*) FROM stats`).Scan(&count); err != nil {
		t.Fatalf("SELECT COUNT(*) FROM stats: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected empty stats table, got %d rows", count)
	}

	type columnInfo struct {
		Name       string `db:"column_name"`
		DataType   string `db:"data_type"`
		UDTName    string `db:"udt_name"`
		IsNullable string `db:"is_nullable"`
	}

	rows, err := db.pool.Query(ctx, `SELECT column_name, data_type, udt_name, is_nullable
		FROM information_schema.columns
		WHERE table_name = 'stats'`)
	if err != nil {
		t.Fatalf("columns query: %v", err)
	}
	defer rows.Close()
	cols, err := pgx.CollectRows(rows, pgx.RowToStructByName[columnInfo])
	if err != nil {
		t.Fatalf("CollectRows: %v", err)
	}
	seen := map[string]columnInfo{}
	for _, col := range cols {
		seen[col.Name] = col
	}
	for _, name := range []string{"id", "start_date", "end_date", "mint_summary", "melt_summary", "blind_sigs_summary", "proofs_summary", "fees"} {
		if _, ok := seen[name]; !ok {
			t.Fatalf("missing column %s", name)
		}
	}
	for _, name := range []string{"start_date", "end_date", "mint_summary", "melt_summary", "blind_sigs_summary", "proofs_summary", "fees"} {
		if seen[name].IsNullable != "NO" {
			t.Fatalf("expected %s to be NOT NULL", name)
		}
	}
	for _, name := range []string{"mint_summary", "melt_summary", "blind_sigs_summary", "proofs_summary"} {
		if seen[name].UDTName != "jsonb" {
			t.Fatalf("expected %s jsonb, got %s", name, seen[name].UDTName)
		}
	}
	var pkCount int
	if err := db.pool.QueryRow(ctx, `SELECT COUNT(*)
		FROM pg_index i
		JOIN pg_class c ON c.oid = i.indrelid
		WHERE c.relname = 'stats' AND i.indisprimary`).Scan(&pkCount); err != nil {
		t.Fatalf("pk query: %v", err)
	}
	if pkCount != 1 {
		t.Fatalf("expected primary key, got %d", pkCount)
	}
	var defaultExpr string
	if err := db.pool.QueryRow(ctx, `SELECT column_default FROM information_schema.columns WHERE table_name='stats' AND column_name='id'`).Scan(&defaultExpr); err != nil {
		t.Fatalf("id default query: %v", err)
	}
	if defaultExpr == "" {
		t.Fatal("expected id to have auto increment default")
	}
}

func TestStatsTableMigrationCreatesFeesColumn(t *testing.T) {
	ctx := context.Background()
	db := setupTestDatabase(t, ctx)
	var dataType string
	var nullable string
	if err := db.pool.QueryRow(ctx, `SELECT data_type, is_nullable FROM information_schema.columns WHERE table_name='stats' AND column_name='fees'`).Scan(&dataType, &nullable); err != nil {
		t.Fatalf("fees column query: %v", err)
	}
	if dataType != "bigint" {
		t.Fatalf("expected bigint fees column, got %s", dataType)
	}
	if nullable != "NO" {
		t.Fatalf("expected fees to be NOT NULL, got %s", nullable)
	}
}

func TestGetLatestStatsSnapshotReturnsNilWhenEmpty(t *testing.T) {
	ctx := context.Background()
	db := setupTestDatabase(t, ctx)
	snapshot, err := db.GetLatestStatsSnapshot(ctx)
	if err != nil {
		t.Fatalf("GetLatestStatsSnapshot: %v", err)
	}
	if snapshot != nil {
		t.Fatalf("expected nil snapshot, got %#v", snapshot)
	}
}

func TestGetLatestStatsSnapshotUsesGreatestEndDateThenID(t *testing.T) {
	ctx := context.Background()
	db := setupTestDatabase(t, ctx)
	if _, err := db.pool.Exec(ctx, `INSERT INTO stats (start_date, end_date, mint_summary, melt_summary, blind_sigs_summary, proofs_summary, fees) VALUES
		(0, 10, '[]', '[]', '[]', '[]', 0),
		(11, 20, '[]', '[]', '[]', '[]', 0),
		(21, 20, '[{"unit":"sat","quantity":1,"amount":2}]', '[]', '[]', '[]', 3)`); err != nil {
		t.Fatalf("insert stats: %v", err)
	}
	snapshot, err := db.GetLatestStatsSnapshot(ctx)
	if err != nil {
		t.Fatalf("GetLatestStatsSnapshot: %v", err)
	}
	if snapshot.StartDate != 21 {
		t.Fatalf("expected latest row by end_date/id tie break, got %#v", snapshot)
	}
}

func TestGetLatestStatsSnapshotSeesLatestCommittedRow(t *testing.T) {
	ctx := context.Background()
	db := setupTestDatabase(t, ctx)
	first, err := db.GetLatestStatsSnapshot(ctx)
	if err != nil {
		t.Fatalf("GetLatestStatsSnapshot first: %v", err)
	}
	if first != nil {
		t.Fatalf("expected nil first snapshot, got %#v", first)
	}
	if _, err := db.pool.Exec(ctx, `INSERT INTO stats (start_date, end_date, mint_summary, melt_summary, blind_sigs_summary, proofs_summary, fees) VALUES (0, 10, '[]', '[]', '[]', '[]', 0)`); err != nil {
		t.Fatalf("insert stats: %v", err)
	}
	second, err := db.GetLatestStatsSnapshot(ctx)
	if err != nil {
		t.Fatalf("GetLatestStatsSnapshot second: %v", err)
	}
	if second == nil || second.EndDate != 10 {
		t.Fatalf("expected latest snapshot, got %#v", second)
	}
}

func TestGetLatestStatsSnapshotRoundTripsJSONBSummaries(t *testing.T) {
	ctx := context.Background()
	db := setupTestDatabase(t, ctx)
	//nolint:exhaustruct // test only sets fields relevant to round-trip assertions
	expected := database.StatsSnapshot{
		StartDate:        0,
		EndDate:          10,
		MintSummary:      []database.StatsSummaryItem{{Unit: "sat", Quantity: 1, Amount: 5}},
		MeltSummary:      nil,
		BlindSigsSummary: []database.StatsSummaryItem{},
		ProofsSummary:    nil,
		Fees:             7,
	}
	if err := db.InsertStatsSnapshot(ctx, expected); err != nil {
		t.Fatalf("InsertStatsSnapshot: %v", err)
	}
	snapshot, err := db.GetLatestStatsSnapshot(ctx)
	if err != nil {
		t.Fatalf("GetLatestStatsSnapshot: %v", err)
	}
	if len(snapshot.MintSummary) != 1 || snapshot.MintSummary[0].Amount != 5 {
		t.Fatalf("unexpected mint summary: %#v", snapshot.MintSummary)
	}
	if snapshot.MeltSummary == nil || snapshot.ProofsSummary == nil {
		t.Fatal("expected empty summaries to round-trip as empty slices")
	}
	if snapshot.Fees != 7 {
		t.Fatalf("expected fees 7, got %d", snapshot.Fees)
	}
}

func TestGetMintStatsRowsFiltersPaidAndIssuedInInclusiveRange(t *testing.T) {
	ctx := context.Background()
	db := setupTestDatabase(t, ctx)
	if _, err := db.pool.Exec(ctx, `INSERT INTO mint_request (quote, request, expiry, unit, minted, state, seen_at, amount, checking_id, pubkey, description) VALUES
		('q-start', 'request', 100, 'sat', true, 'ISSUED', 10, 5, '', NULL, NULL),
		('q-end', 'request', 100, 'sat', true, 'PAID', 20, 7, '', NULL, NULL),
		('q-pending', 'request', 100, 'sat', false, 'PENDING', 15, 9, '', NULL, NULL)`); err != nil {
		t.Fatalf("insert mint_request: %v", err)
	}
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatalf("GetTx: %v", err)
	}
	defer func() { _ = db.Rollback(ctx, tx) }()
	rows, err := db.GetMintStatsRows(ctx, tx, 10, 20)
	if err != nil {
		t.Fatalf("GetMintStatsRows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %#v", rows)
	}
}

func TestGetMeltStatsRowsUsesInclusiveRangeAndStateFilter(t *testing.T) {
	ctx := context.Background()
	db := setupTestDatabase(t, ctx)
	if _, err := db.pool.Exec(ctx, `INSERT INTO melt_request (quote, request, fee_reserve, expiry, unit, amount, melted, state, payment_preimage, seen_at, mpp, fee_paid, checking_id) VALUES
		('m-start', 'request', 0, 100, 'sat', 5, true, 'ISSUED', '', 10, false, 0, ''),
		('m-end', 'request', 0, 100, 'sat', 7, true, 'PAID', '', 20, true, 0, ''),
		('m-unpaid', 'request', 0, 100, 'sat', 9, false, 'UNPAID', '', 15, false, 0, '')`); err != nil {
		t.Fatalf("insert melt_request: %v", err)
	}
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatalf("GetTx: %v", err)
	}
	defer func() { _ = db.Rollback(ctx, tx) }()
	rows, err := db.GetMeltStatsRows(ctx, tx, 10, 20)
	if err != nil {
		t.Fatalf("GetMeltStatsRows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %#v", rows)
	}
}

func TestGetProofStatsRowsUsesInclusiveRangeAndReturnsSpentProofsWithResolvedUnit(t *testing.T) {
	ctx := context.Background()
	db := setupTestDatabase(t, ctx)
	saveSeed(t, ctx, db, "proof-keyset", "sat")
	if _, err := db.pool.Exec(ctx, `INSERT INTO proofs (amount, id, secret, c, y, witness, seen_at, state, quote) VALUES
		(3, 'proof-keyset', 's1', decode('01','hex'), decode('02','hex'), '', 10, 'SPENT', NULL),
		(4, 'proof-keyset', 's2', decode('03','hex'), decode('04','hex'), '', 20, 'SPENT', NULL),
		(5, 'proof-keyset', 's3', decode('05','hex'), decode('06','hex'), '', 15, 'PENDING', NULL)`); err != nil {
		t.Fatalf("insert proofs: %v", err)
	}
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatalf("GetTx: %v", err)
	}
	defer func() { _ = db.Rollback(ctx, tx) }()
	rows, err := db.GetProofStatsRows(ctx, tx, 10, 20)
	if err != nil {
		t.Fatalf("GetProofStatsRows: %v", err)
	}
	if len(rows) != 2 || rows[0].Unit != "sat" {
		t.Fatalf("unexpected proof rows: %#v", rows)
	}
}

func TestGetBlindSigStatsRowsUsesInclusiveRangeAndReturnsResolvedUnits(t *testing.T) {
	ctx := context.Background()
	db := setupTestDatabase(t, ctx)
	saveSeed(t, ctx, db, "blind-keyset", "sat")
	if _, err := db.pool.Exec(ctx, `INSERT INTO recovery_signature (id, amount, "B_", "C_", created_at, dleq_e, dleq_s) VALUES
		('blind-keyset', 3, decode('01','hex'), decode('02','hex'), 10, NULL, NULL),
		('blind-keyset', 4, decode('03','hex'), decode('04','hex'), 20, NULL, NULL)`); err != nil {
		t.Fatalf("insert recovery_signature: %v", err)
	}
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatalf("GetTx: %v", err)
	}
	defer func() { _ = db.Rollback(ctx, tx) }()
	rows, err := db.GetBlindSigStatsRows(ctx, tx, 10, 20)
	if err != nil {
		t.Fatalf("GetBlindSigStatsRows: %v", err)
	}
	if len(rows) != 2 || rows[0].Unit != "sat" {
		t.Fatalf("unexpected blind sig rows: %#v", rows)
	}
}

func TestGetProofStatsRowsExposesUnresolvedKeysetIDs(t *testing.T) {
	ctx := context.Background()
	db := setupTestDatabase(t, ctx)
	if _, err := db.pool.Exec(ctx, `INSERT INTO proofs (amount, id, secret, c, y, witness, seen_at, state, quote)
		VALUES (3, 'missing-proof', 's1', decode('01','hex'), decode('02','hex'), '', 10, 'SPENT', NULL)`); err != nil {
		t.Fatalf("insert proofs: %v", err)
	}
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatalf("GetTx: %v", err)
	}
	defer func() { _ = db.Rollback(ctx, tx) }()
	rows, err := db.GetProofStatsRows(ctx, tx, 0, 20)
	if err != nil {
		t.Fatalf("GetProofStatsRows: %v", err)
	}
	if len(rows) != 1 || rows[0].KeysetID != "missing-proof" || rows[0].Unit != "" {
		t.Fatalf("unexpected unresolved proof rows: %#v", rows)
	}
}

func TestGetBlindSigStatsRowsExposesUnresolvedKeysetIDs(t *testing.T) {
	ctx := context.Background()
	db := setupTestDatabase(t, ctx)
	if _, err := db.pool.Exec(ctx, `INSERT INTO recovery_signature (id, amount, "B_", "C_", created_at, dleq_e, dleq_s)
		VALUES ('missing-blind', 3, decode('01','hex'), decode('02','hex'), 10, NULL, NULL)`); err != nil {
		t.Fatalf("insert recovery_signature: %v", err)
	}
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatalf("GetTx: %v", err)
	}
	defer func() { _ = db.Rollback(ctx, tx) }()
	rows, err := db.GetBlindSigStatsRows(ctx, tx, 0, 20)
	if err != nil {
		t.Fatalf("GetBlindSigStatsRows: %v", err)
	}
	if len(rows) != 1 || rows[0].KeysetID != "missing-blind" || rows[0].Unit != "" {
		t.Fatalf("unexpected unresolved blind sig rows: %#v", rows)
	}
}

func TestInsertStatsSnapshotPersistsJSONBArraysAndNormalizesNilToEmptyArrays(t *testing.T) {
	ctx := context.Background()
	db := setupTestDatabase(t, ctx)
	//nolint:exhaustruct // test only sets fields relevant to insert assertions
	snapshot := database.StatsSnapshot{
		StartDate:        0,
		EndDate:          10,
		MintSummary:      []database.StatsSummaryItem{{Unit: "sat", Quantity: 1, Amount: 2}},
		MeltSummary:      nil,
		BlindSigsSummary: nil,
		ProofsSummary:    []database.StatsSummaryItem{},
		Fees:             5,
	}
	if err := db.InsertStatsSnapshot(ctx, snapshot); err != nil {
		t.Fatalf("InsertStatsSnapshot: %v", err)
	}
	var meltSummary string
	var blindSummary string
	var fees uint64
	if err := db.pool.QueryRow(ctx, `SELECT melt_summary::text, blind_sigs_summary::text, fees FROM stats LIMIT 1`).Scan(&meltSummary, &blindSummary, &fees); err != nil {
		t.Fatalf("summary query: %v", err)
	}
	if meltSummary != "[]" || blindSummary != "[]" {
		t.Fatalf("expected empty arrays, got melt=%s blind=%s", meltSummary, blindSummary)
	}
	if fees != 5 {
		t.Fatalf("expected fees 5, got %d", fees)
	}
}

func TestGetStatsFeeRowsReturnsSpentProofCountsByKeyset(t *testing.T) {
	ctx := context.Background()
	db := setupTestDatabase(t, ctx)
	saveSeedWithFee(t, ctx, db, "fee-sat", "sat", 1500)
	saveSeedWithFee(t, ctx, db, "fee-auth", "auth", 9999)
	if _, err := db.pool.Exec(ctx, `INSERT INTO proofs (amount, id, secret, c, y, witness, seen_at, state, quote) VALUES
		(3, 'fee-sat', 's1', decode('01','hex'), decode('02','hex'), '', 10, 'SPENT', NULL),
		(4, 'fee-sat', 's2', decode('03','hex'), decode('04','hex'), '', 20, 'SPENT', NULL),
		(5, 'fee-auth', 's3', decode('05','hex'), decode('06','hex'), '', 15, 'SPENT', NULL),
		(6, 'fee-sat', 's4', decode('07','hex'), decode('08','hex'), '', 22, 'PENDING', NULL)`); err != nil {
		t.Fatalf("insert proofs: %v", err)
	}
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatalf("GetTx: %v", err)
	}
	defer func() { _ = db.Rollback(ctx, tx) }()
	rows, err := db.GetStatsFeeRows(ctx, tx, 10, 20)
	if err != nil {
		t.Fatalf("GetStatsFeeRows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 fee rows, got %#v", rows)
	}
	if rows[0].KeysetID != "fee-auth" || rows[0].Quantity != 1 || rows[1].KeysetID != "fee-sat" || rows[1].Quantity != 2 || rows[1].InputFeePpk != 1500 {
		t.Fatalf("unexpected fee rows: %#v", rows)
	}
}

func TestGetStatsSnapshotsBySinceIncludesRowsWithEndDateAtOrAfterSince(t *testing.T) {
	ctx := context.Background()
	db := setupTestDatabase(t, ctx)
	if _, err := db.pool.Exec(ctx, `INSERT INTO stats (start_date, end_date, mint_summary, melt_summary, blind_sigs_summary, proofs_summary, fees) VALUES
		(0, 9, '[]', '[]', '[]', '[]', 1),
		(10, 10, '[]', '[]', '[]', '[]', 2),
		(11, 20, '[]', '[]', '[]', '[]', 3)`); err != nil {
		t.Fatalf("insert stats: %v", err)
	}
	rows, err := db.GetStatsSnapshotsBySince(ctx, 10)
	if err != nil {
		t.Fatalf("GetStatsSnapshotsBySince: %v", err)
	}
	if len(rows) != 2 || rows[0].EndDate != 10 || rows[1].EndDate != 20 {
		t.Fatalf("unexpected filtered rows: %#v", rows)
	}
}

func TestGetStatsSnapshotsBySinceReturnsOrderedRows(t *testing.T) {
	ctx := context.Background()
	db := setupTestDatabase(t, ctx)
	if _, err := db.pool.Exec(ctx, `INSERT INTO stats (start_date, end_date, mint_summary, melt_summary, blind_sigs_summary, proofs_summary, fees) VALUES
		(11, 20, '[]', '[]', '[]', '[]', 5),
		(0, 10, '[]', '[]', '[]', '[]', 3),
		(21, 20, '[]', '[]', '[]', '[]', 7)`); err != nil {
		t.Fatalf("insert stats: %v", err)
	}
	rows, err := db.GetStatsSnapshotsBySince(ctx, 0)
	if err != nil {
		t.Fatalf("GetStatsSnapshotsBySince: %v", err)
	}
	if len(rows) != 3 || rows[0].EndDate != 10 || rows[1].StartDate != 11 || rows[2].StartDate != 21 {
		t.Fatalf("unexpected order: %#v", rows)
	}
}

var _ database.MintDB = (*Postgresql)(nil)
