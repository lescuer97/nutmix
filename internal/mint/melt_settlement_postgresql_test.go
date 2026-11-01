//go:build integration

//nolint:exhaustruct_v5 // Integration fixtures set only fields relevant to settlement.
package mint

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database/postgresql"
	"github.com/lescuer97/nutmix/internal/lightning"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const postgresMeltSettlementQuote = "integration-melt-settlement"

type meltSettlementBarrierSigner struct {
	changeLifecycleSigner
	entered chan struct{}
	release chan struct{}
	once    sync.Once
	calls   atomic.Int32
}

type startupCountingSigner struct {
	changeLifecycleSigner
	calls atomic.Int32
}

func (s *startupCountingSigner) SignBlindMessages(ctx context.Context, messages []cashu.BlindedMessage) ([]cashu.BlindSignature, []cashu.SignedOutput, error) {
	s.calls.Add(1)
	return s.changeLifecycleSigner.SignBlindMessages(ctx, messages)
}

func (s *meltSettlementBarrierSigner) SignBlindMessages(ctx context.Context, messages []cashu.BlindedMessage) ([]cashu.BlindSignature, []cashu.SignedOutput, error) {
	s.calls.Add(1)
	s.once.Do(func() { close(s.entered) })
	select {
	case <-s.release:
		return s.changeLifecycleSigner.SignBlindMessages(ctx, messages)
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
}

func setupPostgresMeltSettlement(t *testing.T) (postgresql.Postgresql, context.Context, cashu.MeltRequestDB, cashu.Proofs, []cashu.BlindedMessage) {
	t.Helper()
	ctx := context.Background()
	container, err := postgres.Run(ctx, "postgres:16.2",
		postgres.WithDatabase("postgres"),
		postgres.WithUsername("user"),
		postgres.WithPassword("password"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(30*time.Second)),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			t.Errorf("terminate postgres container: %v", err)
		}
	})
	uri, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatal(fmt.Errorf("postgres connection string: %w", err))
	}
	t.Setenv("DATABASE_URL", uri)
	db, err := postgresql.DatabaseSetup(ctx, "migrations")
	if err != nil {
		t.Fatal(fmt.Errorf("setup postgres database: %w", err))
	}
	t.Cleanup(db.Close)

	wallet := lightning.FakeWallet{Network: chaincfg.RegressionNetParams}
	invoice, err := wallet.RequestInvoice(cashu.NewAmount(cashu.Sat, 2), nil)
	if err != nil {
		t.Fatal(fmt.Errorf("create test invoice: %w", err))
	}
	quote := cashu.MeltRequestDB{
		Quote: postgresMeltSettlementQuote, Request: invoice.PaymentRequest, Unit: cashu.Sat.String(),
		CheckingId: invoice.CheckingId, Amount: 2, State: cashu.PENDING,
	}
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	proofQuote := quote.Quote
	proofs := cashu.Proofs{{
		C:  cashu.WrappedPublicKey{PublicKey: secp256k1.PrivKeyFromBytes([]byte{51}).PubKey()},
		Y:  cashu.WrappedPublicKey{PublicKey: secp256k1.PrivKeyFromBytes([]byte{61}).PubKey()},
		Id: testSatKeysetId, Secret: "settlement-proof", Amount: 15, State: cashu.PROOF_PENDING, Quote: &proofQuote,
	}}
	blanks := make(cashu.BlindedMessages, 3)
	for i := range blanks {
		blanks[i] = cashu.BlindedMessage{
			B_: cashu.WrappedPublicKey{PublicKey: secp256k1.PrivKeyFromBytes([]byte{byte(71 + i)}).PubKey()},
			Id: testSatKeysetId,
		}
	}
	if err := db.SaveMeltRequest(tx, quote); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := db.SaveProof(ctx, tx, proofs); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := db.ReserveMeltChangeOutputs(ctx, tx, blanks, quote.Quote); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := db.Commit(ctx, tx); err != nil {
		t.Fatal(err)
	}
	return db, ctx, quote, proofs, blanks
}

