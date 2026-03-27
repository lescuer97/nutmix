//nolint:exhaustruct,govet
package stats

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/internal/database"
)

type statsMethodContract interface {
	GetReadTx(context.Context) (pgx.Tx, error)
	GetLatestStatsSnapshot(context.Context, pgx.Tx) (*database.StatsSnapshot, error)
	GetMintStatsRows(context.Context, pgx.Tx, int64, int64) ([]database.MintStatsRow, error)
	GetMeltStatsRows(context.Context, pgx.Tx, int64, int64) ([]database.MeltStatsRow, error)
	GetProofStatsRows(context.Context, pgx.Tx, int64, int64) ([]database.KeysetStatsRow, error)
	GetBlindSigStatsRows(context.Context, pgx.Tx, int64, int64) ([]database.KeysetStatsRow, error)
	GetStatsFeeRows(context.Context, pgx.Tx, int64, int64) ([]database.KeysetFeeRow, error)
	GetStatsSnapshotsBySince(context.Context, int64) ([]database.StatsSnapshot, error)
	InsertStatsSnapshot(context.Context, database.StatsSnapshot) error
}

func TestStatsTypesIncludeFees(t *testing.T) {
	_ = database.StatsSummaryItem{}     //nolint:exhaustruct
	_ = database.StatsSnapshot{Fees: 1} //nolint:exhaustruct
	_ = database.MintStatsRow{}         //nolint:exhaustruct
	_ = database.MeltStatsRow{}         //nolint:exhaustruct
	_ = database.KeysetStatsRow{}       //nolint:exhaustruct
	_ = database.KeysetFeeRow{}         //nolint:exhaustruct
	var _ statsMethodContract = (database.MintDB)(nil)
}

//nolint:govet // test double field order is not performance critical
type stubStore struct {
	latest       *database.StatsSnapshot
	mintRows     []database.MintStatsRow
	meltRows     []database.MeltStatsRow
	proofRows    []database.KeysetStatsRow
	blindSigRows []database.KeysetStatsRow
	feeRows      []database.KeysetFeeRow
	inserted     []database.StatsSnapshot
	insertErr    error
	rollbacks    int
}

func (s *stubStore) GetReadTx(context.Context) (pgx.Tx, error) { return nil, nil }
func (s *stubStore) Rollback(context.Context, pgx.Tx) error {
	s.rollbacks++
	return nil
}
func (s *stubStore) GetLatestStatsSnapshot(context.Context, pgx.Tx) (*database.StatsSnapshot, error) {
	return s.latest, nil
}
func (s *stubStore) GetMintStatsRows(context.Context, pgx.Tx, int64, int64) ([]database.MintStatsRow, error) {
	return s.mintRows, nil
}
func (s *stubStore) GetMeltStatsRows(context.Context, pgx.Tx, int64, int64) ([]database.MeltStatsRow, error) {
	return s.meltRows, nil
}
func (s *stubStore) GetProofStatsRows(context.Context, pgx.Tx, int64, int64) ([]database.KeysetStatsRow, error) {
	return s.proofRows, nil
}
func (s *stubStore) GetBlindSigStatsRows(context.Context, pgx.Tx, int64, int64) ([]database.KeysetStatsRow, error) {
	return s.blindSigRows, nil
}
func (s *stubStore) GetStatsFeeRows(context.Context, pgx.Tx, int64, int64) ([]database.KeysetFeeRow, error) {
	return s.feeRows, nil
}
func (s *stubStore) GetStatsSnapshotsBySince(context.Context, int64) ([]database.StatsSnapshot, error) {
	return s.inserted, nil
}
func (s *stubStore) InsertStatsSnapshot(_ context.Context, snapshot database.StatsSnapshot) error {
	s.inserted = append(s.inserted, snapshot)
	return s.insertErr
}

func newTestService(store Store) Service {
	return Service{ //nolint:exhaustruct
		DB:  store,
		Now: func() time.Time { return time.Unix(110, 0) },
		DecodeMintAmount: func(request string) (uint64, error) {
			if request == "decode-error" {
				return 0, errors.New("decode failed")
			}
			return 77, nil
		},
	}
}

