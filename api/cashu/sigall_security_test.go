package cashu

import (
	"encoding/hex"
	"errors"
	"fmt"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lescuer97/nutmix/pkg/crypto"
)

func TestSpendConditionDuplicateTagsFieldRejected(t *testing.T) {
	for _, fields := range []string{
		`"tags":[["sigflag","SIG_ALL"]],"tags":[["locktime","4102444800"]]`,
		`"tags":[],"tags":[]`,
		`"tags":[],"Tags":[]`,
		`"tags":[],"tAgS":[]`,
		`"tags":[],"ta\u0067s":[]`,
		`"tags":null,"tags":[]`,
		`"tags":[],"tags":null`,
	} {
		t.Run(fields, func(t *testing.T) {
			proof := Proof{Secret: fmt.Sprintf(`["P2PK",{"nonce":"test","data":"test",%s}]`, fields)}
			_, _, err := proof.IsProofSpendConditioned()
			if !errors.Is(err, ErrDuplicateTag) {
				t.Fatalf("IsProofSpendConditioned() = %v, want duplicate tags field rejection", err)
			}
		})
	}
}

func TestSigAllDuplicateTagsAttackRejected(t *testing.T) {
	attacker := secp256k1.PrivKeyFromBytes([]byte{2})
	victim := secp256k1.PrivKeyFromBytes([]byte{3})
	refund := secp256k1.PrivKeyFromBytes([]byte{4})
	mintKey := secp256k1.PrivKeyFromBytes([]byte{1})
	victimKey := hex.EncodeToString(victim.PubKey().SerializeCompressed())
	victimTags := fmt.Sprintf(`[["locktime","4102444800"],["refund","%s"]]`, hex.EncodeToString(refund.PubKey().SerializeCompressed()))
	inputs := Proofs{
		{Amount: 1, Secret: fmt.Sprintf(`["P2PK",{"nonce":"attacker","data":"%s","tags":[["sigflag","SIG_ALL"],["pubkeys","%s"]],"tags":%s}]`, victimKey, hex.EncodeToString(attacker.PubKey().SerializeCompressed()), victimTags)},
		{Amount: 1, Secret: fmt.Sprintf(`["P2PK",{"nonce":"victim","data":"%s","tags":%s}]`, victimKey, victimTags)},
	}
	for i := range inputs {
		y, err := crypto.HashToCurve([]byte(inputs[i].Secret))
		if err != nil {
			t.Fatal(err)
		}
		inputs[i].C = WrappedPublicKey{crypto.SignBlindedMessage(y, mintKey)}
		if !crypto.Verify(inputs[i].Secret, mintKey, inputs[i].C.PublicKey) {
			t.Fatal("test input must have a valid mint signature")
		}
	}
	outputs := BlindedMessages{{Amount: 2, B_: WrappedPublicKey{attacker.PubKey()}}}
	swap := PostSwapRequest{Inputs: inputs, Outputs: outputs}
	melt := PostMeltBolt11Request{Quote: "test-quote", Inputs: inputs, Outputs: outputs}
	for _, operation := range []struct {
		name     string
		message  string
		validate func() error
	}{
		{"swap", swap.makeSigAllMsg(), swap.ValidateSigflag},
		{"melt", melt.makeSigAllMsg(), melt.ValidateSigflag},
	} {
		t.Run(operation.name, func(t *testing.T) {
			inputs[0].Witness = signSecret(t, attacker, operation.message)
			if err := operation.validate(); !errors.Is(err, ErrDuplicateTag) {
				t.Fatalf("ValidateSigflag() = %v, want duplicate tags field rejection", err)
			}
		})
	}
}

func TestSigAllRepetitionInvariants(t *testing.T) {
	secret := func(kind, nonce, data, tags string) string {
		return fmt.Sprintf(`["%s",{"nonce":"%s","data":"%s","tags":%s}]`, kind, nonce, data, tags)
	}
	allTags := `[["sigflag","SIG_ALL"]]`
	for _, tc := range []struct {
		name    string
		first   string
		second  string
		wantErr bool
	}{
		{"matching P2PK", secret("P2PK", "first", "data", allTags), secret("P2PK", "second", "data", allTags), false},
		{"matching HTLC", secret("HTLC", "first", "data", allTags), secret("HTLC", "second", "data", allTags), false},
		{"different kind", secret("P2PK", "first", "data", allTags), secret("HTLC", "second", "data", allTags), true},
		{"unknown kind", secret("UNKNOWN", "first", "data", allTags), secret("UNKNOWN", "second", "data", allTags), true},
		{"default SIG_INPUTS", secret("P2PK", "first", "data", `[]`), secret("P2PK", "second", "data", `[]`), true},
		{"explicit SIG_INPUTS", secret("P2PK", "first", "data", `[["sigflag","SIG_INPUTS"]]`), secret("P2PK", "second", "data", `[["sigflag","SIG_INPUTS"]]`), true},
		{"first lacks SIG_ALL", secret("P2PK", "first", "data", `[]`), secret("P2PK", "second", "data", allTags), true},
		{"second lacks SIG_ALL", secret("P2PK", "first", "data", allTags), secret("P2PK", "second", "data", `[]`), true},
		{"different data", secret("P2PK", "first", "data", allTags), secret("P2PK", "second", "other", allTags), true},
		{"different tags", secret("P2PK", "first", "data", allTags), secret("P2PK", "second", "data", `[["sigflag","SIG_ALL"],["locktime","4102444800"]]`), true},
		{"malformed first secret", `not-json`, secret("P2PK", "second", "data", allTags), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inputs := Proofs{{Secret: tc.first}, {Secret: tc.second}}
			swap := PostSwapRequest{Inputs: inputs}
			melt := PostMeltBolt11Request{Inputs: inputs}
			for _, operation := range []struct {
				name     string
				validate func() error
			}{
				{"swap", swap.verifySigAllRepetition},
				{"melt", melt.verifySigAllRepetition},
			} {
				t.Run(operation.name, func(t *testing.T) {
					err := operation.validate()
					if (err != nil) != tc.wantErr {
						t.Fatalf("verifySigAllRepetition() = %v, wantErr %v", err, tc.wantErr)
					}
				})
			}
		})
	}
}