func TestPostgresMeltSettlementStartupRecoversInternalPreparation(t *testing.T) {
	db, ctx, quote, proofs, blanks := setupPostgresMeltSettlement(t)
	prepared, err := db.GetTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Rollback(ctx, prepared) }()
	if err := db.ChangeMeltRequestState(ctx, prepared, quote.Quote, cashu.PAID, true, 1); err != nil {
		t.Fatal(err)
	}
	if err := db.AddPreimageMeltRequest(ctx, prepared, quote.Quote, "startup-durable-preimage"); err != nil {
		t.Fatal(err)
	}
	if err := db.Commit(ctx, prepared); err != nil {
		t.Fatal(err)
	}

	signer := &startupCountingSigner{}
	mint := &Mint{MintDB: db, Signer: signer, LightningBackend: noLightningReconciliationWallet{}}
	if err := mint.ReconcilePendingMeltQuotes(ctx); err != nil {
		t.Fatalf("startup reconciliation: %v", err)
	}
	if signer.calls.Load() != 1 {
		t.Fatalf("signer calls = %d, want 1", signer.calls.Load())
	}
	if err := mint.ReconcilePendingMeltQuotes(ctx); err != nil {
		t.Fatalf("second startup reconciliation: %v", err)
	}
	if signer.calls.Load() != 1 {
		t.Fatalf("second startup signed again: %d calls", signer.calls.Load())
	}

	checkTx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Rollback(ctx, checkTx) }()
	settled, err := db.GetMeltRequestById(ctx, checkTx, quote.Quote)
	if err != nil {
		t.Fatal(err)
	}
	if settled.State != cashu.PAID || !settled.Melted || settled.FeePaid != 1 || settled.PaymentPreimage != "startup-durable-preimage" {
		t.Fatalf("durable quote fields changed: %+v", settled)
	}
	storedProofs, err := db.GetProofsFromQuote(ctx, checkTx, quote.Quote)
	if err != nil {
		t.Fatal(err)
	}
	for _, proof := range storedProofs {
		if proof.State != cashu.PROOF_SPENT {
			t.Errorf("proof %s state = %v, want SPENT", proof.Secret, proof.State)
		}
	}
	if len(storedProofs) != len(proofs) {
		t.Fatalf("stored proofs = %d, want %d", len(storedProofs), len(proofs))
	}
	remaining, err := db.LoadPendingMeltChangeForUpdate(ctx, checkTx, quote.Quote)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 0 {
		t.Fatalf("unused reserved blanks remain: %d", len(remaining))
	}
	blankKeys := make([]cashu.WrappedPublicKey, len(blanks))
	for i := range blanks {
		blankKeys[i] = blanks[i].B_
	}
	restored, err := db.GetRestoreSigsFromBlindedMessages(ctx, checkTx, blankKeys)
	if err != nil {
		t.Fatal(err)
	}
	if len(restored) != 2 {
		t.Fatalf("persisted change signatures = %d, want 2", len(restored))
	}
	wantAmounts := map[string]uint64{blanks[0].B_.ToHex(): 4, blanks[1].B_.ToHex(): 8}
	seen := make(map[string]bool, len(restored))
	for _, sig := range restored {
		point := sig.B_.ToHex()
		if wantAmounts[point] != sig.Amount || sig.MeltQuote != quote.Quote || sig.Id != testSatKeysetId || sig.C_.ToHex() != point {
			t.Errorf("persisted signature for %s = %+v; want quote %s and amount %d", point, sig, quote.Quote, wantAmounts[point])
		}
		seen[point] = true
	}
	for point := range wantAmounts {
		if !seen[point] {
			t.Errorf("missing persisted change signature for blank %s", point)
		}
	}
	if seen[blanks[2].B_.ToHex()] {
		t.Errorf("unused third blank unexpectedly has a persisted signature")
	}
}

