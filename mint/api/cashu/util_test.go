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

	generatedKeysets, err := GenerateKeysets(key, GetAmountsForKeysets(), "id", Sat)

	if err != nil {
		t.Errorf("could not generate keyset %+v", err)

	}

	orderedKeys := OrderKeysetByUnit(generatedKeysets)

	firstOrdKey := orderedKeys["keysets"][0]

	if firstOrdKey.Keys["1"] != "0368a33e7aad5f9983dccd05b5792d8c5f3c9e28d5cad4e448a69eead5b84b3869" {
		t.Errorf("keyset is not correct")
	}

}
