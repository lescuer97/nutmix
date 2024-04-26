package cashu

import (
	"encoding/hex"
	"strconv"

	"github.com/tyler-smith/go-bip32"
)

func OrderKeysetByUnit(keysets []Keyset) (map[string][]KeysetResponse, error) {
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

		for _, keyset := range value {
			privkey, err := bip32.B58Deserialize(keyset.PrivKey)
			if err != nil {

				return nil, err
			}
			keysetResponse.Keys[strconv.Itoa(keyset.Amount)] = hex.EncodeToString(privkey.PublicKey().Key)
		}

		res["keysets"] = append(res["keysets"], keysetResponse)
	}
	return res, nil

}