func TestPostgresMeltSettlementSigningCancellationReleasesLocks(t *testing.T) {
	db, ctx, quote, proofs, blanks := setupPostgresMeltSettlement(t)
	entered := make(chan struct{})
	mint := &Mint{MintDB: db, Signer: &meltSettlementBarrierSigner{entered: entered}}
	request := cashu.PostMeltBolt11Request{Inputs: proofs}
	observed := quote
	observed.State, observed.Melted, observed.FeePaid, observed.PaymentPreimage = cashu.PAID, true, 1, "preimage-fixture"
	parent, cancel := context.WithCancel(ctx)
	type settlementOutcome struct {
		err    error
		change []cashu.BlindSignature
	}
	done := make(chan settlementOutcome, 1)
	finished := false
	t.Cleanup(func() {
		cancel()
		if finished {
			return
		}
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("canceled settlement goroutine did not exit")
		}
	})
	go func() {
		_, response, _, err := mint.finalizePaidBolt11Melt(parent,
			bolt11MeltData{AmountProofs: cashu.NewAmount(cashu.Sat, 15), Fee: cashu.NewAmount(cashu.Sat, 0), Unit: cashu.Sat},
			request, observed, cashu.NewAmount(cashu.Sat, 1))
		done <- settlementOutcome{change: response.Change, err: err}
	}()
	select {
	case <-entered:
	case <-time.After(20 * time.Second):
		t.Fatal("signer was not entered while settlement transaction held its locks")
	}
	cancel()
	var outcome settlementOutcome
	select {
	case outcome = <-done:
		finished = true
	case <-time.After(20 * time.Second):
		t.Fatal("canceled settlement did not return")
	}
	if !errors.Is(outcome.err, context.Canceled) {
		t.Fatalf("settlement error=%v, want context cancellation", outcome.err)
	}
	if len(outcome.change) != 0 {
		t.Fatalf("returned %d change signatures after cancellation", len(outcome.change))
	}
	assertPostgresMeltSettlementUnchanged(t, db, ctx, quote.Quote, proofs, blanks)
}

func TestPostgresMeltSettlementLateFailureRollsBack(t *testing.T) {
	db, ctx, quote, proofs, blanks := setupPostgresMeltSettlement(t)
	triggerTx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := triggerTx.Exec(ctx, `CREATE FUNCTION fail_melt_settlement_quote_update() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'intentional late settlement failure' USING ERRCODE = '23514', CONSTRAINT = 'test_melt_settlement_late_failure'; END; $$`); err != nil {
		_ = triggerTx.Rollback(ctx)
		t.Fatal(err)
	}
	if _, err := triggerTx.Exec(ctx, `CREATE TRIGGER fail_melt_settlement_quote_update BEFORE UPDATE ON melt_request FOR EACH ROW EXECUTE FUNCTION fail_melt_settlement_quote_update()`); err != nil {
		_ = triggerTx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := db.Commit(ctx, triggerTx); err != nil {
		t.Fatal(err)
	}

	mint := &Mint{MintDB: db, Signer: changeLifecycleSigner{}}
	observed := quote
	observed.State, observed.Melted, observed.FeePaid, observed.PaymentPreimage = cashu.PAID, true, 1, "preimage-fixture"
	_, response, _, err := mint.finalizePaidBolt11Melt(ctx,
		bolt11MeltData{AmountProofs: cashu.NewAmount(cashu.Sat, 15), Fee: cashu.NewAmount(cashu.Sat, 0), Unit: cashu.Sat},
		cashu.PostMeltBolt11Request{Inputs: proofs}, observed, cashu.NewAmount(cashu.Sat, 1))
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" || pgErr.ConstraintName != "test_melt_settlement_late_failure" {
		t.Fatalf("settlement error=%v, want trigger PgError 23514", err)
	}
	if len(response.Change) != 0 {
		t.Fatalf("returned %d change signatures after failed transaction", len(response.Change))
	}
	assertPostgresMeltSettlementUnchanged(t, db, ctx, quote.Quote, proofs, blanks)
}

