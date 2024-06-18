package cashu

import (
	"encoding/hex"
	"testing"

	"github.com/tyler-smith/go-bip32"
)

func TestGenerateKeysetsAndIdGeneration(t *testing.T) {

	// setup key
	key, err := bip32.NewMasterKey([]byte("seed"))
	if err != nil {
		t.Errorf("could not setup master key %+v", err)
	}

	generatedKeysets, err := GenerateKeysets(key, GetAmountsForKeysets(), "id", Sat)

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

	if hex.EncodeToString(generatedKeysets[0].PrivKey.PubKey().SerializeCompressed()) != "0368a33e7aad5f9983dccd05b5792d8c5f3c9e28d5cad4e448a69eead5b84b3869" {
		t.Errorf("keyset id PrivKEy is not correct. %+v", hex.EncodeToString(generatedKeysets[0].PrivKey.PubKey().SerializeCompressed()))
	}

	keysetId, err := DeriveKeysetId(generatedKeysets)

	if err != nil {
		t.Errorf("could not derive keyset id %+v", err)
	}

	if keysetId != "00b7413b33e713ea" {
		t.Errorf("keyset id is not correct")
	}

}

func TestDeriveSeedsFromKey(t *testing.T) {

	masterKey := "0000000000000000000000000000000000000000000000000000000000000001"

	generatedSeeds, err := DeriveSeedsFromKey(masterKey, 1, AvailableSeeds)

	if err != nil {
		t.Errorf("could not derive seeds from key %+v", err)
	}

	if len(generatedSeeds) != 1 {
		t.Errorf("seed length is not 2")
	}

	if hex.EncodeToString(generatedSeeds[0].Seed) != "0f451868e048a61dcf274af7c3a463f48d32dbabb47bfd3f4da850f4d6525975" {
		t.Errorf("seed 0 is not correct %v", hex.EncodeToString(generatedSeeds[0].Seed))
	}

	if generatedSeeds[0].Unit != Sat.String() {
		t.Errorf("seed 0 unit is not correct")
	}

	if generatedSeeds[0].Id != "00178484f5e74df9" {
		t.Errorf("seed 0 id is not correct %v", generatedSeeds[0].Id)
	}

}
