package cashu

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/tyler-smith/go-bip32"
)

var PosibleKeysetValues []int = []int{0, 1, 2, 4, 8, 16, 32, 64, 128, 256, 512, 1024, 2048, 4096, 8192, 16384, 32768, 65536, 131072}

func DeriveKeysetId(keysets []Keyset) (string, error) {
	concatBinaryArray := []byte{}
	for _, keyset := range keysets {
		pubkey := keyset.GetPubKey()

		concatBinaryArray = append(concatBinaryArray, pubkey.SerializeCompressed()...)
	}
	hashedKeysetId := sha256.Sum256(concatBinaryArray)
	hex := hex.EncodeToString(hashedKeysetId[:])

	return "00" + hex[:14], nil
}

func GenerateKeysets(masterKey *bip32.Key, values []int, id string, unit Unit) ([]Keyset, error) {
	var keysets []Keyset

	// Get the current time
	currentTime := time.Now()

	// Format the time as a string
	formattedTime := currentTime.Unix()

	for i, value := range values {
		childKey, err := masterKey.NewChildKey(uint32(i))
		if err != nil {
			return nil, err
		}
		privKey := secp256k1.PrivKeyFromBytes(childKey.Key)

		keyset := Keyset{
			Id:        id,
			Active:    true,
			Unit:      unit.String(),
			Amount:    value,
			PrivKey:   privKey,
			CreatedAt: formattedTime,
		}

		keysets = append(keysets, keyset)
	}

	return keysets, nil
}

func SetUpSeedAndKeyset(masterKey *bip32.Key, version int, unit Unit) (Seed, error) {
	// Get the current time
	currentTime := time.Now().Unix()

	// Derive key from version

	versionKey, err := masterKey.NewChildKey(uint32(version))

	if err != nil {
		return Seed{}, fmt.Errorf("Error deriving key from version: %v", err)
	}

	list_of_keys, err := GenerateKeysets(versionKey, PosibleKeysetValues, "", unit)

	if err != nil {
		return Seed{}, err
	}

	id, err := DeriveKeysetId(list_of_keys)

	if err != nil {
		return Seed{}, err
	}

	newSeed := Seed{
		Seed:      versionKey.Key,
		Active:    true,
		CreatedAt: currentTime,
		Unit:      unit.String(),
		Id:        id,
		Version:   version,
	}

	return newSeed, nil
}

func DeriveSeedsFromKey(keyFromMint string, version int, availableSeeds []Unit) ([]Seed, error) {
	key_bytes, err := hex.DecodeString(keyFromMint)

	var seeds []Seed

	if err != nil {
		return nil, fmt.Errorf("Error decoding mint private key: %+v ", err)
	}

	masterKey, err := bip32.NewMasterKey(key_bytes)

	if err != nil {
		return nil, fmt.Errorf("Error creating master key: %+v ", err)
	}

	for _, seedDerivationPath := range availableSeeds {

		// Set the derivation for each type of ecash. Ex: sat, usd, eur
		seedKey, err := masterKey.NewChildKey(uint32(seedDerivationPath))

		if err != nil {
			return nil, fmt.Errorf("could not generate derivation por seed: %+v ", err)
		}

		seed, err := SetUpSeedAndKeyset(seedKey, version, seedDerivationPath)

		seeds = append(seeds, seed)

	}

	return seeds, nil
}