func TestPostgresMeltSettlementConcurrentOriginalAndPoll(t *testing.T) {
	for _, winner := range []string{"original-first", "poll-first"} {
		t.Run(winner, func(t *testing.T) {
			db, ctx, quote, proofs, blanks := setupPostgresMeltSettlement(t)
			workCtx, cancel := context.WithCancel(ctx)
			entered := make(chan struct{})
			release := make(chan struct{})
			var releaseOnce sync.Once
			signer := &meltSettlementBarrierSigner{entered: entered, release: release}
			mint := &Mint{
				MintDB: db, Signer: signer,
				LightningBackend: lightning.FakeWallet{Network: chaincfg.RegressionNetParams},
			}
			observed := quote
			observed.State, observed.Melted = cashu.PAID, true
			observed.FeePaid, observed.PaymentPreimage = 10, "fakewalletpreimage"
			request := cashu.PostMeltBolt11Request{Inputs: proofs}
			fee := cashu.NewAmount(cashu.Sat, 10)
			meltData := bolt11MeltData{AmountProofs: cashu.NewAmount(cashu.Sat, 15), Fee: cashu.NewAmount(cashu.Sat, 0), Unit: cashu.Sat}
			type workflowResult struct {
				err    error
				quote  *cashu.MeltRequestDB
				change []cashu.BlindSignature
			}
			original := func() workflowResult {
				settled, response, _, err := mint.finalizePaidBolt11Melt(workCtx, meltData, request, observed, fee)
				return workflowResult{quote: &settled, change: response.Change, err: err}
			}
			poll := func() workflowResult {
				settled, err := mint.RefreshMeltQuoteState(workCtx, quote.Quote)
				return workflowResult{quote: &settled, change: settled.Change, err: err}
			}
			first, second := original, poll
			if winner == "poll-first" {
				first, second = poll, original
			}
			firstDone := make(chan workflowResult, 1)
			var workflows sync.WaitGroup
			workflows.Add(1)
			go func() {
				defer workflows.Done()
				firstDone <- first()
			}()
			t.Cleanup(func() {
				cancel()
				releaseOnce.Do(func() { close(release) })
				finished := make(chan struct{})
				go func() {
					workflows.Wait()
					close(finished)
				}()
				select {
				case <-finished:
				case <-time.After(5 * time.Second):
					t.Error("settlement workflow goroutines did not exit")
				}
			})
			select {
			case <-entered:
			case <-time.After(20 * time.Second):
				t.Fatal("settlement signer was not entered")
			}
			secondDone := make(chan workflowResult, 1)
			workflows.Add(1)
			go func() {
				defer workflows.Done()
				secondDone <- second()
			}()
			var losing workflowResult
			select {
			case losing = <-secondDone:
			case <-time.After(20 * time.Second):
				cancel()
				t.Fatal("competing workflow did not return from NOWAIT lock attempt")
			}
			var lockErr *pgconn.PgError
			if losing.err == nil || (!errors.Is(losing.err, cashu.ErrQuoteIsPending) && (!errors.As(losing.err, &lockErr) || lockErr.Code != "55P03")) {
				t.Fatalf("competing workflow error=%v, want NOWAIT/pending error", losing.err)
			}
			if len(losing.change) != 0 {
				t.Fatalf("losing workflow returned %d signatures", len(losing.change))
			}
			releaseOnce.Do(func() { close(release) })
			var committed workflowResult
			select {
			case committed = <-firstDone:
			case <-time.After(20 * time.Second):
				cancel()
				t.Fatal("winning workflow did not commit after signer release")
			}
			if committed.err != nil {
				t.Fatalf("winning workflow failed: %v", committed.err)
			}
			if committed.quote.State != cashu.PAID || !committed.quote.Melted || committed.quote.FeePaid != 10 || committed.quote.PaymentPreimage != "fakewalletpreimage" {
				t.Fatalf("unexpected committed quote: %+v", committed.quote)
			}
			if len(committed.change) == 0 || len(committed.change) != len(committed.quote.Change) {
				t.Fatalf("returned change count=%d, durable change count=%d", len(committed.change), len(committed.quote.Change))
			}
			retry := second()
			if retry.err != nil {
				t.Fatalf("explicit idempotency call failed: %v", retry.err)
			}
			if retry.quote.FeePaid != committed.quote.FeePaid || retry.quote.PaymentPreimage != committed.quote.PaymentPreimage || len(retry.change) != len(committed.change) {
				t.Fatalf("retry differs from durable settlement: retry=%+v committed=%+v", retry.quote, committed.quote)
			}
			for i, sig := range committed.change {
				if sig.Amount != committed.quote.Change[i].Amount || sig.Id != committed.quote.Change[i].Id || sig.C_.ToHex() != committed.quote.Change[i].C_.ToHex() || retry.change[i].C_.ToHex() != sig.C_.ToHex() {
					t.Fatalf("signature %d differs across committed workflow and retry", i)
				}
			}
			if got := signer.calls.Load(); got != 1 {
				t.Fatalf("signer called %d times, want exactly once", got)
			}
			assertPostgresMeltSettlementCommitted(t, db, ctx, quote.Quote, proofs, blanks, committed.quote.Change)
		})
	}
}

