//nolint:exhaustruct // Fixtures specify only fields relevant to the lifecycle scenario.
package mockdb

import (
	"errors"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lescuer97/nutmix/api/cashu"
)

func TestSignedMessagesCannotBeReplaced(t *testing.T) {
	key := cashu.WrappedPublicKey{PublicKey: secp256k1.PrivKeyFromBytes([]byte{1}).PubKey()}
	sig := cashu.RecoverSigDB{B_: key, C_: key, Id: "keyset", Amount: 1, MintQuote: "mint-quote"}
	db := &MockDB{}
	if err := db.SaveRestoreSigs(nil, []cashu.RecoverSigDB{sig, sig}); !errors.Is(err, cashu.ErrRepeatedOutput) || len(db.RecoverSigDB) != 0 {
		t.Fatalf("duplicate batch was partially saved: %v", err)
	}
	if err := db.SaveRestoreSigs(nil, []cashu.RecoverSigDB{sig}); err != nil {
		t.Fatal(err)
	}
	changed := sig
	changed.Amount++
	if err := db.SaveRestoreSigs(nil, []cashu.RecoverSigDB{changed}); !errors.Is(err, cashu.ErrBlindMessageAlreadySigned) {
		t.Fatalf("signed message was overwritten: %v", err)
	}
	restored, err := db.GetRestoreSigsFromBlindedMessages(nil, []cashu.WrappedPublicKey{key})
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
	if err := db.SaveRestoreSigs(nil, []cashu.RecoverSigDB{valid, invalid}); !errors.Is(err, cashu.ErrInvalidBlindMessage) || len(db.RecoverSigDB) != 0 {
		t.Fatalf("invalid batch was partially saved: %v", err)
	}
}
