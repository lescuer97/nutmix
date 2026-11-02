package mint

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/lightning"
	"github.com/lescuer97/nutmix/internal/signer"
	"github.com/lightningnetwork/lnd/zpay32"
)

const meltEventKeysetId = "melt-event-keyset"

// meltEventAmount is the invoice amount shared by the event-ordering fixtures.
const meltEventAmount uint64 = 8

// meltEventDB is a Docker-independent transaction double for the melt flow. It
// stages mutations and only applies them when Commit succeeds, mirroring the
// transaction boundaries the melt code relies on so event ordering can be
// asserted against the committed view rather than against staged writes.
type meltEventDB struct { //nolint:govet // Test double field order is not performance critical.
	database.MintDB

	commits     int
	commitErrAt int
	rollbackErr error

	quote        cashu.MeltRequestDB
	proofs       map[string]cashu.Proof
	deleted      map[string]bool
	mintRequests []cashu.MintRequestDB
	change       map[string][]cashu.BlindedMessage

	staged []func()
}

func newMeltEventDB() *meltEventDB {
	return &meltEventDB{ //nolint:exhaustruct // Only the maps need initialising.
		proofs:  make(map[string]cashu.Proof),
		deleted: make(map[string]bool),
		change:  make(map[string][]cashu.BlindedMessage),
	}
}

// meltEventTx routes the direct pgx.Tx Commit/Rollback calls made by the melt
// code back into the fake database so commit ordering is tracked consistently.
type meltEventTx struct {
	pgx.Tx
	db *meltEventDB
}

func (tx *meltEventTx) Commit(ctx context.Context) error   { return tx.db.Commit(ctx, tx) }
func (tx *meltEventTx) Rollback(ctx context.Context) error { return tx.db.Rollback(ctx, tx) }

func (db *meltEventDB) GetTx(context.Context) (pgx.Tx, error) {
	db.staged = nil
	return &meltEventTx{db: db}, nil //nolint:exhaustruct // Only the routing database matters.
}

func (db *meltEventDB) Commit(context.Context, pgx.Tx) error {
	db.commits++
	if db.commitErrAt != 0 && db.commits == db.commitErrAt {
		return errors.New("commit failed")
	}
	for _, op := range db.staged {
		op()
	}
	db.staged = nil
	return nil
}

func (db *meltEventDB) Rollback(context.Context, pgx.Tx) error {
	db.staged = nil
	return db.rollbackErr
}

func (db *meltEventDB) GetMeltRequestById(pgx.Tx, string) (cashu.MeltRequestDB, error) {
	return db.quote, nil
}

func (db *meltEventDB) GetMintRequestByRequest(pgx.Tx, string) (cashu.MintRequestDB, error) {
	for _, request := range db.mintRequests {
		return request, nil
	}
	return cashu.MintRequestDB{}, pgx.ErrNoRows //nolint:exhaustruct // No matching request.
}

func (db *meltEventDB) GetProofsFromQuote(_ pgx.Tx, quote string) (cashu.Proofs, error) {
	proofs := cashu.Proofs{}
	for _, proof := range db.proofs {
		if db.deleted[proof.Y.ToHex()] {
			continue
		}
		if proof.Quote != nil && *proof.Quote == quote {
			proofs = append(proofs, proof)
		}
	}
	return proofs, nil
}

func (db *meltEventDB) GetProofsFromSecretCurve(_ pgx.Tx, ys []cashu.WrappedPublicKey) (cashu.Proofs, error) {
	proofs := cashu.Proofs{}
	for _, y := range ys {
		key := y.ToHex()
		if db.deleted[key] {
			continue
		}
		if proof, exists := db.proofs[key]; exists {
			proofs = append(proofs, proof)
		}
	}
	return proofs, nil
}

func (db *meltEventDB) GetRestoreSigsFromBlindedMessages(pgx.Tx, []cashu.WrappedPublicKey) ([]cashu.RecoverSigDB, error) {
	return nil, nil
}

