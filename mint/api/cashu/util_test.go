package cashu

import (
	"testing"

	"github.com/tyler-smith/go-bip32"
)

func TestOrderKeysetByUnit(t *testing.T) {
	// setup key
	key, err := bip32.NewMasterKey([]byte("seed"))
	if err != nil {
		t.Errorf("could not setup master key %+v", err)
	}

	generatedKeysets, err := GenerateKeysets(key, PosibleKeysetValues, "id")

	if err != nil {
		t.Errorf("could not generate keyset %+v", err)

	}

	orderedKeys := OrderKeysetByUnit(generatedKeysets)

	firstOrdKey := orderedKeys["keysets"][0]

	if firstOrdKey.Keys["0"] != "03fbf65684a42313691fe562aa315f26409a19aaaaa8ef0163fc8d8598f16fe003" {
		t.Errorf("keyset is not correct")
	}

}
