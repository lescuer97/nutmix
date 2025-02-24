package cashu

import (
	"github.com/tyler-smith/go-bip32"
	"testing"
)

func TestOrderKeysetByUnit(t *testing.T) {
	// setup key
	key, err := bip32.NewMasterKey([]byte("seed"))
	if err != nil {
		t.Errorf("could not setup master key %+v", err)
	}

	generatedKeysets, err := GenerateKeysets(key, GetAmountsForKeysets(), "id", Sat, 0, true)

	if err != nil {
		t.Errorf("could not generate keyset %+v", err)

	}

	orderedKeys := OrderKeysetByUnit(generatedKeysets)

	firstOrdKey := orderedKeys["keysets"][0]

	if firstOrdKey.Keys["1"] != "03fbf65684a42313691fe562aa315f26409a19aaaaa8ef0163fc8d8598f16fe003" {
		t.Errorf("keyset is not correct")
	}

}

func TestAmountOfFeeProofs(t *testing.T) {

	var proofs []Proof
	var keysets []BasicKeysetResponse
	id := "keysetID"
	inputFee := uint(100)

	for i := 0; i < 9; i++ {
		// add 9 proofs
		proof := Proof{
			Id: id,
		}

		keyset := BasicKeysetResponse{
			Id:          id,
			InputFeePpk: inputFee,
		}

		proofs = append(proofs, proof)
		keysets = append(keysets, keyset)
	}

	fee, _ := Fees(proofs, keysets)

	if fee != 1 {
		t.Errorf("fee calculation is incorrect: %v. Should be 1", fee)
	}

	for i := 0; i < 3; i++ {
		// add 9 proofs
		proof := Proof{
			Id: id,
		}

		keyset := BasicKeysetResponse{
			Id:          id,
			InputFeePpk: inputFee,
		}

		proofs = append(proofs, proof)
		keysets = append(keysets, keyset)
	}
	fee, _ = Fees(proofs, keysets)

	if fee != 2 {
		t.Errorf("fee calculation is incorrect: %v. Should be 2", fee)
	}

}