func (db *meltEventDB) GetMeltChangeByQuote(_ pgx.Tx, quote string) ([]cashu.MeltChange, error) {
	messages := db.change[quote]
	change := make([]cashu.MeltChange, 0, len(messages))
	for _, message := range messages {
		change = append(change, cashu.MeltChange{B_: message.B_, Id: message.Id, Quote: quote, CreatedAt: 0}) //nolint:exhaustruct // Only identity fields matter.
	}
	return change, nil
}

func (db *meltEventDB) SaveProof(_ pgx.Tx, proofs []cashu.Proof) error {
	staged := append([]cashu.Proof(nil), proofs...)
	db.staged = append(db.staged, func() {
		for _, proof := range staged {
			key := proof.Y.ToHex()
			db.proofs[key] = proof
			delete(db.deleted, key)
		}
	})
	return nil
}

func (db *meltEventDB) SetProofsState(_ pgx.Tx, proofs cashu.Proofs, state cashu.ProofState) error {
	staged := append(cashu.Proofs(nil), proofs...)
	db.staged = append(db.staged, func() {
		for _, proof := range staged {
			key := proof.Y.ToHex()
			existing, exists := db.proofs[key]
			if !exists {
				continue
			}
			existing.State = state
			db.proofs[key] = existing
		}
	})
	return nil
}

func (db *meltEventDB) DeleteProofs(_ pgx.Tx, proofs cashu.Proofs) error {
	staged := append(cashu.Proofs(nil), proofs...)
	db.staged = append(db.staged, func() {
		for _, proof := range staged {
			key := proof.Y.ToHex()
			delete(db.proofs, key)
			db.deleted[key] = true
		}
	})
	return nil
}

func (db *meltEventDB) ChangeMeltRequestState(_ pgx.Tx, _ string, state cashu.ACTION_STATE, melted bool, feePaid uint64) error {
	db.staged = append(db.staged, func() {
		db.quote.State = state
		db.quote.Melted = melted
		db.quote.FeePaid = feePaid
	})
	return nil
}

func (db *meltEventDB) AddPreimageMeltRequest(_ pgx.Tx, _ string, preimage string) error {
	db.staged = append(db.staged, func() { db.quote.PaymentPreimage = preimage })
	return nil
}

func (db *meltEventDB) ChangeCheckingId(_ pgx.Tx, _ string, checkingId string) error {
	db.staged = append(db.staged, func() { db.quote.CheckingId = checkingId })
	return nil
}

func (db *meltEventDB) SaveMeltChange(_ pgx.Tx, change []cashu.BlindedMessage, quote string) error {
	staged := append([]cashu.BlindedMessage(nil), change...)
	db.staged = append(db.staged, func() { db.change[quote] = append(db.change[quote], staged...) })
	return nil
}

func (db *meltEventDB) DeleteChangeByQuote(_ pgx.Tx, quote string) error {
	db.staged = append(db.staged, func() { delete(db.change, quote) })
	return nil
}

func (db *meltEventDB) SaveRestoreSigs(pgx.Tx, []cashu.RecoverSigDB) error { return nil }

// meltEventSigner is a Docker-independent signer double. The embedded Signer
// makes any unexpected call fail loudly instead of silently succeeding.
type meltEventSigner struct {
	signer.Signer
	keysets signer.GetKeysetsResponse
}

func (s *meltEventSigner) GetKeysets() (signer.GetKeysetsResponse, error) {
	return s.keysets, nil
}

func (s *meltEventSigner) VerifyProofs([]cashu.Proof) error {
	return nil
}

// meltEventBackend wraps the fake wallet so a test can observe the committed
// database state at the moment the backend is asked to pay.
type meltEventBackend struct { //nolint:govet // Test double field order is not performance critical.
	lightning.FakeWallet
	onPayInvoice func()
}

