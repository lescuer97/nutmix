package mint

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/lightning"
	"github.com/lescuer97/nutmix/internal/signer"
)

// mintPaidEventDB adds the mint-request surface used by mint quote
// reconciliation and internal melt settlement on top of the melt event
// transaction double. It mirrors same-transaction visibility (a state change is
// readable before commit) and lets a test observe the database at commit time.
type mintPaidEventDB struct { //nolint:govet // Test double field order is not performance critical.
	*meltEventDB

	mintQuote      cashu.MintRequestDB
	mintQuoteReads int
	mintQuoteErrAt int
	onCommit       func()
}

func newMintPaidEventDB(mintQuote cashu.MintRequestDB) *mintPaidEventDB {
	return &mintPaidEventDB{ //nolint:exhaustruct // Only the mint quote needs initialising.
		meltEventDB: newMeltEventDB(),
		mintQuote:   mintQuote,
	}
}

func (db *mintPaidEventDB) GetMintRequestById(pgx.Tx, string) (cashu.MintRequestDB, error) {
	return db.mintQuote, nil
}

func (db *mintPaidEventDB) GetMintRequestByRequest(_ pgx.Tx, request string) (cashu.MintRequestDB, error) {
	db.mintQuoteReads++
	if db.mintQuoteErrAt != 0 && db.mintQuoteReads == db.mintQuoteErrAt {
		return cashu.MintRequestDB{}, errors.New("mint request read failed") //nolint:exhaustruct // No request returned.
	}
	if db.mintQuote.Request != request {
		return cashu.MintRequestDB{}, pgx.ErrNoRows //nolint:exhaustruct // No matching request.
	}
	return db.mintQuote, nil
}

func (db *mintPaidEventDB) ChangeMintRequestState(_ pgx.Tx, _ string, state cashu.ACTION_STATE, minted bool) error {
	db.mintQuote.State = state
	db.mintQuote.Minted = minted
	return nil
}

func (db *mintPaidEventDB) Commit(ctx context.Context, tx pgx.Tx) error {
	if db.onCommit != nil {
		db.onCommit()
	}
	return db.meltEventDB.Commit(ctx, tx)
}

func newMintPaidEventMintQuote(quoteId string, state cashu.ACTION_STATE) cashu.MintRequestDB {
	amount := meltEventAmount
	return cashu.MintRequestDB{ //nolint:exhaustruct // Only the fields exercised by the flow are set.
		Quote:   quoteId,
		Request: RegtestRequest,
		Unit:    cashu.Sat.String(),
		State:   state,
		Amount:  &amount,
		Expiry:  time.Now().Add(time.Minute).Unix(),
		SeenAt:  time.Now().Unix(),
	}
}

func newMintPaidEventMint(db database.MintDB, backend lightning.LightningBackend, observer *Observer) *Mint {
	keyset := cashu.BasicKeysetResponse{Id: meltEventKeysetId, Unit: cashu.Sat.String(), InputFeePpk: 0} //nolint:exhaustruct // Only validation fields matter.
	sig := &meltEventSigner{                                                                             //nolint:exhaustruct // Only the keysets are needed.
		keysets: signer.GetKeysetsResponse{Keysets: []cashu.BasicKeysetResponse{keyset}},
	}
	return &Mint{ //nolint:exhaustruct // Only the collaborating pieces are used.
		MintDB:           db,
		Signer:           sig,
		LightningBackend: backend,
		Observer:         observer,
	}
}

func drainMintPaidEventQuotes(ch chan cashu.MintRequestDB) []cashu.MintRequestDB {
	var quotes []cashu.MintRequestDB
	for {
		select {
		case event := <-ch:
			quotes = append(quotes, event)
		default:
			return quotes
		}
	}
}