func TestCreateSnapshotStartsAtZeroWhenNoPriorSnapshot(t *testing.T) {
	store := &stubStore{mintRows: []database.MintStatsRow{{Quote: "q1", Unit: "sat", Amount: uint64Ptr(5)}}} //nolint:exhaustruct
	service := newTestService(store)

	result, err := service.CreateSnapshot(context.Background())
	if err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}
	if result.StartDate != 0 {
		t.Fatalf("expected start date 0, got %d", result.StartDate)
	}
	if result.EndDate != 100 {
		t.Fatalf("expected end date 100, got %d", result.EndDate)
	}
	if result.Outcome != SnapshotInserted {
		t.Fatalf("expected inserted outcome, got %s", result.Outcome)
	}
}

func TestCreateSnapshotUsesPreviousEndDatePlusOne(t *testing.T) {
	store := &stubStore{ //nolint:exhaustruct
		latest:   &database.StatsSnapshot{EndDate: 12}, //nolint:exhaustruct
		meltRows: []database.MeltStatsRow{{Quote: "m1", Unit: "sat", Amount: 9}},
	}
	service := newTestService(store)

	result, err := service.CreateSnapshot(context.Background())
	if err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}
	if result.StartDate != 13 {
		t.Fatalf("expected start date 13, got %d", result.StartDate)
	}
}

func TestCreateSnapshotSkipsWhenEndDateBeforeStartDate(t *testing.T) {
	store := &stubStore{latest: &database.StatsSnapshot{EndDate: 100}} //nolint:exhaustruct
	service := newTestService(store)
	service.Now = func() time.Time { return time.Unix(105, 0) }

	result, err := service.CreateSnapshot(context.Background())
	if err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}
	if result.Outcome != SnapshotSkipped {
		t.Fatalf("expected skipped outcome, got %s", result.Outcome)
	}
	if len(store.inserted) != 0 {
		t.Fatalf("expected no inserts, got %d", len(store.inserted))
	}
}

func TestCreateSnapshotSkipsWhenAllSummariesEmpty(t *testing.T) {
	store := &stubStore{} //nolint:exhaustruct
	service := newTestService(store)

	result, err := service.CreateSnapshot(context.Background())
	if err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}
	if result.Outcome != SnapshotSkipped {
		t.Fatalf("expected skipped outcome, got %s", result.Outcome)
	}
	if len(store.inserted) != 0 {
		t.Fatalf("expected no inserts, got %d", len(store.inserted))
	}
}

func TestCreateSnapshotSortsSummaryItemsByUnit(t *testing.T) {
	store := &stubStore{ //nolint:exhaustruct
		meltRows: []database.MeltStatsRow{{Quote: "1", Unit: "zsat", Amount: 1}, {Quote: "2", Unit: "asat", Amount: 2}},
	}
	service := newTestService(store)

	_, err := service.CreateSnapshot(context.Background())
	if err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}
	if len(store.inserted) != 1 {
		t.Fatalf("expected one insert, got %d", len(store.inserted))
	}
	got := store.inserted[0].MeltSummary
	if len(got) != 2 || got[0].Unit != "asat" || got[1].Unit != "zsat" {
		t.Fatalf("expected sorted summary, got %#v", got)
	}
}

func TestCreateSnapshotAbortsWhenMintFallbackCannotBeResolved(t *testing.T) {
	store := &stubStore{mintRows: []database.MintStatsRow{{Quote: "q1", Unit: "usd", Amount: nil, Request: "invoice"}}} //nolint:exhaustruct
	service := newTestService(store)

	_, err := service.CreateSnapshot(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCreateSnapshotAbortsWhenProofUnitCannotBeResolved(t *testing.T) {
	store := &stubStore{proofRows: []database.KeysetStatsRow{{KeysetID: "proof-keyset", Amount: 1}}} //nolint:exhaustruct
	service := newTestService(store)

	_, err := service.CreateSnapshot(context.Background())
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("proof-keyset")) {
		t.Fatalf("expected keyset error, got %v", err)
	}
}

func TestCreateSnapshotAbortsWhenBlindSigUnitCannotBeResolved(t *testing.T) {
	store := &stubStore{blindSigRows: []database.KeysetStatsRow{{KeysetID: "blind-keyset", Amount: 1}}} //nolint:exhaustruct
	service := newTestService(store)

	_, err := service.CreateSnapshot(context.Background())
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("blind-keyset")) {
		t.Fatalf("expected keyset error, got %v", err)
	}
}

