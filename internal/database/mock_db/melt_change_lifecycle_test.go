//nolint:exhaustruct // Fixtures specify only fields relevant to the lifecycle scenario.
package mockdb

import (
	"errors"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lescuer97/nutmix/api/cashu"
)

func mockChangeOutput(n byte) cashu.BlindedMessage {
	return cashu.BlindedMessage{B_: cashu.WrappedPublicKey{PublicKey: secp256k1.PrivKeyFromBytes([]byte{n}).PubKey()}, Id: "keyset"}
}

func TestMeltChangeOwnershipAndLifecycle(t *testing.T) {
	db := &MockDB{MeltRequest: []cashu.MeltRequestDB{{Quote: "q1", State: cashu.PENDING}, {Quote: "q2", State: cashu.PENDING}}}
	outputs := []cashu.BlindedMessage{mockChangeOutput(3), mockChangeOutput(1), mockChangeOutput(2)}
	if err := db.SaveMeltChange(nil, outputs, "q1"); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveMeltChange(nil, outputs, "q2"); !errors.Is(err, cashu.ErrOutputPending) {
		t.Fatalf("conflicting owner: %v", err)
	}
	sig := cashu.RecoverSigDB{B_: outputs[0].B_, C_: outputs[0].B_, Id: "keyset", Amount: 1}
	if err := db.SaveRestoreSigs(nil, []cashu.RecoverSigDB{sig}); !errors.Is(err, cashu.ErrOutputPending) {
		t.Fatalf("generic writer bypassed ownership: %v", err)
	}
	if err := db.FinalizeMeltChange(nil, "q2", []cashu.RecoverSigDB{sig}); !errors.Is(err, cashu.ErrQuoteIsPending) {
		t.Fatalf("wrong quote finalized: %v", err)
	}
	changed := sig
	changed.Id = "other-keyset"
	if err := db.FinalizeMeltChange(nil, "q1", []cashu.RecoverSigDB{changed}); !errors.Is(err, cashu.ErrInvalidBlindMessage) {
		t.Fatalf("changed metadata accepted: %v", err)
	}
	if err := db.FinalizeMeltChange(nil, "q1", []cashu.RecoverSigDB{sig}); err != nil {
		t.Fatal(err)
	}
	if len(db.MeltChange) != 0 || len(db.RecoverSigDB) != 1 {
		t.Fatal("unused blanks not released, or selected blank not signed")
	}
	quote, err := db.GetMeltRequestById(nil, "q1")
	if err != nil || len(quote.Change) != 1 || quote.Change[0].Amount != 1 {
		t.Fatalf("quote change=%v err=%v", quote.Change, err)
	}
	if db.RecoverSigDB[0].MeltQuote != "q1" {
		t.Fatal("signed change lost provenance")
	}
	if err := db.DeleteChangeByQuote(nil, "q1"); err != nil {
		t.Fatal(err)
	}
	if err := db.CheckBlindMessagesAvailable(nil, []cashu.WrappedPublicKey{outputs[0].B_}); !errors.Is(err, cashu.ErrBlindMessageAlreadySigned) {
		t.Fatalf("cleanup deleted signed change: %v", err)
	}
	if err := db.SaveMeltChange(nil, outputs[1:], "q2"); err != nil {
		t.Fatalf("unused blanks not reusable: %v", err)
	}
	if err := db.FinalizeMeltChange(nil, "q2", nil); err != nil {
		t.Fatal(err)
	}
	if len(db.MeltChange) != 0 || len(db.RecoverSigDB) != 1 {
		t.Fatal("zero-refund finalization retained blanks or deleted signatures")
	}
}

func TestMeltChangeRejectsDuplicateAndReorderedOutputs(t *testing.T) {
	db := &MockDB{MeltRequest: []cashu.MeltRequestDB{{Quote: "q", State: cashu.PENDING}}}
	first, second := mockChangeOutput(1), mockChangeOutput(2)
	if err := db.SaveMeltChange(nil, []cashu.BlindedMessage{first, first}, "q"); !errors.Is(err, cashu.ErrRepeatedOutput) || len(db.MeltChange) != 0 {
		t.Fatalf("duplicate reservation partially persisted: %v", err)
	}
	if err := db.SaveMeltChange(nil, []cashu.BlindedMessage{first, second}, "q"); err != nil {
		t.Fatal(err)
	}
	sig := cashu.RecoverSigDB{B_: second.B_, C_: second.B_, Id: second.Id, Amount: 1}
	if err := db.FinalizeMeltChange(nil, "q", []cashu.RecoverSigDB{sig}); !errors.Is(err, cashu.ErrInvalidBlindMessage) {
		t.Fatalf("non-prefix blank signed: %v", err)
	}
	if len(db.MeltChange) != 2 || len(db.RecoverSigDB) != 0 {
		t.Fatal("failed finalization mutated records")
	}
}
