package signer

import (
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/tyler-smith/go-bip32"
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
		keysetResponse.Unit = value[0].Unit
		keysetResponse.Keys = make(map[string]string)
		keysetResponse.InputFeePpk = value[0].InputFeePpk

		for _, keyset := range value {

			keysetResponse.Keys[strconv.FormatUint(keyset.Amount, 10)] = hex.EncodeToString(keyset.PrivKey.PubKey().SerializeCompressed())
		}

		res.Keysets = append(res.Keysets, keysetResponse)
	}
	return res

}
func DeriveKeyset(mintKey *bip32.Key, seed cashu.Seed) ([]cashu.MintKey, error) {
	unit, err := cashu.UnitFromString(seed.Unit)
	if err != nil {
		return nil, fmt.Errorf("UnitFromString(seed.Unit) %w", err)
	}

	unitKey, err := mintKey.NewChildKey(uint32(unit.EnumIndex()))

	if err != nil {

		return nil, fmt.Errorf("mintKey.NewChildKey(uint32(unit.EnumIndex())). %w", err)
	}

	versionKey, err := unitKey.NewChildKey(uint32(seed.Version))
	if err != nil {
		return nil, fmt.Errorf("mintKey.NewChildKey(uint32(seed.Version)) %w", err)
	}

	amounts := cashu.GetAmountsForKeysets()

	if unit == cashu.AUTH {
		amounts = []uint64{amounts[0]}
	}

	keyset, err := cashu.GenerateKeysets(versionKey, amounts, seed.Id, unit, seed.InputFeePpk, seed.Active)
	if err != nil {
		return nil, fmt.Errorf(`GenerateKeysets(versionKey,GetAmountsForKeysets(), "", unit, 0) %w`, err)
	}

	return keyset, nil
}

func GetKeysetsFromSeeds(seeds []cashu.Seed, mintKey *bip32.Key) (map[string]cashu.MintKeysMap, map[string]cashu.MintKeysMap, error) {
	newKeysets := make(map[string]cashu.MintKeysMap)
	newActiveKeysets := make(map[string]cashu.MintKeysMap)

	for _, seed := range seeds {
		keysets, err := DeriveKeyset(mintKey, seed)
		if err != nil {
			return newKeysets, newActiveKeysets, fmt.Errorf("DeriveKeyset(mintKey, seed) %w", err)
		}

		mintkeyMap := make(cashu.MintKeysMap)
		for _, keyset := range keysets {
			mintkeyMap[keyset.Amount] = keyset
		}

		if seed.Active {
			newActiveKeysets[seed.Id] = mintkeyMap
		}

		newKeysets[seed.Id] = mintkeyMap
	}
	return newKeysets, newActiveKeysets, nil

}