func (b meltEventBackend) PayInvoice(meltQuote cashu.MeltRequestDB, zpayInvoice *zpay32.Invoice, feeReserve cashu.Amount, mpp bool, amount cashu.Amount) (lightning.PaymentResponse, error) {
	if b.onPayInvoice != nil {
		b.onPayInvoice()
	}
	return b.FakeWallet.PayInvoice(meltQuote, zpayInvoice, feeReserve, mpp, amount)
}

func newMeltEventProof(t *testing.T) cashu.Proof {
	t.Helper()

	cPriv, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		t.Fatalf("secp256k1.GeneratePrivateKey(): %v", err)
	}

	// A plain (non P2PK/HTLC) secret built at runtime so it is not mistaken for
	// a hardcoded credential.
	proof := cashu.Proof{ //nolint:exhaustruct // Only the fields exercised by the melt flow are set.
		Id:     meltEventKeysetId,
		Amount: meltEventAmount,
		Secret: strings.Repeat("cd", 32),
		C:      cashu.WrappedPublicKey{PublicKey: cPriv.PubKey()},
		State:  cashu.PROOF_UNSPENT,
	}
	hashed, err := proof.HashSecretToCurve()
	if err != nil {
		t.Fatalf("proof.HashSecretToCurve(): %v", err)
	}
	return hashed
}

func newMeltEventQuote(quoteId string, state cashu.ACTION_STATE) cashu.MeltRequestDB {
	return cashu.MeltRequestDB{ //nolint:exhaustruct // Remaining fields are irrelevant to event ordering.
		Quote:      quoteId,
		Request:    RegtestRequest,
		Unit:       cashu.Sat.String(),
		State:      state,
		Amount:     meltEventAmount,
		FeeReserve: 0,
		Expiry:     time.Now().Add(time.Minute).Unix(),
		SeenAt:     time.Now().Unix(),
	}
}

func newMeltEventMint(db *meltEventDB, backend lightning.LightningBackend, observer *Observer) *Mint {
	keyset := cashu.BasicKeysetResponse{Id: meltEventKeysetId, Unit: cashu.Sat.String(), InputFeePpk: 0} //nolint:exhaustruct // Only the fields used for validation are set.
	sig := &meltEventSigner{ //nolint:exhaustruct // Only the keysets are needed.
		keysets: signer.GetKeysetsResponse{Keysets: []cashu.BasicKeysetResponse{keyset}},
	}
	return &Mint{ //nolint:exhaustruct // Only the collaborating pieces are used.
		MintDB:           db,
		Signer:           sig,
		LightningBackend: backend,
		Observer:         observer,
	}
}

func newMeltEventSuccessBackend() meltEventBackend {
	return meltEventBackend{ //nolint:exhaustruct // Only network and status matter for the fake wallet.
		FakeWallet: lightning.FakeWallet{Network: chaincfg.RegressionNetParams, NodeStatus: lightning.ONLINE_STATUS}, //nolint:exhaustruct // Only network and status matter.
	}
}

func drainMeltEventProofStates(ch chan cashu.Proof) []cashu.ProofState {
	var states []cashu.ProofState
	for {
		select {
		case event := <-ch:
			states = append(states, event.State)
		default:
			return states
		}
	}
}

func drainMeltEventQuotes(ch chan cashu.MeltRequestDB) []cashu.MeltRequestDB {
	var quotes []cashu.MeltRequestDB
	for {
		select {
		case event := <-ch:
			quotes = append(quotes, event)
		default:
			return quotes
		}
	}
}

func drainMeltEventQuoteStates(ch chan cashu.MeltRequestDB) []cashu.ACTION_STATE {
	var states []cashu.ACTION_STATE
	for {
		select {
		case event := <-ch:
			states = append(states, event.State)
		default:
			return states
		}
	}
}