// TestMintPaidEventsReconcileEmitsPaidAfterCommit verifies that reconciling a
// stale unpaid quote the backend settled publishes a single PAID mint event
// only after the state-change commit.
func TestMintPaidEventsReconcileEmitsPaidAfterCommit(t *testing.T) {
	quote := newMintPaidEventMintQuote("mint-paid-reconcile", cashu.UNPAID)
	db := newMintPaidEventDB(quote)

	observer := newObserverForTest()
	mintCh := make(chan cashu.MintRequestDB, 4)
	observer.AddMintWatch(quote.Quote, MintQuoteChannel{Channel: mintCh}) //nolint:exhaustruct // Subscription ID is irrelevant.

	emittedAtCommit := -1
	db.onCommit = func() { emittedAtCommit = len(mintCh) }

	mint := newMintPaidEventMint(db, newMeltEventSuccessBackend(), &observer)

	reconciled, err := mint.reconcileBolt11MintQuoteState(context.Background(), quote, Bolt11)
	if err != nil {
		t.Fatalf("reconcileBolt11MintQuoteState() error = %v", err)
	}
	if reconciled.State != cashu.PAID {
		t.Fatalf("reconciled state = %q, want PAID", reconciled.State)
	}
	if emittedAtCommit != 0 {
		t.Fatalf("mint event already queued at commit time (len %d), want none", emittedAtCommit)
	}

	events := drainMintPaidEventQuotes(mintCh)
	if len(events) != 1 || events[0].State != cashu.PAID || events[0].Quote != quote.Quote {
		t.Fatalf("mint events = %+v, want a single PAID event for %q", events, quote.Quote)
	}
}

// TestMintPaidEventsReconcileSkipsAlreadyPaid verifies that a stale request for
// a quote that the reloaded row already has as PAID publishes no duplicate
// event.
func TestMintPaidEventsReconcileSkipsAlreadyPaid(t *testing.T) {
	const quoteId = "mint-paid-already"
	stale := newMintPaidEventMintQuote(quoteId, cashu.UNPAID)
	paid := newMintPaidEventMintQuote(quoteId, cashu.PAID)
	db := newMintPaidEventDB(paid)

	observer := newObserverForTest()
	mintCh := make(chan cashu.MintRequestDB, 4)
	observer.AddMintWatch(quoteId, MintQuoteChannel{Channel: mintCh}) //nolint:exhaustruct // Subscription ID is irrelevant.

	mint := newMintPaidEventMint(db, newMeltEventSuccessBackend(), &observer)

	reconciled, err := mint.reconcileBolt11MintQuoteState(context.Background(), stale, Bolt11)
	if err != nil {
		t.Fatalf("reconcileBolt11MintQuoteState() error = %v", err)
	}
	if reconciled.State != cashu.PAID {
		t.Fatalf("reconciled state = %q, want PAID", reconciled.State)
	}
	if events := drainMintPaidEventQuotes(mintCh); len(events) != 0 {
		t.Fatalf("mint events = %+v, want none for an already paid quote", events)
	}
}

// TestMintPaidEventsReconcileNoEventWhenCommitFails verifies that a failed or
// ambiguous state-change commit never advertises the PAID transition.
func TestMintPaidEventsReconcileNoEventWhenCommitFails(t *testing.T) {
	quote := newMintPaidEventMintQuote("mint-paid-reconcile-commit-fail", cashu.UNPAID)
	db := newMintPaidEventDB(quote)
	db.commitErrAt = 1

	observer := newObserverForTest()
	mintCh := make(chan cashu.MintRequestDB, 4)
	observer.AddMintWatch(quote.Quote, MintQuoteChannel{Channel: mintCh}) //nolint:exhaustruct // Subscription ID is irrelevant.

	mint := newMintPaidEventMint(db, newMeltEventSuccessBackend(), &observer)

	if _, err := mint.reconcileBolt11MintQuoteState(context.Background(), quote, Bolt11); err == nil {
		t.Fatal("reconcileBolt11MintQuoteState() error = nil, want commit error")
	}
	if events := drainMintPaidEventQuotes(mintCh); len(events) != 0 {
		t.Fatalf("mint events = %+v, want none after a failed commit", events)
	}
}

// TestMintPaidEventsReconcileNilObserverIsSafe verifies the reconciliation path
// still persists the PAID transition without an observer configured.
func TestMintPaidEventsReconcileNilObserverIsSafe(t *testing.T) {
	quote := newMintPaidEventMintQuote("mint-paid-reconcile-nil", cashu.UNPAID)
	db := newMintPaidEventDB(quote)

	mint := newMintPaidEventMint(db, newMeltEventSuccessBackend(), nil)

	reconciled, err := mint.reconcileBolt11MintQuoteState(context.Background(), quote, Bolt11)
	if err != nil {
		t.Fatalf("reconcileBolt11MintQuoteState() error = %v", err)
	}
	if reconciled.State != cashu.PAID {
		t.Fatalf("reconciled state = %q, want PAID", reconciled.State)
	}
}

