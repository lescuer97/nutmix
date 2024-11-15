package cashu

import (
	"encoding/hex"
	"github.com/tyler-smith/go-bip32"
	"testing"
)

func TestGenerateKeysetsAndIdGeneration(t *testing.T) {

	// setup key
	key, err := bip32.NewMasterKey([]byte("seed"))
	if err != nil {
		t.Errorf("could not setup master key %+v", err)
	}

	generatedKeysets, err := GenerateKeysets(key, GetAmountsForKeysets(), "id", Sat, 0, true)

	if err != nil {
		t.Errorf("could not generate keyset %+v", err)
	}

	if len(generatedKeysets) != len(GetAmountsForKeysets()) {
		t.Errorf("keyset length is not the same as PosibleKeysetValues length")
	}

	// check if the keyset amount is 0
	if generatedKeysets[0].Amount != 1 {
		t.Errorf("keyset amount is not 0")
	}
	if generatedKeysets[0].Unit != Sat.String() {
		t.Errorf("keyset unit is not Sat")
	}

	if hex.EncodeToString(generatedKeysets[0].PrivKey.PubKey().SerializeCompressed()) != "03fbf65684a42313691fe562aa315f26409a19aaaaa8ef0163fc8d8598f16fe003" {
		t.Errorf("keyset id PrivKEy is not correct. %+v", hex.EncodeToString(generatedKeysets[0].PrivKey.PubKey().SerializeCompressed()))
	}

	keysetId, err := DeriveKeysetId(generatedKeysets)

	if err != nil {
		t.Errorf("could not derive keyset id %+v", err)
	}

	if keysetId != "0014d74f728e80b8" {
		t.Errorf("keyset id is not correct")
	}

}

func TestChangeProofsStateToPending(t *testing.T) {

	proofs := Proofs{
		Proof{
			Amount: 1,
			State:  PROOF_UNSPENT,
		},
		Proof{
			Amount: 2,
			State:  PROOF_UNSPENT,
		},
	}
	proofs.SetProofsState(PROOF_PENDING)

	if proofs[0].State != PROOF_PENDING {
		t.Errorf("proof transformation not working, should be: %v ", proofs[1].State)
	}
	if proofs[1].State != PROOF_PENDING {
		t.Errorf("proof transformation not working, should be: %v ", proofs[1].State)

	}

}
func TestChangeProofsStateToPendingAndQuoteSet(t *testing.T) {

	proofs := Proofs{
		Proof{
			Amount: 1,
			State:  PROOF_UNSPENT,
		},
		Proof{
			Amount: 2,
			State:  PROOF_UNSPENT,
		},
	}
	proofs.SetPendingAndQuoteRef("123")

	if proofs[0].State != PROOF_PENDING {
		t.Errorf("proof transformation not working, should be: %v ", proofs[1].State)
	}
	res := "123"
	if *proofs[0].Quote != res {
		t.Errorf("proof transformation not working, should be: %v. is:  ", "123")
	}
	if proofs[1].State != PROOF_PENDING {
		t.Errorf("proof transformation not working, should be: %v ", proofs[1].State)

	}
	if *proofs[1].Quote != res {
		t.Errorf("proof transformation not working, should be: %v ", "123")
	}

}
