package mint

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/signer"
)

const swapEventKeysetId = "swap-event-keyset"

// swapEventOp is a single staged database mutation. Operations are only applied
// to the committed view when Commit succeeds, so the fake mirrors the
// transaction boundaries the swap code relies on.
type swapEventOp struct { //nolint:govet // Test double field order is not performance critical.
	kind  string
	keys  []string
	state cashu.ProofState
}

// swapEventDB is a Docker-independent transaction double. It embeds the MintDB
// interface so unimplemented methods fail loudly if the code under test starts
// calling them.
type swapEventDB struct { //nolint:govet // Test double field order is not performance critical.
	database.MintDB
	commits            int
	commitErrAt        int
	rollbackErr        error
	saveRestoreSigsErr error
	deleteErr          error

	committed map[string]cashu.ProofState
	deleted   map[string]bool
	staged    []swapEventOp
}

func newSwapEventDB() *swapEventDB {
	return &swapEventDB{ //nolint:exhaustruct // Only the maps need initialising.
		committed: make(map[string]cashu.ProofState),
		deleted:   make(map[string]bool),
	}
}

// swapEventTx routes the direct pgx.Tx Commit/Rollback calls made by ExecuteSwap
// back into the fake database so commit ordering is tracked consistently.
type swapEventTx struct {
	pgx.Tx
	db *swapEventDB
}

func (tx *swapEventTx) Commit(ctx context.Context) error {
	return tx.db.Commit(ctx, tx)
}

func (tx *swapEventTx) Rollback(ctx context.Context) error {
	return tx.db.Rollback(ctx, tx)
}

func (db *swapEventDB) GetTx(context.Context) (pgx.Tx, error) {
	db.staged = nil
	return &swapEventTx{db: db}, nil //nolint:exhaustruct // Only the routing database matters.
}

func (db *swapEventDB) Commit(context.Context, pgx.Tx) error {
	db.commits++
	if db.commitErrAt != 0 && db.commits == db.commitErrAt {
		return errors.New("commit failed")
	}
	for _, op := range db.staged {
		db.apply(op)
	}
	db.staged = nil
	return nil
}

func (db *swapEventDB) Rollback(context.Context, pgx.Tx) error {
	db.staged = nil
	return db.rollbackErr
}

func (db *swapEventDB) apply(op swapEventOp) {
	switch op.kind {
	case "state":
		for _, key := range op.keys {
			db.committed[key] = op.state
			delete(db.deleted, key)
		}
	case "delete":
		for _, key := range op.keys {
			delete(db.committed, key)
			db.deleted[key] = true
		}
	}
}

func proofStateKeys(proofs []cashu.Proof) []string {
	keys := make([]string, len(proofs))
	for i, proof := range proofs {
		keys[i] = proof.Y.ToHex()
	}
	return keys
}

func (db *swapEventDB) GetProofsFromSecretCurve(pgx.Tx, []cashu.WrappedPublicKey) (cashu.Proofs, error) {
	return nil, nil
}

func (db *swapEventDB) GetRestoreSigsFromBlindedMessages(pgx.Tx, []cashu.WrappedPublicKey) ([]cashu.RecoverSigDB, error) {
	return nil, nil
}

func (db *swapEventDB) SaveProof(_ pgx.Tx, proofs []cashu.Proof) error {
	for _, proof := range proofs {
		db.staged = append(db.staged, swapEventOp{kind: "state", keys: []string{proof.Y.ToHex()}, state: proof.State})
	}
	return nil
}

func (db *swapEventDB) SetProofsState(_ pgx.Tx, proofs cashu.Proofs, state cashu.ProofState) error {
	db.staged = append(db.staged, swapEventOp{kind: "state", keys: proofStateKeys(proofs), state: state})
	return nil
}

func (db *swapEventDB) SaveRestoreSigs(pgx.Tx, []cashu.RecoverSigDB) error {
	return db.saveRestoreSigsErr
}

func (db *swapEventDB) DeleteProofs(_ pgx.Tx, proofs cashu.Proofs) error {
	if db.deleteErr != nil {
		return db.deleteErr
	}
	db.staged = append(db.staged, swapEventOp{kind: "delete", keys: proofStateKeys(proofs)}) //nolint:exhaustruct // Deletes carry no state.
	return nil
}

