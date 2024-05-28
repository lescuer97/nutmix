package cashu

import (
	"github.com/tyler-smith/go-bip32"
	"testing"
)

func TestGenerateKeysetsAndIdGeneration(t *testing.T) {

	// setup key
	key, err := bip32.NewMasterKey([]byte("seed"))
	if err != nil {
		t.Errorf("could not setup master key %+v", err)
	}

	generatedKeysets, err := GenerateKeysets(key, PosibleKeysetValues, "id")

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
