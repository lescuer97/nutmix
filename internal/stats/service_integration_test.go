//nolint:exhaustruct,contextcheck
package stats

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/database/postgresql"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestCreateSnapshotWithRealPostgresPersistsExpectedStatsRow(t *testing.T) {
	ctx := context.Background()
	db := setupStatsIntegrationDB(t, ctx)

	seedStatsIntegrationFixtures(t, ctx, db)

	//nolint:exhaustruct // optional service hooks use zero values in this integration test
	service := Service{
		DB:  db,
		Now: func() time.Time { return time.Unix(110, 0) },
		DecodeMintAmount: func(string) (uint64, error) {
			return 0, fmt.Errorf("decoder should not be called")
		},
	}

	result, err := service.CreateSnapshot(ctx)
	if err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}
	if result.Outcome != SnapshotInserted {
		t.Fatalf("expected inserted outcome, got %s", result.Outcome)
	}
	if result.StartDate != 0 || result.EndDate != 100 {
		t.Fatalf("unexpected window: %#v", result)
	}

	readTx, err := db.GetReadTx(ctx)
	if err != nil {
		t.Fatalf("GetReadTx: %v", err)
	}
	defer func() { _ = db.Rollback(ctx, readTx) }()

	snapshot, err := db.GetLatestStatsSnapshot(ctx, readTx)
	if err != nil {
		t.Fatalf("GetLatestStatsSnapshot: %v", err)
	}
	if snapshot == nil {
		t.Fatal("expected snapshot row")
	}
	if snapshot.StartDate != 0 || snapshot.EndDate != 100 {
		t.Fatalf("unexpected persisted window: %#v", snapshot)
	}
	assertSummary(t, snapshot.MintSummary, []databaseSummary{{Unit: "sat", Quantity: 2, Amount: 15}, {Unit: "usd", Quantity: 1, Amount: 9}})
	assertSummary(t, snapshot.MeltSummary, []databaseSummary{{Unit: "sat", Quantity: 2, Amount: 10}, {Unit: "usd", Quantity: 1, Amount: 8}})
	assertSummary(t, snapshot.ProofsSummary, []databaseSummary{{Unit: "sat", Quantity: 1, Amount: 11}, {Unit: "usd", Quantity: 1, Amount: 7}})
	assertSummary(t, snapshot.BlindSigsSummary, []databaseSummary{{Unit: "sat", Quantity: 1, Amount: 12}, {Unit: "usd", Quantity: 1, Amount: 13}})
	if snapshot.Fees != 4 {
		t.Fatalf("expected fees 4, got %d", snapshot.Fees)
	}
}

type databaseSummary struct {
	Unit     string
	Quantity uint64
	Amount   uint64
}

func assertSummary(t *testing.T, got []database.StatsSummaryItem, want []databaseSummary) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("unexpected summary length: got %#v want %#v", got, want)
	}
	for i, item := range got {
		if item.Unit != want[i].Unit || item.Quantity != want[i].Quantity || item.Amount != want[i].Amount {
			t.Fatalf("unexpected summary item at %d: got %#v want %#v", i, item, want[i])
		}
	}
}

func setupStatsIntegrationDB(t *testing.T, ctx context.Context) postgresql.Postgresql {
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

	db, err := postgresql.DatabaseSetup(ctx, "migrations")
	if err != nil {
		t.Fatalf("DatabaseSetup: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func seedStatsIntegrationFixtures(t *testing.T, ctx context.Context, db postgresql.Postgresql) {
	t.Helper()
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatalf("GetTx: %v", err)
	}
	defer func() { _ = db.Rollback(ctx, tx) }()

	for _, seed := range []cashu.Seed{ //nolint:exhaustruct // integration fixtures only set needed seed fields
		{Active: true, CreatedAt: 1, Unit: "sat", Id: "ks-sat", Version: 1, InputFeePpk: 1500, DerivationPath: "m/0", Amounts: []uint64{1}, Legacy: false},
		{Active: true, CreatedAt: 1, Unit: "usd", Id: "ks-usd", Version: 1, InputFeePpk: 2500, DerivationPath: "m/1", Amounts: []uint64{1}, Legacy: false},
	} {
		//nolint:contextcheck // SaveNewSeed does not take context
		if err := db.SaveNewSeed(tx, seed); err != nil {
			t.Fatalf("SaveNewSeed: %v", err)
		}
	}

	if _, err := tx.Exec(ctx, `INSERT INTO mint_request (quote, request, expiry, unit, minted, state, seen_at, amount, checking_id, pubkey, description) VALUES
		('mint-sat-1', 'req1', 1000, 'sat', true, 'ISSUED', 10, 10, '', NULL, NULL),
		('mint-sat-2', 'req2', 1000, 'sat', true, 'PAID', 20, 5, '', NULL, NULL),
		('mint-usd-1', 'req3', 1000, 'usd', true, 'ISSUED', 30, 9, '', NULL, NULL),
		('mint-pending', 'req4', 1000, 'sat', false, 'PENDING', 40, 99, '', NULL, NULL)`); err != nil {
		t.Fatalf("insert mint_request: %v", err)
	}

	if _, err := tx.Exec(ctx, `INSERT INTO melt_request (quote, request, fee_reserve, expiry, unit, amount, melted, state, payment_preimage, seen_at, mpp, fee_paid, checking_id) VALUES
		('melt-sat-1', 'mreq1', 0, 1000, 'sat', 4, true, 'ISSUED', '', 10, false, 0, ''),
		('melt-sat-2', 'mreq2', 0, 1000, 'sat', 6, true, 'PAID', '', 20, true, 0, ''),
		('melt-usd-1', 'mreq3', 0, 1000, 'usd', 8, true, 'ISSUED', '', 25, false, 0, ''),
		('melt-pending', 'mreq4', 0, 1000, 'sat', 77, false, 'PENDING', '', 30, false, 0, '')`); err != nil {
		t.Fatalf("insert melt_request: %v", err)
	}

	if _, err := tx.Exec(ctx, `INSERT INTO proofs (amount, id, secret, c, y, witness, seen_at, state, quote) VALUES
		(11, 'ks-sat', 'proof-1', decode('01', 'hex'), decode('02', 'hex'), '', 12, 'SPENT', NULL),
		(7, 'ks-usd', 'proof-2', decode('03', 'hex'), decode('04', 'hex'), '', 22, 'SPENT', NULL),
		(99, 'ks-sat', 'proof-3', decode('05', 'hex'), decode('06', 'hex'), '', 32, 'UNSPENT', NULL)`); err != nil {
		t.Fatalf("insert proofs: %v", err)
	}

	if _, err := tx.Exec(ctx, `INSERT INTO recovery_signature (id, amount, "B_", "C_", created_at, dleq_e, dleq_s) VALUES
		('ks-sat', 12, decode('07', 'hex'), decode('08', 'hex'), 14, NULL, NULL),
		('ks-usd', 13, decode('09', 'hex'), decode('0a', 'hex'), 24, NULL, NULL)`); err != nil {
		t.Fatalf("insert recovery_signature: %v", err)
	}

	if err := db.Commit(ctx, tx); err != nil {
		t.Fatalf("Commit: %v", err)
	}
}