func TestCreateSnapshotUsesMintAmountOrInvoiceFallback(t *testing.T) {
	store := &stubStore{ //nolint:exhaustruct
		mintRows: []database.MintStatsRow{
			{Quote: "stored", Unit: "sat", Amount: uint64Ptr(3)},
			{Quote: "decoded", Unit: "sat", Request: "invoice"},
		},
	}
	service := newTestService(store)

	_, err := service.CreateSnapshot(context.Background())
	if err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}
	got := store.inserted[0].MintSummary
	if len(got) != 1 || got[0].Quantity != 2 || got[0].Amount != 80 {
		t.Fatalf("unexpected mint summary: %#v", got)
	}
}

func TestCreateSnapshotAggregatesMeltRowsUsingStoredAmount(t *testing.T) {
	store := &stubStore{ //nolint:exhaustruct
		meltRows: []database.MeltStatsRow{{Quote: "m1", Unit: "sat", Amount: 4}, {Quote: "m2", Unit: "sat", Amount: 6}},
	}
	service := newTestService(store)

	_, err := service.CreateSnapshot(context.Background())
	if err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}
	got := store.inserted[0].MeltSummary
	if len(got) != 1 || got[0].Quantity != 2 || got[0].Amount != 10 {
		t.Fatalf("unexpected melt summary: %#v", got)
	}
}

func TestCreateSnapshotCalculatesFeesLikeAdmin(t *testing.T) {
	store := &stubStore{ //nolint:exhaustruct
		meltRows: []database.MeltStatsRow{{Quote: "m1", Unit: "sat", Amount: 1}},
		feeRows: []database.KeysetFeeRow{
			{KeysetID: "sat-a", Unit: "sat", Quantity: 2, InputFeePpk: 1500},
			{KeysetID: "sat-b", Unit: "sat", Quantity: 1, InputFeePpk: 500},
		},
	}
	service := newTestService(store)

	_, err := service.CreateSnapshot(context.Background())
	if err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}
	if got := store.inserted[0].Fees; got != 4 {
		t.Fatalf("expected fees 4, got %d", got)
	}
}

func TestCreateSnapshotExcludesAuthFees(t *testing.T) {
	store := &stubStore{ //nolint:exhaustruct
		meltRows: []database.MeltStatsRow{{Quote: "m1", Unit: "sat", Amount: 1}},
		feeRows: []database.KeysetFeeRow{
			{KeysetID: "auth-a", Unit: "auth", Quantity: 9, InputFeePpk: 9999},
			{KeysetID: "sat-a", Unit: "sat", Quantity: 1, InputFeePpk: 1000},
		},
	}
	service := newTestService(store)

	_, err := service.CreateSnapshot(context.Background())
	if err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}
	if got := store.inserted[0].Fees; got != 1 {
		t.Fatalf("expected fees 1, got %d", got)
	}
}

func TestCreateSnapshotRoundsFeesLikeAdmin(t *testing.T) {
	store := &stubStore{
		meltRows: []database.MeltStatsRow{{Quote: "m1", Unit: "sat", Amount: 1}},
		feeRows:  []database.KeysetFeeRow{{KeysetID: "sat-a", Unit: "sat", Quantity: 1, InputFeePpk: 1001}},
	}
	service := newTestService(store)

	_, err := service.CreateSnapshot(context.Background())
	if err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}
	if got := store.inserted[0].Fees; got != 2 {
		t.Fatalf("expected rounded fees 2, got %d", got)
	}
}

type fakeTicker struct{ ch chan time.Time }