// swapEventSigner is a Docker-independent signer double. The hook lets a test
// observe the WebSocket state right before the outputs are signed.
type swapEventSigner struct { //nolint:govet // Test double field order is not performance critical.
	signer.Signer
	keysets  signer.GetKeysetsResponse
	signErr  error
	signHook func()
}

func (s *swapEventSigner) GetKeysets() (signer.GetKeysetsResponse, error) {
	return s.keysets, nil
}

func (s *swapEventSigner) VerifyProofs([]cashu.Proof) error {
	return nil
}

func (s *swapEventSigner) SignBlindMessages(messages []cashu.BlindedMessage) ([]cashu.BlindSignature, []cashu.RecoverSigDB, error) {
	if s.signHook != nil {
		s.signHook()
	}
	if s.signErr != nil {
		return nil, nil, s.signErr
	}
	signatures := make([]cashu.BlindSignature, len(messages))
	for i, message := range messages {
		signatures[i] = cashu.BlindSignature{Id: message.Id, Amount: message.Amount} //nolint:exhaustruct // Only identity fields matter.
	}
	return signatures, nil, nil
}

type swapEventRun struct { //nolint:govet // Test double field order is not performance critical.
	mint              *Mint
	db                *swapEventDB
	events            chan cashu.Proof
	firstEvent        *cashu.Proof
	stateAtFirstEvent cashu.ProofState
	deletedAtFirst    bool
	err               error
	proofY            string
}

func newSwapEventRequest(t *testing.T) (cashu.PostSwapRequest, cashu.Proof) {
	t.Helper()

	cPriv, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		t.Fatalf("secp256k1.GeneratePrivateKey(): %v", err)
	}
	bPriv, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		t.Fatalf("secp256k1.GeneratePrivateKey(): %v", err)
	}

	// A plain (non P2PK/HTLC) secret: 64 hex characters, valid UTF-8 and well
	// under the 512 byte limit. Built at runtime so it is not mistaken for a
	// hardcoded credential.
	proof := cashu.Proof{ //nolint:exhaustruct // Only the fields exercised by ExecuteSwap are set.
		Id:     swapEventKeysetId,
		Amount: 8,
		Secret: strings.Repeat("ab", 32),
		C:      cashu.WrappedPublicKey{PublicKey: cPriv.PubKey()},
		State:  cashu.PROOF_UNSPENT,
	}
	hashedProof, err := proof.HashSecretToCurve()
	if err != nil {
		t.Fatalf("proof.HashSecretToCurve(): %v", err)
	}

	request := cashu.PostSwapRequest{
		Inputs: cashu.Proofs{proof},
		Outputs: cashu.BlindedMessages{
			{Id: swapEventKeysetId, Amount: 8, B_: cashu.WrappedPublicKey{PublicKey: bPriv.PubKey()}, Witness: ""}, //nolint:exhaustruct // Only the fields exercised by ExecuteSwap are set.
		},
	}
	return request, hashedProof
}

func runSwapEvents(t *testing.T, db *swapEventDB, signErr error) swapEventRun {
	t.Helper()

	request, hashedProof := newSwapEventRequest(t)
	proofY := hashedProof.Y.ToHex()

	observer := newObserverForTest()
	events := make(chan cashu.Proof, 3)
	observer.AddProofWatch(proofY, ProofWatchChannel{Channel: events}) //nolint:exhaustruct // Subscription ID is irrelevant.

	sig := &swapEventSigner{ //nolint:exhaustruct // Only the fields exercised by ExecuteSwap are set.
		keysets: signer.GetKeysetsResponse{Keysets: []cashu.BasicKeysetResponse{{Id: swapEventKeysetId, Unit: cashu.Sat.String(), InputFeePpk: 0}}}, //nolint:exhaustruct // Only the fields used for validation are set.
		signErr: signErr,
	}

	run := swapEventRun{db: db, events: events, proofY: proofY} //nolint:exhaustruct // Remaining fields are filled during the run.
	sig.signHook = func() {
		select {
		case event := <-events:
			run.firstEvent = &event
			run.stateAtFirstEvent = db.committed[proofY]
			run.deletedAtFirst = db.deleted[proofY]
		default:
			t.Errorf("signer started before the pending event was emitted")
		}
	}

	mint := Mint{MintDB: db, Signer: sig, Observer: &observer} //nolint:exhaustruct // Only the collaborating pieces are used.
	run.mint = &mint
	_, run.err = mint.ExecuteSwap(context.Background(), request)
	return run
}

