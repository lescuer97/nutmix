package cashu

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/tyler-smith/go-bip32"
)

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

func GenerateKeysets(masterKey *bip32.Key, values []uint64, id string, unit Unit, inputFee int) ([]Keyset, error) {
	var keysets []Keyset

	// Get the current time
	currentTime := time.Now()

	// Format the time as a string
	formattedTime := currentTime.Unix()

	for i, value := range values {
		// uses the value it represents to derive the key
		childKey, err := masterKey.NewChildKey(uint32(i))
		if err != nil {
			return nil, err
		}
		privKey := secp256k1.PrivKeyFromBytes(childKey.Key)

		keyset := Keyset{
			Id:          id,
			Active:      true,
			Unit:        unit.String(),
			Amount:      value,
			PrivKey:     privKey,
			CreatedAt:   formattedTime,
			InputFeePpk: inputFee,
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
		return Seed{}, fmt.Errorf("Error deriving key from version: %w", err)
	}

	list_of_keys, err := GenerateKeysets(versionKey, GetAmountsForKeysets(), "", unit, 0)

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
		Encrypted: false,
	}

	return newSeed, nil
}

func DeriveSeedsFromKey(keyFromMint string, version int, availableSeeds []Unit) ([]Seed, error) {

	var seeds []Seed

	for _, seedDerivationPath := range availableSeeds {

		seed, err := DeriveIndividualSeedFromKey(keyFromMint, version, seedDerivationPath)

		if err != nil {
			return seeds, fmt.Errorf("DeriveIndividualSeedFromKey(keyFromMint, version, seedDerivationPath): %w ", err)
		}

		// encrypt seeds before saving

		seeds = append(seeds, seed)

	}

	return seeds, nil
}

func DeriveIndividualSeedFromKey(keyFromMint string, version int, unit Unit) (Seed, error) {
	var seed Seed
	key_bytes, err := hex.DecodeString(keyFromMint)
	if err != nil {
		return seed, fmt.Errorf("Error decoding mint private key: %+v ", err)
	}

	masterKey, err := bip32.NewMasterKey(key_bytes)

	if err != nil {
		return seed, fmt.Errorf("Error creating master key: %w ", err)
	}

	// Set the derivation for each type of ecash. Ex: sat, usd, eur
	seedKey, err := masterKey.NewChildKey(uint32(unit))

	if err != nil {
		return seed, fmt.Errorf("could not generate derivation por seed: %w ", err)
	}

	seed, err = SetUpSeedAndKeyset(seedKey, version, unit)

	// Encrypt seed and set encrypted to true
	err = seed.EncryptSeed(keyFromMint)
	if err != nil {
		return seed, fmt.Errorf("Error encrypting seed: %w", err)
	}

	seed.Encrypted = true

	return seed, nil

}

const MaxKeysetAmount int = 64

func GetAmountsForKeysets() []uint64 {
	keys := make([]uint64, 0)

	for i := 0; i < MaxKeysetAmount; i++ {
		keys = append(keys, uint64(math.Pow(2, float64(i))))
	}
	return keys
}

// Given an amount, it returns list of amounts e.g 13 -> [1, 4, 8]
// that can be used to build blinded messages or split operations.
// from nutshell implementation
func AmountSplit(amount uint64) []uint64 {
	rv := make([]uint64, 0)
	for pos := 0; amount > 0; pos++ {
		if amount&1 == 1 {
			rv = append(rv, 1<<pos)
		}
		amount >>= 1
	}
	return rv
}