func (f fakeTicker) C() <-chan time.Time { return f.ch }
func (f fakeTicker) Stop()               {}

func TestRunStartsWithImmediateSnapshotAttempt(t *testing.T) {
	service := Service{} //nolint:exhaustruct
	called := make(chan struct{}, 1)
	service.runSnapshot = func(context.Context) (SnapshotResult, error) {
		called <- struct{}{}
		return SnapshotResult{}, nil //nolint:exhaustruct
	}
	service.NewTicker = func(time.Duration) ticker { return fakeTicker{ch: make(chan time.Time)} }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go service.Run(ctx, time.Hour)

	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("expected immediate snapshot attempt")
	}
}

func TestRunStopsWhenContextCancelled(t *testing.T) {
	service := Service{} //nolint:exhaustruct
	tickCh := make(chan time.Time)
	service.runSnapshot = func(context.Context) (SnapshotResult, error) { return SnapshotResult{}, nil } //nolint:exhaustruct
	service.NewTicker = func(time.Duration) ticker { return fakeTicker{ch: tickCh} }

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		service.Run(ctx, time.Hour)
		close(done)
	}()
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("expected Run to stop on cancellation")
	}
}

func TestRunDoesNotOverlapSnapshotAttempts(t *testing.T) {
	service := Service{} //nolint:exhaustruct
	tickCh := make(chan time.Time, 2)
	start := make(chan struct{}, 2)
	release := make(chan struct{})
	completed := make(chan struct{}, 2)
	var running int
	var mu sync.Mutex

	service.runSnapshot = func(context.Context) (SnapshotResult, error) {
		mu.Lock()
		running++
		if running > 1 {
			mu.Unlock()
			t.Fatal("overlapping snapshot attempts detected")
		}
		mu.Unlock()
		start <- struct{}{}
		<-release
		mu.Lock()
		running--
		mu.Unlock()
		completed <- struct{}{}
		return SnapshotResult{}, nil //nolint:exhaustruct
	}
	service.NewTicker = func(time.Duration) ticker { return fakeTicker{ch: tickCh} }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go service.Run(ctx, time.Hour)
	<-start
	tickCh <- time.Now()
	tickCh <- time.Now()
	time.Sleep(50 * time.Millisecond)
	if len(start) != 0 {
		t.Fatal("expected no overlapping start before first snapshot completes")
	}
	close(release)
	select {
	case <-completed:
	case <-time.After(time.Second):
		t.Fatal("expected first completion")
	}
	select {
	case <-start:
	case <-time.After(time.Second):
		t.Fatal("expected queued snapshot to start after first completion")
	}
}

func TestRunContinuesAfterSnapshotError(t *testing.T) {
	service := Service{} //nolint:exhaustruct
	tickCh := make(chan time.Time, 1)
	var calls int
	service.runSnapshot = func(context.Context) (SnapshotResult, error) {
		calls++
		if calls == 1 {
			return SnapshotResult{StartDate: 0, EndDate: 10}, errors.New("boom") //nolint:exhaustruct
		}
		return SnapshotResult{Outcome: SnapshotSkipped, StartDate: 0, EndDate: 10}, nil
	}
	service.NewTicker = func(time.Duration) ticker { return fakeTicker{ch: tickCh} }
	var logBuf bytes.Buffer
	service.Logger = slog.New(slog.NewJSONHandler(&logBuf, nil))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go service.Run(ctx, time.Hour)
	time.Sleep(50 * time.Millisecond)
	tickCh <- time.Now()
	time.Sleep(50 * time.Millisecond)

	if calls < 2 {
		t.Fatalf("expected service to continue after error, got %d calls", calls)
	}
	if !bytes.Contains(logBuf.Bytes(), []byte("stats snapshot failed")) {
		t.Fatalf("expected error log, got %s", logBuf.String())
	}
	if !bytes.Contains(logBuf.Bytes(), []byte("stats snapshot skipped")) {
		t.Fatalf("expected skip log, got %s", logBuf.String())
	}
}

func uint64Ptr(v uint64) *uint64 { return &v }