func drainProofEvents(ch chan cashu.Proof) []cashu.Proof {
	var events []cashu.Proof
	for {
		select {
		case event := <-ch:
			events = append(events, event)
		default:
			return events
		}
	}
}

func eventStates(events []cashu.Proof) []cashu.ProofState {
	states := make([]cashu.ProofState, len(events))
	for i, event := range events {
		states[i] = event.State
	}
	return states
}

func assertEventStates(t *testing.T, got []cashu.Proof, want []cashu.ProofState) {
	t.Helper()
	gotStates := eventStates(got)
	if len(gotStates) != len(want) {
		t.Fatalf("event states = %v, want %v", gotStates, want)
	}
	for i := range want {
		if gotStates[i] != want[i] {
			t.Fatalf("event states = %v, want %v", gotStates, want)
		}
	}
}

// TestExecuteSwapEmitsPendingThenSpent verifies the happy path ordering: the
// PENDING event is delivered after the reservation commit and before signing,
// and SPENT is delivered once the spend transaction commits.
func TestExecuteSwapEmitsPendingThenSpent(t *testing.T) {
	db := newSwapEventDB()
	run := runSwapEvents(t, db, nil)

	if run.err != nil {
		t.Fatalf("ExecuteSwap() error = %v", run.err)
	}
	if run.firstEvent == nil {
		t.Fatal("no pending event observed before signing")
	}
	if run.firstEvent.State != cashu.PROOF_PENDING {
		t.Fatalf("first event state = %q, want PENDING", run.firstEvent.State)
	}
	if run.stateAtFirstEvent != cashu.PROOF_PENDING {
		t.Fatalf("db state at pending event = %q, want PENDING", run.stateAtFirstEvent)
	}
	if run.deletedAtFirst {
		t.Fatal("proof was deleted before the pending event")
	}

	assertEventStates(t, drainProofEvents(run.events), []cashu.ProofState{cashu.PROOF_SPENT})
	if db.committed[run.proofY] != cashu.PROOF_SPENT {
		t.Fatalf("committed proof state = %q, want SPENT", db.committed[run.proofY])
	}
}

// TestExecuteSwapEmitsUnspentAfterConfirmedCleanup verifies that a signing
// failure which is unambiguously rolled back deletes the pending proofs and
// emits UNSPENT, without ever emitting SPENT.
func TestExecuteSwapEmitsUnspentAfterConfirmedCleanup(t *testing.T) {
	db := newSwapEventDB()
	run := runSwapEvents(t, db, errors.New("sign outputs failed"))

	if run.err == nil {
		t.Fatal("ExecuteSwap() error = nil, want signing error")
	}
	if run.firstEvent == nil || run.firstEvent.State != cashu.PROOF_PENDING {
		t.Fatalf("first event = %+v, want PENDING", run.firstEvent)
	}

	assertEventStates(t, drainProofEvents(run.events), []cashu.ProofState{cashu.PROOF_UNSPENT})
	if _, exists := db.committed[run.proofY]; exists {
		t.Fatalf("committed proof still present after cleanup: %q", db.committed[run.proofY])
	}
	if !db.deleted[run.proofY] {
		t.Fatal("proof was not deleted from the committed view")
	}
}

// TestExecuteSwapEmitsUnspentAfterRolledBackSpend verifies the confirmed
// rollback path: when the spend transaction fails and its rollback succeeds,
// the pending proofs are cleaned up and UNSPENT is emitted.
func TestExecuteSwapEmitsUnspentAfterRolledBackSpend(t *testing.T) {
	db := newSwapEventDB()
	db.saveRestoreSigsErr = errors.New("save restore sigs failed")
	run := runSwapEvents(t, db, nil)

	if run.err == nil {
		t.Fatal("ExecuteSwap() error = nil, want spend error")
	}
	if run.firstEvent == nil || run.firstEvent.State != cashu.PROOF_PENDING {
		t.Fatalf("first event = %+v, want PENDING", run.firstEvent)
	}

	assertEventStates(t, drainProofEvents(run.events), []cashu.ProofState{cashu.PROOF_UNSPENT})
	if _, exists := db.committed[run.proofY]; exists {
		t.Fatal("proof still committed after rolled back spend")
	}
}

