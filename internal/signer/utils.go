package signer

import (
	"encoding/hex"
	"github.com/lescuer97/nutmix/api/cashu"
)

func OrderKeysetByUnit(keysets []cashu.MintKey) GetKeysResponse {
	var typesOfUnits = make(map[string][]cashu.MintKey)

	for _, keyset := range keysets {
		if len(typesOfUnits[keyset.Unit]) == 0 {
			typesOfUnits[keyset.Unit] = append(typesOfUnits[keyset.Unit], keyset)
			continue
		} else {
			typesOfUnits[keyset.Unit] = append(typesOfUnits[keyset.Unit], keyset)
		}
	}

	res := GetKeysResponse{}

	res.Keysets = []KeysetResponse{}

	for _, value := range typesOfUnits {
		var keysetResponse KeysetResponse
		keysetResponse.Id = value[0].Id
		keysetResponse.Active = value[0].Active
		keysetResponse.Unit = value[0].Unit
		keysetResponse.Keys = make(map[uint64]string)
		keysetResponse.InputFeePpk = value[0].InputFeePpk

		for _, keyset := range value {

			keysetResponse.Keys[keyset.Amount] = hex.EncodeToString(keyset.PrivKey.PubKey().SerializeCompressed())
		}

		res.Keysets = append(res.Keysets, keysetResponse)
	}
	return res

}