func assertPostgresMeltSettlementCommitted(t *testing.T, db postgresql.Postgresql, ctx context.Context, quoteID string, expectedProofs cashu.Proofs, blanks []cashu.BlindedMessage, expectedChange []cashu.BlindSignature) {
	t.Helper()
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Rollback(context.WithoutCancel(ctx), tx) }()
	quote, err := db.GetMeltRequestById(ctx, tx, quoteID)
	if err != nil {
		t.Fatal(err)
	}
	if quote.State != cashu.PAID || !quote.Melted || quote.FeePaid != 10 || quote.PaymentPreimage != "fakewalletpreimage" {
		t.Fatalf("unexpected durable quote: %+v", quote)
	}
	proofs, err := db.GetProofsFromQuote(ctx, tx, quoteID)
	if err != nil {
		t.Fatal(err)
	}
	if len(proofs) != len(expectedProofs) {
		t.Fatalf("proof count=%d, want %d", len(proofs), len(expectedProofs))
	}
	for _, proof := range proofs {
		if proof.State != cashu.PROOF_SPENT {
			t.Fatalf("proof is not spent: %+v", proof)
		}
	}
	if len(quote.Change) != len(expectedChange) {
		t.Fatalf("durable change count=%d, want %d", len(quote.Change), len(expectedChange))
	}
	for i := range expectedChange {
		if quote.Change[i].Amount != expectedChange[i].Amount || quote.Change[i].Id != expectedChange[i].Id || quote.Change[i].C_.ToHex() != expectedChange[i].C_.ToHex() {
			t.Fatalf("durable signature %d differs from workflow result", i)
		}
	}
	blankKeys := make([]cashu.WrappedPublicKey, len(blanks))
	for i := range blanks {
		blankKeys[i] = blanks[i].B_
	}
	restored, err := db.GetRestoreSigsFromBlindedMessages(ctx, tx, blankKeys)
	if err != nil {
		t.Fatal(err)
	}
	if len(restored) != len(expectedChange) {
		t.Fatalf("recovery signatures=%d, want %d", len(restored), len(expectedChange))
	}
}

func assertPostgresMeltSettlementUnchanged(t *testing.T, db postgresql.Postgresql, ctx context.Context, quoteID string, expectedProofs cashu.Proofs, blanks []cashu.BlindedMessage) {
	t.Helper()
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Rollback(context.WithoutCancel(ctx), tx) }()
	quote, err := db.GetMeltRequestById(ctx, tx, quoteID)
	if err != nil {
		t.Fatalf("independent NOWAIT quote read after rollback: %v", err)
	}
	if quote.State != cashu.PENDING || quote.Melted || quote.FeePaid != 0 || quote.PaymentPreimage != "" {
		t.Fatalf("quote changed despite rollback: %+v", quote)
	}
	proofs, err := db.GetProofsFromQuote(ctx, tx, quoteID)
	if err != nil {
		t.Fatal(err)
	}
	if len(proofs) != len(expectedProofs) {
		t.Fatalf("proof count=%d, want %d", len(proofs), len(expectedProofs))
	}
	for i, proof := range proofs {
		if proof.State != cashu.PROOF_PENDING || proof.Secret != expectedProofs[i].Secret {
			t.Fatalf("proof %d changed despite rollback: %+v", i, proof)
		}
	}
	pending, err := db.LoadPendingMeltChangeForUpdate(ctx, tx, quoteID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != len(blanks) {
		t.Fatalf("pending blanks=%d, want %d", len(pending), len(blanks))
	}
	blankKeys := make([]cashu.WrappedPublicKey, len(blanks))
	for i := range blanks {
		blankKeys[i] = blanks[i].B_
	}
	signed, err := db.GetRestoreSigsFromBlindedMessages(ctx, tx, blankKeys)
	if err != nil {
		t.Fatal(err)
	}
	if len(signed) != 0 {
		t.Fatalf("found %d recovery signatures after rollback", len(signed))
	}
}