// TestMintPaidEventsInternalSettlementEmitsPaidWithMeltEvents verifies that an
// internal melt settlement publishes the mint quote PAID event alongside the
// already committed melt and proof events.
func TestMintPaidEventsInternalSettlementEmitsPaidWithMeltEvents(t *testing.T) {
	meltQuote := newMeltEventQuote("mint-paid-internal", cashu.UNPAID)
	mintQuote := newMintPaidEventMintQuote("mint-paid-internal-mint", cashu.UNPAID)

	db := newMintPaidEventDB(mintQuote)
	db.quote = meltQuote

	proof := newMeltEventProof(t)
	proofY := proof.Y.ToHex()

	observer := newObserverForTest()
	proofCh := make(chan cashu.Proof, 4)
	meltCh := make(chan cashu.MeltRequestDB, 4)
	mintCh := make(chan cashu.MintRequestDB, 4)
	observer.AddProofWatch(proofY, ProofWatchChannel{Channel: proofCh})       //nolint:exhaustruct // Subscription ID is irrelevant.
	observer.AddMeltWatch(meltQuote.Quote, MeltQuoteChannel{Channel: meltCh}) //nolint:exhaustruct // Subscription ID is irrelevant.
	observer.AddMintWatch(mintQuote.Quote, MintQuoteChannel{Channel: mintCh}) //nolint:exhaustruct // Subscription ID is irrelevant.

	mint := newMintPaidEventMint(db, newMeltEventSuccessBackend(), &observer)
	request := cashu.PostMeltBolt11Request{Quote: meltQuote.Quote, Inputs: cashu.Proofs{proof}} //nolint:exhaustruct // Outputs are intentionally empty.

	if _, err := mint.reserveMeltInputsAndMarkPending(context.Background(), request); err != nil {
		t.Fatalf("reserveMeltInputsAndMarkPending() error = %v", err)
	}

	if states := drainMeltEventProofStates(proofCh); !reflect.DeepEqual(states, []cashu.ProofState{cashu.PROOF_PENDING}) {
		t.Fatalf("proof event states = %v, want [PENDING]", states)
	}
	if quotes := drainMeltEventQuotes(meltCh); len(quotes) != 1 || quotes[0].State != cashu.PAID {
		t.Fatalf("melt events = %+v, want a single PAID event", quotes)
	}
	mintEvents := drainMintPaidEventQuotes(mintCh)
	if len(mintEvents) != 1 || mintEvents[0].State != cashu.PAID || mintEvents[0].Quote != mintQuote.Quote {
		t.Fatalf("mint events = %+v, want a single PAID event for %q", mintEvents, mintQuote.Quote)
	}
	if db.quote.State != cashu.PAID {
		t.Fatalf("committed melt state = %q, want PAID", db.quote.State)
	}
	if db.mintQuote.State != cashu.PAID {
		t.Fatalf("committed mint state = %q, want PAID", db.mintQuote.State)
	}
}

// TestMintPaidEventsInternalSettlementNoEventWhenCommitFails verifies that a
// failed reservation commit never publishes the internal mint PAID event.
func TestMintPaidEventsInternalSettlementNoEventWhenCommitFails(t *testing.T) {
	meltQuote := newMeltEventQuote("mint-paid-internal-commit-fail", cashu.UNPAID)
	mintQuote := newMintPaidEventMintQuote("mint-paid-internal-commit-fail-mint", cashu.UNPAID)

	db := newMintPaidEventDB(mintQuote)
	db.quote = meltQuote
	db.commitErrAt = 1

	proof := newMeltEventProof(t)
	proofY := proof.Y.ToHex()

	observer := newObserverForTest()
	proofCh := make(chan cashu.Proof, 4)
	meltCh := make(chan cashu.MeltRequestDB, 4)
	mintCh := make(chan cashu.MintRequestDB, 4)
	observer.AddProofWatch(proofY, ProofWatchChannel{Channel: proofCh})       //nolint:exhaustruct // Subscription ID is irrelevant.
	observer.AddMeltWatch(meltQuote.Quote, MeltQuoteChannel{Channel: meltCh}) //nolint:exhaustruct // Subscription ID is irrelevant.
	observer.AddMintWatch(mintQuote.Quote, MintQuoteChannel{Channel: mintCh}) //nolint:exhaustruct // Subscription ID is irrelevant.

	mint := newMintPaidEventMint(db, newMeltEventSuccessBackend(), &observer)
	request := cashu.PostMeltBolt11Request{Quote: meltQuote.Quote, Inputs: cashu.Proofs{proof}} //nolint:exhaustruct // Outputs are intentionally empty.

	if _, err := mint.reserveMeltInputsAndMarkPending(context.Background(), request); err == nil {
		t.Fatal("reserveMeltInputsAndMarkPending() error = nil, want commit error")
	}

	if events := drainMintPaidEventQuotes(mintCh); len(events) != 0 {
		t.Fatalf("mint events = %+v, want none after a failed commit", events)
	}
	if events := drainMeltEventQuotes(meltCh); len(events) != 0 {
		t.Fatalf("melt events = %+v, want none after a failed commit", events)
	}
}