// TestMeltEventsEmitsPendingBeforePaymentThenSpent verifies the happy path: the
// PENDING proof and melt events are delivered after the reservation commit and
// before the backend is asked to pay, and SPENT/PAID are delivered once the
// finalization commit succeeds.
func TestMeltEventsEmitsPendingBeforePaymentThenSpent(t *testing.T) {
	db := newMeltEventDB()
	quote := newMeltEventQuote("melt-events-success", cashu.UNPAID)
	db.quote = quote

	proof := newMeltEventProof(t)
	proofY := proof.Y.ToHex()

	observer := newObserverForTest()
	proofCh := make(chan cashu.Proof, 4)
	meltCh := make(chan cashu.MeltRequestDB, 4)
	observer.AddProofWatch(proofY, ProofWatchChannel{Channel: proofCh})    //nolint:exhaustruct // Subscription ID is irrelevant.
	observer.AddMeltWatch(quote.Quote, MeltQuoteChannel{Channel: meltCh}) //nolint:exhaustruct // Subscription ID is irrelevant.

	var proofAtPayment *cashu.Proof
	var meltAtPayment *cashu.MeltRequestDB
	var proofStateAtPayment cashu.ProofState
	var quoteStateAtPayment cashu.ACTION_STATE
	var deletedAtPayment bool

	backend := newMeltEventSuccessBackend()
	backend.onPayInvoice = func() {
		select {
		case event := <-proofCh:
			proofAtPayment = &event
		default:
			t.Errorf("no pending proof event before the backend payment")
		}
		select {
		case event := <-meltCh:
			meltAtPayment = &event
		default:
			t.Errorf("no pending melt event before the backend payment")
		}
		proofStateAtPayment = db.proofs[proofY].State
		quoteStateAtPayment = db.quote.State
		deletedAtPayment = db.deleted[proofY]
	}

	mint := newMeltEventMint(db, backend, &observer)
	request := cashu.PostMeltBolt11Request{Quote: quote.Quote, Inputs: cashu.Proofs{proof}} //nolint:exhaustruct // Outputs are intentionally empty.

	if _, _, err := mint.bolt11Melt(context.Background(), request); err != nil {
		t.Fatalf("bolt11Melt() error = %v", err)
	}

	if proofAtPayment == nil || proofAtPayment.State != cashu.PROOF_PENDING {
		t.Fatalf("proof event before payment = %+v, want PENDING", proofAtPayment)
	}
	if meltAtPayment == nil || meltAtPayment.State != cashu.PENDING {
		t.Fatalf("melt event before payment = %+v, want PENDING", meltAtPayment)
	}
	if proofStateAtPayment != cashu.PROOF_PENDING {
		t.Fatalf("committed proof state before payment = %q, want PENDING", proofStateAtPayment)
	}
	if quoteStateAtPayment != cashu.PENDING {
		t.Fatalf("committed quote state before payment = %q, want PENDING", quoteStateAtPayment)
	}
	if deletedAtPayment {
		t.Fatal("proof was deleted before the backend payment")
	}

	if states := drainMeltEventProofStates(proofCh); !reflect.DeepEqual(states, []cashu.ProofState{cashu.PROOF_SPENT}) {
		t.Fatalf("proof event states after finalization = %v, want [SPENT]", states)
	}
	if states := drainMeltEventQuoteStates(meltCh); !reflect.DeepEqual(states, []cashu.ACTION_STATE{cashu.PAID}) {
		t.Fatalf("melt event states after finalization = %v, want [PAID]", states)
	}
	if db.proofs[proofY].State != cashu.PROOF_SPENT {
		t.Fatalf("committed proof state = %q, want SPENT", db.proofs[proofY].State)
	}
	if db.quote.State != cashu.PAID {
		t.Fatalf("committed quote state = %q, want PAID", db.quote.State)
	}
}

