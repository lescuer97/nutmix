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

	generatedKeysets, err := GenerateKeysets(key, PosibleKeysetValues, "id", Sat)

	if err != nil {
		t.Errorf("could not generate keyset %+v", err)
	}

	if len(generatedKeysets) != len(PosibleKeysetValues) {
		t.Errorf("keyset length is not the same as PosibleKeysetValues length")
	}

	// check if the keyset amount is 0
	if generatedKeysets[0].Amount != 0 {
		t.Errorf("keyset amount is not 0")
	}
	if generatedKeysets[0].Unit != Sat.String() {
		t.Errorf("keyset unit is not Sat")
	}

	if generatedKeysets[0].PrivKey.Key.String() != "c4ed3e54b91e7a49cfecbdfc9c9305fa3f51aecaeeac670cec752c32b381f917" {
		t.Errorf("keyset id is not id")
	}

	keysetId, err := DeriveKeysetId(generatedKeysets)

	if err != nil {
		t.Errorf("could not derive keyset id %+v", err)
	}
	if keysetId != "00fc7e7881e44faa" {
		t.Errorf("keyset id is not correct")
	}

}

func TestDeriveSeedsFromKey(t *testing.T) {

	masterKey := "0000000000000000000000000000000000000000000000000000000000000001"

	generatedSeeds, err := DeriveSeedsFromKey(masterKey, 1, AvailableSeeds)

	if err != nil {
		t.Errorf("could not derive seeds from key %+v", err)
	}

	if len(generatedSeeds) != 2 {
		t.Errorf("seed length is not 2")
	}

	if hex.EncodeToString(generatedSeeds[0].Seed) != "0f451868e048a61dcf274af7c3a463f48d32dbabb47bfd3f4da850f4d6525975" {
		t.Errorf("seed 0 is not correct")
	}

	if hex.EncodeToString(generatedSeeds[1].Seed) != "c5e163917a7995f9d6c197a0164e011995821d2be7fc65f06cdfc4bf0289e0af" {
		t.Errorf("seed 1 is not correct")
	}

	if generatedSeeds[0].Unit != Sat.String() {
		t.Errorf("seed 0 unit is not correct")
	}
	if generatedSeeds[1].Unit != Msat.String() {
		t.Errorf("seed 1 unit is not correct")
	}

	if generatedSeeds[0].Id != "00516525c0c0508e" {
		t.Errorf("seed 0 id is not correct")
	}
	if generatedSeeds[1].Id != "008ef8a9552a5538" {
		t.Errorf("seed 0 id is not correct")
	}

}