package mint

import (
	crand "crypto/rand"
	"encoding/hex"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lescuer97/nutmix/api/cashu"
)

func testProofWithState(t *testing.T, state cashu.ProofState, witness string) cashu.Proof {
	t.Helper()

	secretBytes := make([]byte, 32)
	_, err := crand.Read(secretBytes)
	if err != nil {
		t.Fatalf("rand.Read(secretBytes): %v", err)
	}

	cPriv, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		t.Fatalf("secp256k1.GeneratePrivateKey(): %v", err)
	}

	proof := cashu.Proof{
		Amount:  1,
		C:       cashu.WrappedPublicKey{PublicKey: cPriv.PubKey()},
		Id:      "test-keyset",
		Secret:  hex.EncodeToString(secretBytes),
		Y:       cashu.WrappedPublicKey{PublicKey: nil},
		Quote:   nil,
		Witness: witness,
		State:   state,
		SeenAt:  0,
	}

	proof, err = proof.HashSecretToCurve()
	if err != nil {
		t.Fatalf("proof.HashSecretToCurve(): %v", err)
	}

	return proof
}