// TestMeltEventsReleasesProofsWhenPaymentConfirmedFailed verifies that a
// confirmed payment failure releases the reserved proofs: UNSPENT and UNPAID
// are delivered after the cleanup commit and no SPENT/PAID event is emitted.
func TestMeltEventsReleasesProofsWhenPaymentConfirmedFailed(t *testing.T) {
	db := newMeltEventDB()
	quote := newMeltEventQuote("melt-events-failed", cashu.UNPAID)
	db.quote = quote

	proof := newMeltEventProof(t)
	proofY := proof.Y.ToHex()

	observer := newObserverForTest()
	proofCh := make(chan cashu.Proof, 4)
	meltCh := make(chan cashu.MeltRequestDB, 4)
	observer.AddProofWatch(proofY, ProofWatchChannel{Channel: proofCh})    //nolint:exhaustruct // Subscription ID is irrelevant.
	observer.AddMeltWatch(quote.Quote, MeltQuoteChannel{Channel: meltCh}) //nolint:exhaustruct // Subscription ID is irrelevant.

	backend := meltEventBackend{ //nolint:exhaustruct // Only the fake wallet errors matter.
		FakeWallet: lightning.FakeWallet{ //nolint:exhaustruct // Only the fields exercised by the failure path are set.
			Network:         chaincfg.RegressionNetParams,
			NodeStatus:      lightning.ONLINE_STATUS,
			UnpurposeErrors: []lightning.FakeWalletError{lightning.FailPaymentFailed, lightning.FailQueryFailed},
		},
	}
	mint := newMeltEventMint(db, backend, &observer)
	request := cashu.PostMeltBolt11Request{Quote: quote.Quote, Inputs: cashu.Proofs{proof}} //nolint:exhaustruct // Outputs are intentionally empty.

	if _, _, err := mint.bolt11Melt(context.Background(), request); err != nil {
		t.Fatalf("bolt11Melt() error = %v", err)
	}

	if states := drainMeltEventProofStates(proofCh); !reflect.DeepEqual(states, []cashu.ProofState{cashu.PROOF_PENDING, cashu.PROOF_UNSPENT}) {
		t.Fatalf("proof event states = %v, want [PENDING UNSPENT]", states)
	}
	if states := drainMeltEventQuoteStates(meltCh); !reflect.DeepEqual(states, []cashu.ACTION_STATE{cashu.PENDING, cashu.UNPAID}) {
		t.Fatalf("melt event states = %v, want [PENDING UNPAID]", states)
	}
	if _, exists := db.proofs[proofY]; exists {
		t.Fatal("proof still committed after confirmed failure")
	}
	if !db.deleted[proofY] {
		t.Fatal("proof was not deleted after confirmed failure")
	}
	if db.quote.State != cashu.UNPAID {
		t.Fatalf("committed quote state = %q, want UNPAID", db.quote.State)
	}
}

// TestMeltEventsNoEventWhenReservationCommitFails verifies that no proof or melt
// event is advertised when the reservation transaction cannot be committed.
func TestMeltEventsNoEventWhenReservationCommitFails(t *testing.T) {
	db := newMeltEventDB()
	db.commitErrAt = 2
	quote := newMeltEventQuote("melt-events-reserve-commit-fail", cashu.UNPAID)
	db.quote = quote

	proof := newMeltEventProof(t)
	proofY := proof.Y.ToHex()

	observer := newObserverForTest()
	proofCh := make(chan cashu.Proof, 4)
	meltCh := make(chan cashu.MeltRequestDB, 4)
	observer.AddProofWatch(proofY, ProofWatchChannel{Channel: proofCh})    //nolint:exhaustruct // Subscription ID is irrelevant.
	observer.AddMeltWatch(quote.Quote, MeltQuoteChannel{Channel: meltCh}) //nolint:exhaustruct // Subscription ID is irrelevant.

	mint := newMeltEventMint(db, newMeltEventSuccessBackend(), &observer)
	request := cashu.PostMeltBolt11Request{Quote: quote.Quote, Inputs: cashu.Proofs{proof}} //nolint:exhaustruct // Outputs are intentionally empty.

	if _, _, err := mint.bolt11Melt(context.Background(), request); err == nil {
		t.Fatal("bolt11Melt() error = nil, want reservation commit error")
	}

	if events := drainMeltEventProofStates(proofCh); len(events) != 0 {
		t.Fatalf("proof events = %v, want none", events)
	}
	if events := drainMeltEventQuoteStates(meltCh); len(events) != 0 {
		t.Fatalf("melt events = %v, want none", events)
	}
}

