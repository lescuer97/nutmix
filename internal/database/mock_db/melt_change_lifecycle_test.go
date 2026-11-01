//nolint:exhaustruct // Fixtures specify only fields relevant to settlement.
package mockdb

import (
	"context"
	"errors"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
)

func mockChangeOutput(n byte) cashu.BlindedMessage {
	return cashu.BlindedMessage{B_: cashu.WrappedPublicKey{PublicKey: secp256k1.PrivKeyFromBytes([]byte{n}).PubKey()}, Id: "keyset"}
}

func settlementProof(quote string, amount uint64) cashu.Proof {
	p := cashu.Proof{
		C:  cashu.WrappedPublicKey{PublicKey: secp256k1.PrivKeyFromBytes([]byte{31}).PubKey()},
		Y:  cashu.WrappedPublicKey{PublicKey: secp256k1.PrivKeyFromBytes([]byte{32}).PubKey()},
		Id: "keyset", Secret: "settlement-proof", Amount: amount, State: cashu.PROOF_PENDING,
	}
	p.Quote = &quote
	return p
}

func settlementFixture() (*MockDB, database.MeltSettlementCommand) {
	quote := "quote"
	proof := settlementProof(quote, 10)
	blank := mockChangeOutput(3)
	output := cashu.SignedOutput{B_: blank.B_, C_: proof.C, Id: blank.Id, Amount: 3}
	db := &MockDB{
		MeltRequest: []cashu.MeltRequestDB{{Quote: quote, CheckingId: "checking", Unit: "sat", Amount: 5, State: cashu.PENDING}},
		Proofs:      cashu.Proofs{proof},
		MeltChange:  []cashu.MeltChange{{B_: blank.B_, Id: blank.Id, Quote: quote, CreatedAt: 1}},
	}
	return db, database.MeltSettlementCommand{
		QuoteID: quote, CheckingID: "checking", Unit: "sat", Inputs: cashu.Proofs{proof},
		Outputs: []cashu.SignedOutput{output}, LightningFee: 1, InputFee: 1, Preimage: "preimage",
	}
}

func TestCompleteMeltSettlementMockValidatesAtomically(t *testing.T) {
	for _, mutate := range []struct { //nolint:govet // Table favors name-first readability over packing.
		name string
		fn   func(*database.MeltSettlementCommand)
	}{
		{"wrong owner", func(c *database.MeltSettlementCommand) { c.QuoteID = "other" }},
		{"missing input", func(c *database.MeltSettlementCommand) { c.Inputs = nil }},
		{"wrong input", func(c *database.MeltSettlementCommand) { c.Inputs[0].Secret = "other" }},
		{"over refund", func(c *database.MeltSettlementCommand) { c.Outputs[0].Amount = 4 }},
		{"wrong prefix output", func(c *database.MeltSettlementCommand) { c.Outputs[0].B_ = mockChangeOutput(9).B_ }},
		{"keyset mismatch", func(c *database.MeltSettlementCommand) { c.Outputs[0].Id = "wrong-keyset" }},
		{"more outputs than blanks", func(c *database.MeltSettlementCommand) { c.Outputs = append(c.Outputs, c.Outputs[0]) }},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			db, command := settlementFixture()
			mutate.fn(&command)
			_, err := db.CompleteMeltSettlement(context.Background(), nil, command)
			if err == nil {
				t.Fatal("invalid settlement accepted")
			}
			if len(db.RecoverSigDB) != 0 || len(db.MeltChange) != 1 || db.Proofs[0].State != cashu.PROOF_PENDING || db.MeltRequest[0].State != cashu.PENDING {
				t.Fatalf("failed validation mutated storage: %+v", db)
			}
		})
	}
}

func TestCompleteMeltSettlementMockAndIdempotency(t *testing.T) {
	db, command := settlementFixture()
	result, err := db.CompleteMeltSettlement(context.Background(), nil, command)
	if err != nil || result.AlreadyCompleted || result.Quote.State != cashu.PAID || result.Quote.FeePaid != 1 || result.Quote.PaymentPreimage != "preimage" || len(result.Quote.Change) != 1 || result.Proofs[0].State != cashu.PROOF_SPENT {
		t.Fatalf("settlement=%+v err=%v", result, err)
	}
	command.LightningFee, command.InputFee, command.Preimage = 99, 99, "override"
	retry, err := db.CompleteMeltSettlement(context.Background(), nil, command)
	if err != nil || !retry.AlreadyCompleted || retry.Quote.FeePaid != 1 || retry.Quote.PaymentPreimage != "preimage" || len(retry.Quote.Change) != 1 || len(db.RecoverSigDB) != 1 {
		t.Fatalf("idempotent settlement=%+v err=%v", retry, err)
	}
}

func TestCompleteMeltSettlementMockPaidPreparationWithPendingInputs(t *testing.T) {
	db, command := settlementFixture()
	db.MeltRequest[0].State = cashu.PAID
	db.MeltRequest[0].Melted = true
	db.MeltRequest[0].FeePaid = 1
	db.MeltRequest[0].PaymentPreimage = "durable-preimage"
	command.LightningFee = 50
	command.Preimage = "untrusted-override"
	if _, err := db.CompleteMeltSettlement(context.Background(), nil, command); !errors.Is(err, database.ErrMeltSettlementState) {
		t.Fatalf("mismatched durable payment metadata error=%v", err)
	}
	if len(db.RecoverSigDB) != 0 || len(db.MeltChange) != 1 || db.Proofs[0].State != cashu.PROOF_PENDING || db.MeltRequest[0].State != cashu.PAID {
		t.Fatalf("rejected retry mutated settlement: %+v", db)
	}
	command.LightningFee, command.Preimage = 1, "durable-preimage"
	result, err := db.CompleteMeltSettlement(context.Background(), nil, command)
	if err != nil || result.AlreadyCompleted || result.Quote.FeePaid != 1 || result.Quote.PaymentPreimage != "durable-preimage" || result.Proofs[0].State != cashu.PROOF_SPENT {
		t.Fatalf("prepared settlement=%+v err=%v", result, err)
	}
	if len(db.RecoverSigDB) != 1 || len(db.MeltChange) != 0 {
		t.Fatalf("successful preparation signatures=%d pending=%d", len(db.RecoverSigDB), len(db.MeltChange))
	}
}

func TestCompleteMeltSettlementMockRejectsPaidUnmeltedQuote(t *testing.T) {
	db, command := settlementFixture()
	db.MeltRequest[0].State = cashu.PAID
	if _, err := db.CompleteMeltSettlement(context.Background(), nil, command); !errors.Is(err, database.ErrMeltSettlementState) {
		t.Fatalf("PAID but unmelted quote error=%v", err)
	}
}

func TestCompleteMeltSettlementMockContextAndOwnership(t *testing.T) {
	db, command := settlementFixture()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := db.CompleteMeltSettlement(ctx, nil, command); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled context error=%v", err)
	}
	command.CheckingID = "other"
	if _, err := db.CompleteMeltSettlement(context.Background(), nil, command); !errors.Is(err, database.ErrMeltSettlementState) {
		t.Fatalf("wrong checking ID error=%v", err)
	}
}