// TestExecuteSwapNoEventWhenReservationCommitFails verifies that no proof state
// event is advertised when the reservation transaction cannot be committed.
func TestExecuteSwapNoEventWhenReservationCommitFails(t *testing.T) {
	db := newSwapEventDB()
	db.commitErrAt = 1
	run := runSwapEvents(t, db, nil)

	if run.err == nil {
		t.Fatal("ExecuteSwap() error = nil, want reservation commit error")
	}
	if run.firstEvent != nil {
		t.Fatalf("unexpected event before failed reservation commit: %+v", run.firstEvent)
	}
	if events := drainProofEvents(run.events); len(events) != 0 {
		t.Fatalf("events = %v, want none", eventStates(events))
	}
}

// TestExecuteSwapNoUnspentWhenCleanupCommitFails verifies that a failed cleanup
// commit never advertises UNSPENT (the proofs may still be in the database).
func TestExecuteSwapNoUnspentWhenCleanupCommitFails(t *testing.T) {
	db := newSwapEventDB()
	db.commitErrAt = 2
	run := runSwapEvents(t, db, errors.New("sign outputs failed"))

	if run.err == nil {
		t.Fatal("ExecuteSwap() error = nil, want cleanup error")
	}
	if run.firstEvent == nil || run.firstEvent.State != cashu.PROOF_PENDING {
		t.Fatalf("first event = %+v, want PENDING", run.firstEvent)
	}
	if events := drainProofEvents(run.events); len(events) != 0 {
		t.Fatalf("events after failed cleanup commit = %v, want none", eventStates(events))
	}
	if _, exists := db.committed[run.proofY]; !exists {
		t.Fatal("proof unexpectedly removed although cleanup commit failed")
	}
}

// TestExecuteSwapNoUnspentWhenSpendCommitAmbiguous verifies the guard: when the
// spend commit fails and the transaction is already closed (ambiguous commit),
// the proofs are not deleted and no UNSPENT is advertised.
func TestExecuteSwapNoUnspentWhenSpendCommitAmbiguous(t *testing.T) {
	db := newSwapEventDB()
	db.commitErrAt = 2
	db.rollbackErr = pgx.ErrTxClosed
	run := runSwapEvents(t, db, nil)

	if run.err == nil {
		t.Fatal("ExecuteSwap() error = nil, want ambiguous spend commit error")
	}
	if run.firstEvent == nil || run.firstEvent.State != cashu.PROOF_PENDING {
		t.Fatalf("first event = %+v, want PENDING", run.firstEvent)
	}
	if events := drainProofEvents(run.events); len(events) != 0 {
		t.Fatalf("events after ambiguous commit = %v, want none", eventStates(events))
	}
	if db.committed[run.proofY] != cashu.PROOF_PENDING {
		t.Fatalf("committed proof state = %q, want untouched PENDING", db.committed[run.proofY])
	}
	if db.deleted[run.proofY] {
		t.Fatal("proof was deleted on an ambiguous commit")
	}
}

// TestExecuteSwapNilObserverIsSafe verifies the baseline behaviour: ExecuteSwap
// must not panic when no observer is configured.
func TestExecuteSwapNilObserverIsSafe(t *testing.T) {
	db := newSwapEventDB()
	request, _ := newSwapEventRequest(t)

	sig := &swapEventSigner{ //nolint:exhaustruct // Only the fields exercised by ExecuteSwap are set.
		keysets: signer.GetKeysetsResponse{Keysets: []cashu.BasicKeysetResponse{{Id: swapEventKeysetId, Unit: cashu.Sat.String(), InputFeePpk: 0}}}, //nolint:exhaustruct // Only the fields used for validation are set.
	}
	mint := Mint{MintDB: db, Signer: sig, Observer: nil} //nolint:exhaustruct // Observer intentionally nil.

	if _, err := mint.ExecuteSwap(context.Background(), request); err != nil {
		t.Fatalf("ExecuteSwap() error = %v", err)
	}
	if db.committed[request.Inputs[0].Y.ToHex()] != cashu.PROOF_SPENT {
		t.Fatal("swap did not reach the spent state without an observer")
	}
}