// TestMeltEventsNoUnspentWhenCleanupCommitFails verifies that a failed cleanup
// commit never advertises UNSPENT: the proofs may still be reserved in the
// database.
func TestMeltEventsNoUnspentWhenCleanupCommitFails(t *testing.T) {
	db := newMeltEventDB()
	db.commitErrAt = 4
	quote := newMeltEventQuote("melt-events-cleanup-commit-fail", cashu.UNPAID)
	db.quote = quote

	proof := newMeltEventProof(t)
	proofY := proof.Y.ToHex()

	observer := newObserverForTest()
	proofCh := make(chan cashu.Proof, 4)
	meltCh := make(chan cashu.MeltRequestDB, 4)
	observer.AddProofWatch(proofY, ProofWatchChannel{Channel: proofCh})    //nolint:exhaustruct // Subscription ID is irrelevant.
	observer.AddMeltWatch(quote.Quote, MeltQuoteChannel{Channel: meltCh}) //nolint:exhaustruct // Subscription ID is irrelevant.

	backend := meltEventBackend{ //nolint:exhaustruct // Only the fake wallet errors matter.
		FakeWallet: lightning.FakeWallet{ //nolint:exhaustruct // Only the fields exercised by the failure path are set.
			Network:         chaincfg.RegressionNetParams,
			NodeStatus:      lightning.ONLINE_STATUS,
			UnpurposeErrors: []lightning.FakeWalletError{lightning.FailPaymentFailed, lightning.FailQueryFailed},
		},
	}
	mint := newMeltEventMint(db, backend, &observer)
	request := cashu.PostMeltBolt11Request{Quote: quote.Quote, Inputs: cashu.Proofs{proof}} //nolint:exhaustruct // Outputs are intentionally empty.

	if _, _, err := mint.bolt11Melt(context.Background(), request); err == nil {
		t.Fatal("bolt11Melt() error = nil, want cleanup commit error")
	}

	if states := drainMeltEventProofStates(proofCh); !reflect.DeepEqual(states, []cashu.ProofState{cashu.PROOF_PENDING}) {
		t.Fatalf("proof event states = %v, want [PENDING]", states)
	}
	if states := drainMeltEventQuoteStates(meltCh); !reflect.DeepEqual(states, []cashu.ACTION_STATE{cashu.PENDING}) {
		t.Fatalf("melt event states = %v, want [PENDING]", states)
	}
	if _, exists := db.proofs[proofY]; !exists {
		t.Fatal("proof unexpectedly removed although cleanup commit failed")
	}
	if db.quote.State != cashu.PENDING {
		t.Fatalf("committed quote state = %q, want untouched PENDING", db.quote.State)
	}
}

