package cashu

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/tyler-smith/go-bip32"
	"math"
	"time"
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

func GenerateKeysets(versionKey *bip32.Key, values []uint64, id string, unit Unit, inputFee uint, active bool) ([]Keyset, error) {
	var keysets []Keyset

	// Get the current time
	currentTime := time.Now()

	// Format the time as a string
	formattedTime := currentTime.Unix()

	for i, value := range values {
		// uses the value it represents to derive the key
		childKey, err := versionKey.NewChildKey(uint32(i))
		if err != nil {
			return nil, err
		}
		privKey := secp256k1.PrivKeyFromBytes(childKey.Key)

		keyset := Keyset{
			Id:          id,
			Active:      active,
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
