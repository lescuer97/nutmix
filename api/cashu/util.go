package cashu

import (
	"crypto/rand"
	"encoding/hex"
	"strconv"
)

func OrderKeysetByUnit(keysets []Keyset) KeysResponse {
	var typesOfUnits = make(map[string][]Keyset)

	for _, keyset := range keysets {
		if len(typesOfUnits[keyset.Unit]) == 0 {
			typesOfUnits[keyset.Unit] = append(typesOfUnits[keyset.Unit], keyset)
			continue
		} else {
			typesOfUnits[keyset.Unit] = append(typesOfUnits[keyset.Unit], keyset)
		}
	}

	res := make(map[string][]KeysetResponse)

	res["keysets"] = []KeysetResponse{}

	for _, value := range typesOfUnits {
		var keysetResponse KeysetResponse
		keysetResponse.Id = value[0].Id
		keysetResponse.Unit = value[0].Unit
		keysetResponse.Keys = make(map[string]string)
		keysetResponse.InputFeePpk = value[0].InputFeePpk

		for _, keyset := range value {

			keysetResponse.Keys[strconv.FormatUint(keyset.Amount, 10)] = hex.EncodeToString(keyset.PrivKey.PubKey().SerializeCompressed())
		}

		res["keysets"] = append(res["keysets"], keysetResponse)
	}
	return res

}
func GenerateNonceHex() (string, error) {

	// generate random Nonce
	nonce := make([]byte, 32)  // create a slice with length 16 for the nonce
	_, err := rand.Read(nonce) // read random bytes into the nonce slice
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(nonce), nil
}

func Fees(proofs []Proof, keysets []Keyset) (int, error) {
	totalFees := 0

	var keysetToUse Keyset
	for _, proof := range proofs {
		// find keyset to compare to fees if keyset id is not found throw error
		// only check for new keyset if proofs id is different
		if keysetToUse.Id != proof.Id {
			for _, keyset := range keysets {
				if keyset.Id == proof.Id {

					keysetToUse = keyset
				}
			}
			if keysetToUse.Id != proof.Id {
				return 0, ErrKeysetForProofNotFound

			}

		}

		totalFees += keysetToUse.InputFeePpk

	}

	totalFees = (totalFees + 999) / 1000

	return totalFees, nil

}