// TestMeltEventsNoTerminalEventWhenFinalizeCommitFails verifies that a failed
// finalization commit emits no SPENT/PAID terminal event.
func TestMeltEventsNoTerminalEventWhenFinalizeCommitFails(t *testing.T) {
	db := newMeltEventDB()
	db.commitErrAt = 3
	quote := newMeltEventQuote("melt-events-finalize-commit-fail", cashu.UNPAID)
	db.quote = quote

	proof := newMeltEventProof(t)
	proofY := proof.Y.ToHex()

	observer := newObserverForTest()
	proofCh := make(chan cashu.Proof, 4)
	meltCh := make(chan cashu.MeltRequestDB, 4)
	observer.AddProofWatch(proofY, ProofWatchChannel{Channel: proofCh})    //nolint:exhaustruct // Subscription ID is irrelevant.
	observer.AddMeltWatch(quote.Quote, MeltQuoteChannel{Channel: meltCh}) //nolint:exhaustruct // Subscription ID is irrelevant.

	mint := newMeltEventMint(db, newMeltEventSuccessBackend(), &observer)
	request := cashu.PostMeltBolt11Request{Quote: quote.Quote, Inputs: cashu.Proofs{proof}} //nolint:exhaustruct // Outputs are intentionally empty.

	if _, _, err := mint.bolt11Melt(context.Background(), request); err == nil {
		t.Fatal("bolt11Melt() error = nil, want finalize commit error")
	}

	if states := drainMeltEventProofStates(proofCh); !reflect.DeepEqual(states, []cashu.ProofState{cashu.PROOF_PENDING}) {
		t.Fatalf("proof event states = %v, want [PENDING]", states)
	}
	if states := drainMeltEventQuoteStates(meltCh); !reflect.DeepEqual(states, []cashu.ACTION_STATE{cashu.PENDING}) {
		t.Fatalf("melt event states = %v, want [PENDING]", states)
	}
	if db.proofs[proofY].State != cashu.PROOF_PENDING {
		t.Fatalf("committed proof state = %q, want PENDING", db.proofs[proofY].State)
	}
	if db.quote.State != cashu.PENDING {
		t.Fatalf("committed quote state = %q, want PENDING", db.quote.State)
	}
}

// TestMeltEventsNilObserverIsSafe verifies the baseline: the melt flow completes
// and persists its terminal state without an observer configured.
func TestMeltEventsNilObserverIsSafe(t *testing.T) {
	db := newMeltEventDB()
	quote := newMeltEventQuote("melt-events-nil-observer", cashu.UNPAID)
	db.quote = quote

	proof := newMeltEventProof(t)
	proofY := proof.Y.ToHex()

	mint := newMeltEventMint(db, newMeltEventSuccessBackend(), nil)
	request := cashu.PostMeltBolt11Request{Quote: quote.Quote, Inputs: cashu.Proofs{proof}} //nolint:exhaustruct // Outputs are intentionally empty.

	if _, _, err := mint.bolt11Melt(context.Background(), request); err != nil {
		t.Fatalf("bolt11Melt() error = %v", err)
	}
	if db.quote.State != cashu.PAID {
		t.Fatalf("committed quote state = %q, want PAID", db.quote.State)
	}
	if db.proofs[proofY].State != cashu.PROOF_SPENT {
		t.Fatalf("committed proof state = %q, want SPENT", db.proofs[proofY].State)
	}
}

// TestRefreshMeltEventsEmitsSpentAndPaidOnSettlement verifies that reconciling a
// PENDING quote that the backend settled emits SPENT and a PAID melt event after
// the settlement commit.
func TestRefreshMeltEventsEmitsSpentAndPaidOnSettlement(t *testing.T) {
	db := newMeltEventDB()
	quote := newMeltEventQuote("refresh-melt-settled", cashu.PENDING)
	db.quote = quote

	proof := newMeltEventProof(t)
	proof.Quote = &quote.Quote
	proof.State = cashu.PROOF_PENDING
	db.proofs[proof.Y.ToHex()] = proof

	observer := newObserverForTest()
	proofCh := make(chan cashu.Proof, 4)
	meltCh := make(chan cashu.MeltRequestDB, 4)
	observer.AddProofWatch(proof.Y.ToHex(), ProofWatchChannel{Channel: proofCh}) //nolint:exhaustruct // Subscription ID is irrelevant.
	observer.AddMeltWatch(quote.Quote, MeltQuoteChannel{Channel: meltCh})        //nolint:exhaustruct // Subscription ID is irrelevant.

	mint := newMeltEventMint(db, newMeltEventSuccessBackend(), &observer)

	refreshed, err := mint.RefreshMeltQuoteState(context.Background(), quote.Quote)
	if err != nil {
		t.Fatalf("RefreshMeltQuoteState() error = %v", err)
	}
	if refreshed.State != cashu.PAID || !refreshed.Melted {
		t.Fatalf("refreshed quote = %+v, want PAID and melted", refreshed)
	}

	if states := drainMeltEventProofStates(proofCh); !reflect.DeepEqual(states, []cashu.ProofState{cashu.PROOF_SPENT}) {
		t.Fatalf("proof event states = %v, want [SPENT]", states)
	}
	quotes := drainMeltEventQuotes(meltCh)
	if len(quotes) != 1 || quotes[0].State != cashu.PAID || !quotes[0].Melted {
		t.Fatalf("melt events = %+v, want a single PAID melted event", quotes)
	}
	if db.quote.State != cashu.PAID {
		t.Fatalf("committed quote state = %q, want PAID", db.quote.State)
	}
	if db.proofs[proof.Y.ToHex()].State != cashu.PROOF_SPENT {
		t.Fatalf("committed proof state = %q, want SPENT", db.proofs[proof.Y.ToHex()].State)
	}
}

