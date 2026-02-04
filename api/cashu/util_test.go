package cashu_test

import (
	"encoding/hex"
	"testing"

	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lescuer97/nutmix/api/cashu"
	localsigner "github.com/lescuer97/nutmix/internal/signer/local_signer"
)

func TestOrderKeysetByUnit(t *testing.T) {
	// setup key
	keyBytes, err := hex.DecodeString("0000000000000000000000000000000000000000000000000000000000000001")
	if err != nil {
		t.Errorf("could not decode key %+v", err)
	}
	key, err := hdkeychain.NewMaster(keyBytes, &chaincfg.MainNetParams)
	if err != nil {
		t.Errorf("could not setup master key %+v", err)
	}

	seed := cashu.Seed{
		Id:          "id",
		Unit:        cashu.Sat.String(),
		Version:     0,
		InputFeePpk: 0,
		Amounts:     cashu.GetAmountsForKeysets(cashu.LegacyMaxKeysetAmount),
		Legacy:      true,
	}

	generatedKeysets, err := localsigner.GenerateKeysets(key, seed)
	if err != nil {
		t.Errorf("could not generate keyset %+v", err)
	}

	orderedKeys := cashu.OrderKeysetByUnit(generatedKeysets)

	firstOrdKey := orderedKeys["keysets"][0]

	if firstOrdKey.Keys["1"] != "03a524f43d6166ad3567f18b0a5c769c6ab4dc02149f4d5095ccf4e8ffa293e785" {
		t.Errorf("keyset is not correct. %v", firstOrdKey.Keys["1"])
	}

}

func TestAmountOfFeeProofs(t *testing.T) {

	var proofs []cashu.Proof
	var keysets []cashu.BasicKeysetResponse
	id := "keysetID"
	inputFee := uint(100)

	for i := 0; i < 9; i++ {
		// add 9 proofs
		proof := cashu.Proof{
			Id: id,
		}

		keyset := cashu.BasicKeysetResponse{
			Id:          id,
			InputFeePpk: inputFee,
		}

		proofs = append(proofs, proof)
		keysets = append(keysets, keyset)
	}

	fee, _ := cashu.Fees(proofs, keysets)

	if fee != 1 {
		t.Errorf("fee calculation is incorrect: %v. Should be 1", fee)
	}

	for i := 0; i < 3; i++ {
		// add 9 proofs
		proof := cashu.Proof{
			Id: id,
		}

		keyset := cashu.BasicKeysetResponse{
			Id:          id,
			InputFeePpk: inputFee,
		}

		proofs = append(proofs, proof)
		keysets = append(keysets, keyset)
	}
	fee, _ = cashu.Fees(proofs, keysets)

	if fee != 2 {
		t.Errorf("fee calculation is incorrect: %v. Should be 2", fee)
	}

}
