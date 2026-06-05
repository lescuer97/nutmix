package mint

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lescuer97/nutmix/api/cashu"
	mockdb "github.com/lescuer97/nutmix/internal/database/mock_db"
	"github.com/lescuer97/nutmix/internal/utils"
)

func TestCheckProofStateReturnsStoredState(t *testing.T) {
	pendingProof := testProofWithState(t, cashu.PROOF_PENDING, "pending-witness")
	spentProof := testProofWithState(t, cashu.PROOF_SPENT, "")
	missingProof := testProofWithState(t, cashu.PROOF_UNSPENT, "")
	mockDB := new(mockdb.MockDB)
	mockDB.Proofs = []cashu.Proof{pendingProof, spentProof}
	var config utils.Config

	mint := &Mint{
		LightningBackend:        nil,
		MintDB:                  mockDB,
		Signer:                  nil,
		OICDClient:              nil,
		Observer:                nil,
		NostrNotificationConfig: nil,
		MintPubkey:              "",
		Config:                  config,
	}

	states, err := CheckProofState(context.Background(), mint, []cashu.WrappedPublicKey{pendingProof.Y, spentProof.Y, missingProof.Y})
	if err != nil {
		t.Fatalf("CheckProofState(context.Background(), mint, Ys): %v", err)
	}

	if states[0].State != cashu.PROOF_PENDING {
		t.Fatalf("expected first proof to be pending, got %v", states[0].State)
	}
	if states[0].Witness == nil || *states[0].Witness != "pending-witness" {
		t.Fatalf("expected pending witness to be returned, got %+v", states[0].Witness)
	}
	if states[1].State != cashu.PROOF_SPENT {
		t.Fatalf("expected second proof to be spent, got %v", states[1].State)
	}
	if states[2].State != cashu.PROOF_UNSPENT {
		t.Fatalf("expected third proof to be unspent, got %v", states[2].State)
	}
}

func TestCheckProofStatePrefersSpentOverPendingForDuplicateRows(t *testing.T) {
	pendingProof := testProofWithState(t, cashu.PROOF_PENDING, "")
	spentProof := pendingProof
	spentProof.State = cashu.PROOF_SPENT
	mockDB := new(mockdb.MockDB)
	mockDB.Proofs = []cashu.Proof{pendingProof, spentProof}
	var config utils.Config

	mint := &Mint{
		LightningBackend:        nil,
		MintDB:                  mockDB,
		Signer:                  nil,
		OICDClient:              nil,
		Observer:                nil,
		NostrNotificationConfig: nil,
		MintPubkey:              "",
		Config:                  config,
	}

	states, err := CheckProofState(context.Background(), mint, []cashu.WrappedPublicKey{pendingProof.Y})
	if err != nil {
		t.Fatalf("CheckProofState(context.Background(), mint, Ys): %v", err)
	}

	if states[0].State != cashu.PROOF_SPENT {
		t.Fatalf("expected spent state to win, got %v", states[0].State)
	}
}

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