// TestMintPaidEventsInternalSettlementSnapshotFailureRollsBack verifies that a
// failed pre-commit mint snapshot aborts the reservation: the internal-only
// settlement publishes no mint, melt or proof event.
func TestMintPaidEventsInternalSettlementSnapshotFailureRollsBack(t *testing.T) {
	meltQuote := newMeltEventQuote("mint-paid-internal-read-fail", cashu.UNPAID)
	mintQuote := newMintPaidEventMintQuote("mint-paid-internal-read-fail-mint", cashu.UNPAID)

	db := newMintPaidEventDB(mintQuote)
	db.quote = meltQuote
	db.mintQuoteErrAt = 2

	proof := newMeltEventProof(t)

	observer := newObserverForTest()
	meltCh := make(chan cashu.MeltRequestDB, 4)
	mintCh := make(chan cashu.MintRequestDB, 4)
	observer.AddMeltWatch(meltQuote.Quote, MeltQuoteChannel{Channel: meltCh}) //nolint:exhaustruct // Subscription ID is irrelevant.
	observer.AddMintWatch(mintQuote.Quote, MintQuoteChannel{Channel: mintCh}) //nolint:exhaustruct // Subscription ID is irrelevant.

	mint := newMintPaidEventMint(db, newMeltEventSuccessBackend(), &observer)
	request := cashu.PostMeltBolt11Request{Quote: meltQuote.Quote, Inputs: cashu.Proofs{proof}} //nolint:exhaustruct // Outputs are intentionally empty.

	if _, err := mint.reserveMeltInputsAndMarkPending(context.Background(), request); err == nil {
		t.Fatal("reserveMeltInputsAndMarkPending() error = nil, want snapshot read error")
	}
	if events := drainMintPaidEventQuotes(mintCh); len(events) != 0 {
		t.Fatalf("mint events = %+v, want none after a failed snapshot", events)
	}
	if events := drainMeltEventQuotes(meltCh); len(events) != 0 {
		t.Fatalf("melt events = %+v, want none after a failed snapshot", events)
	}
}

// TestMintPaidEventsInternalSettlementNilObserverSkipsExtraRead verifies that
// the internal settlement still pays the mint quote without an observer, and
// that the pre-commit snapshot read is skipped when nobody is listening.
func TestMintPaidEventsInternalSettlementNilObserverSkipsExtraRead(t *testing.T) {
	meltQuote := newMeltEventQuote("mint-paid-internal-nil", cashu.UNPAID)
	mintQuote := newMintPaidEventMintQuote("mint-paid-internal-nil-mint", cashu.UNPAID)

	db := newMintPaidEventDB(mintQuote)
	db.quote = meltQuote

	proof := newMeltEventProof(t)

	mint := newMintPaidEventMint(db, newMeltEventSuccessBackend(), nil)
	request := cashu.PostMeltBolt11Request{Quote: meltQuote.Quote, Inputs: cashu.Proofs{proof}} //nolint:exhaustruct // Outputs are intentionally empty.

	quote, err := mint.reserveMeltInputsAndMarkPending(context.Background(), request)
	if err != nil {
		t.Fatalf("reserveMeltInputsAndMarkPending() error = %v", err)
	}
	if quote.State != cashu.PAID {
		t.Fatalf("reserve state = %q, want PAID", quote.State)
	}
	if db.mintQuoteReads != 1 {
		t.Fatalf("mint request reads = %d, want 1 (settlement only)", db.mintQuoteReads)
	}
	if db.mintQuote.State != cashu.PAID {
		t.Fatalf("committed mint state = %q, want PAID", db.mintQuote.State)
	}
}
