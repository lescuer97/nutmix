package localsigner

import (
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/tyler-smith/go-bip32"
)

func legacyGetMintPrivateKey() (*bip32.Key, error) {
	mint_privkey := os.Getenv("MINT_PRIVATE_KEY")
	if mint_privkey == "" {
		return nil, fmt.Errorf(`os.Getenv("MINT_PRIVATE_KEY")`)
	}
	defer func() {
		mint_privkey = ""
	}()

	decodedPrivKey, err := hex.DecodeString(mint_privkey)
	if err != nil {
		return nil, fmt.Errorf(`hex.DecodeString(mint_privkey). %w`, err)
	}
	mintKey := secp256k1.PrivKeyFromBytes(decodedPrivKey)

	masterKey, err := bip32.NewMasterKey(mintKey.Serialize())
	if err != nil {
		return nil, fmt.Errorf(" bip32.NewMasterKey(privateKey.Serialize()). %w", err)
	}
	return masterKey, nil

}

func legacyDeriveKeyset(mintKey *bip32.Key, seed cashu.Seed) ([]cashu.MintKey, error) {
	unit, err := cashu.UnitFromString(seed.Unit)
	if err != nil {
		return nil, fmt.Errorf("UnitFromString(seed.Unit) %w", err)
	}

	unitKey, err := mintKey.NewChildKey(uint32(unit.EnumIndex()))

	if err != nil {

		return nil, fmt.Errorf("mintKey.NewChildKey(uint32(unit.EnumIndex())). %w", err)
	}

	versionKey, err := unitKey.NewChildKey(seed.Version)
	if err != nil {
		return nil, fmt.Errorf("mintKey.NewChildKey(uint32(seed.Version)) %w", err)
	}

	keyset, err := legacyGenerateKeysets(versionKey, seed)
	if err != nil {
		return nil, fmt.Errorf(`GenerateKeysets(versionKey,GetAmountsForKeysets(), "", unit, 0) %w`, err)
	}

	return keyset, nil
}

func legacyGenerateKeysets(versionKey *bip32.Key, seed cashu.Seed) ([]cashu.MintKey, error) {
	var keysets []cashu.MintKey

	// Get the current time
	currentTime := time.Now()

	// Format the time as a string
	formattedTime := currentTime.Unix()
	for i, value := range seed.Amounts {
		// uses the value it represents to derive the key
		childKey, err := versionKey.NewChildKey(uint32(i))
		if err != nil {
			return nil, err
		}
		privKey := secp256k1.PrivKeyFromBytes(childKey.Key)

		keyset := cashu.MintKey{
			Id:          seed.Id,
			Active:      seed.Active,
			Unit:        seed.Unit,
			Amount:      value,
			PrivKey:     privKey,
			CreatedAt:   formattedTime,
			InputFeePpk: seed.InputFeePpk,
		}

		keysets = append(keysets, keyset)
	}

	return keysets, nil
}
