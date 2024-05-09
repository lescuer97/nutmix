package cashu

import (
	"crypto/sha256"
	"encoding/hex"
	"log"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/tyler-smith/go-bip32"
)

var PosibleKeysetValues []int = []int{1, 2, 4, 8, 16, 32, 64, 128, 256, 512, 1024, 2048, 4096, 8192, 16384, 32768, 65536, 131072}

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

func GenerateKeysets(masterKey *bip32.Key, values []int, id string) []Keyset {
	var keysets []Keyset

	// Get the current time
	currentTime := time.Now()

	// Format the time as a string
	formattedTime := currentTime.Unix()

	for i, value := range values {
		childKey, err := masterKey.NewChildKey(uint32(i))
		if err != nil {
			log.Fatal("Error generating child key: ", err)
		}
        privKey := secp256k1.PrivKeyFromBytes(childKey.Key)

		keyset := Keyset{
			Id:        id,
			Active:    true,
			Unit:      Sat.String(),
			Amount:    value,
			PrivKey:    privKey,
			CreatedAt: formattedTime,
		}

		keysets = append(keysets, keyset)
	}

	return keysets
}
