//nolint:exhaustruct_v5 // Fixtures specify only fields relevant to the lifecycle scenario.
package mockdb

import (
	"context"
	"errors"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lescuer97/nutmix/api/cashu"
)

func signedOutput(sig cashu.RecoverSigDB) cashu.SignedOutput {
	return cashu.SignedOutput{B_: sig.B_, C_: sig.C_, Dleq: sig.Dleq, Id: sig.Id, Amount: sig.Amount, CreatedAt: sig.CreatedAt}
}

func TestSignedMessagesCannotBeReplaced(t *testing.T) {
	key := cashu.WrappedPublicKey{PublicKey: secp256k1.PrivKeyFromBytes([]byte{1}).PubKey()}
	sig := cashu.RecoverSigDB{B_: key, C_: key, Id: "keyset", Amount: 1, MintQuote: "mint-quote"}
	db := &MockDB{}
	if err := db.InsertMintSignedOutputs(context.Background(), nil, "", []cashu.SignedOutput{signedOutput(sig)}); !errors.Is(err, cashu.ErrInvalidBlindMessage) || len(db.RecoverSigDB) != 0 {
		t.Fatalf("empty mint quote was accepted: %v", err)
	}
	if err := db.InsertMintSignedOutputs(context.Background(), nil, "mint-quote", []cashu.SignedOutput{signedOutput(sig), signedOutput(sig)}); !errors.Is(err, cashu.ErrRepeatedOutput) || len(db.RecoverSigDB) != 0 {
		t.Fatalf("duplicate batch was partially saved: %v", err)
	}
	if err := db.InsertMintSignedOutputs(context.Background(), nil, "mint-quote", []cashu.SignedOutput{signedOutput(sig)}); err != nil {
		t.Fatal(err)
	}
	changed := sig
	changed.Amount++
	if err := db.InsertSignedOutputs(context.Background(), nil, []cashu.SignedOutput{signedOutput(changed)}); !errors.Is(err, cashu.ErrBlindMessageAlreadySigned) {
		t.Fatalf("signed message was overwritten: %v", err)
	}
	restored, err := db.GetRestoreSigsFromBlindedMessages(context.Background(), nil, []cashu.WrappedPublicKey{key})
	if err != nil || len(restored) != 1 || restored[0].Amount != sig.Amount || restored[0].MintQuote != sig.MintQuote {
		t.Fatalf("signed metadata changed: %v, %v", restored, err)
	}
}

func TestInvalidSignedBatchDoesNotPartiallyPersist(t *testing.T) {
	key := cashu.WrappedPublicKey{PublicKey: secp256k1.PrivKeyFromBytes([]byte{1}).PubKey()}
	valid := cashu.RecoverSigDB{B_: key, C_: key, Id: "keyset", Amount: 1}
	invalid := valid
	invalid.C_ = cashu.WrappedPublicKey{}
	db := &MockDB{}
	if err := db.InsertSignedOutputs(context.Background(), nil, []cashu.SignedOutput{signedOutput(valid), signedOutput(invalid)}); !errors.Is(err, cashu.ErrInvalidBlindMessage) || len(db.RecoverSigDB) != 0 {
		t.Fatalf("invalid batch was partially saved: %v", err)
	}
}
