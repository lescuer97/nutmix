package cashu

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/tyler-smith/go-bip32"
)

// Deprecated: Use DeriveKeysetIdV2 instead. This function generates V1 keyset IDs which are less unique.
func DeriveKeysetId(keysets []*secp256k1.PublicKey) (string, error) {
	concatBinaryArray := []byte{}
	for _, pubkey := range keysets {
		if pubkey == nil {
			panic("pubkey should have never been nil at this time")
		}
		concatBinaryArray = append(concatBinaryArray, pubkey.SerializeCompressed()...)
	}
	hashedKeysetId := sha256.Sum256(concatBinaryArray)
	hex := hex.EncodeToString(hashedKeysetId[:])

	return "00" + hex[:14], nil
}

type pubkeyWithAmount struct {
	Pubkey *secp256k1.PublicKey
	Amount uint64
}

func sortPubkeyMapToOrganizedArray(pubkeyMap map[uint64]*secp256k1.PublicKey) []pubkeyWithAmount {
	arrayPubkeys := make([]pubkeyWithAmount, len(pubkeyMap))

	i := 0
	for amount, key := range pubkeyMap {
		arrayPubkeys[i] = pubkeyWithAmount{
			Amount: amount,
			Pubkey: key,
		}
		i++
	}

	slices.SortFunc(arrayPubkeys, func(a, b pubkeyWithAmount) int {
		return int(a.Amount) - int(b.Amount)
	})
	return arrayPubkeys

}

func generateKeysetV2Preimage(sortedPubkeyArray []pubkeyWithAmount, unit string, fee uint, finalExpiry *time.Time) string {
	preimage := ""
	for i := range sortedPubkeyArray {
		preimage += fmt.Sprintf("%v:%x", sortedPubkeyArray[i].Amount, sortedPubkeyArray[i].Pubkey.SerializeCompressed())
		if i != len(sortedPubkeyArray)-1 {
			preimage += ","
		}
	}

	preimage += fmt.Sprintf("|unit:%s", unit)
	if fee > 0 {
		preimage += fmt.Sprintf("|input_fee_ppk:%v", fee)
	}

	if finalExpiry != nil {
		preimage += fmt.Sprintf("|final_expiry:%v", finalExpiry.Unix())
	}

	return preimage
}

func DeriveKeysetIdV2(pubKeysMap map[uint64]*secp256k1.PublicKey, unit string, fee uint, finalExpiry *time.Time) string {
	arrayPubkeys := sortPubkeyMapToOrganizedArray(pubKeysMap)
	preimage := generateKeysetV2Preimage(arrayPubkeys, unit, fee, finalExpiry)
	hash := sha256.Sum256([]byte(preimage))
	return "01" + hex.EncodeToString(hash[:])
}

func GenerateKeysets(versionKey *bip32.Key, values []uint64, seed Seed) ([]MintKey, error) {
	var keysets []MintKey

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

		keyset := MintKey{
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