// TestRefreshMeltEventsEmitsUnspentAndUnpaidOnFailure verifies that reconciling a
// PENDING quote the backend failed emits UNSPENT and an UNPAID melt event after
// the cleanup commit.
func TestRefreshMeltEventsEmitsUnspentAndUnpaidOnFailure(t *testing.T) {
	db := newMeltEventDB()
	quote := newMeltEventQuote("refresh-melt-failed", cashu.PENDING)
	db.quote = quote

	proof := newMeltEventProof(t)
	proof.Quote = &quote.Quote
	proof.State = cashu.PROOF_PENDING
	proofY := proof.Y.ToHex()
	db.proofs[proofY] = proof

	observer := newObserverForTest()
	proofCh := make(chan cashu.Proof, 4)
	meltCh := make(chan cashu.MeltRequestDB, 4)
	observer.AddProofWatch(proofY, ProofWatchChannel{Channel: proofCh})    //nolint:exhaustruct // Subscription ID is irrelevant.
	observer.AddMeltWatch(quote.Quote, MeltQuoteChannel{Channel: meltCh}) //nolint:exhaustruct // Subscription ID is irrelevant.

	backend := meltEventBackend{ //nolint:exhaustruct // Only the fake wallet errors matter.
		FakeWallet: lightning.FakeWallet{ //nolint:exhaustruct // Only the fields exercised by the failure path are set.
			Network:         chaincfg.RegressionNetParams,
			NodeStatus:      lightning.ONLINE_STATUS,
			UnpurposeErrors: []lightning.FakeWalletError{lightning.FailQueryFailed},
		},
	}
	mint := newMeltEventMint(db, backend, &observer)

	refreshed, err := mint.RefreshMeltQuoteState(context.Background(), quote.Quote)
	if err != nil {
		t.Fatalf("RefreshMeltQuoteState() error = %v", err)
	}
	if refreshed.State != cashu.UNPAID {
		t.Fatalf("refreshed quote state = %q, want UNPAID", refreshed.State)
	}

	if states := drainMeltEventProofStates(proofCh); !reflect.DeepEqual(states, []cashu.ProofState{cashu.PROOF_UNSPENT}) {
		t.Fatalf("proof event states = %v, want [UNSPENT]", states)
	}
	quotes := drainMeltEventQuotes(meltCh)
	if len(quotes) != 1 || quotes[0].State != cashu.UNPAID {
		t.Fatalf("melt events = %+v, want a single UNPAID event", quotes)
	}
	if _, exists := db.proofs[proofY]; exists {
		t.Fatal("proof still committed after failed refresh")
	}
	if db.quote.State != cashu.UNPAID {
		t.Fatalf("committed quote state = %q, want UNPAID", db.quote.State)
	}
}
