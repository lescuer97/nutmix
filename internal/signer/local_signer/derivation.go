package localsigner

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/lescuer97/nutmix/api/cashu"
)

func GenerateKeysets(versionKey *hdkeychain.ExtendedKey, seed cashu.Seed) ([]cashu.MintKey, error) {
	var keysets = make([]cashu.MintKey, len(seed.Amounts))

	// Get the current time
	currentTime := time.Now()

	// Format the time as a string
	formattedTime := currentTime.Unix()
	for i, value := range seed.Amounts {
		derivationNum := hdkeychain.HardenedKeyStart + uint32(i)
		if seed.Legacy {
			derivationNum = uint32(i)
		}
		// uses the value it represents to derive the key
		childKey, err := versionKey.Derive(derivationNum)
		if err != nil {
			return nil, err
		}
		privKey, err := childKey.ECPrivKey()
		if err != nil {
			return nil, err
		}

		keyset := cashu.MintKey{
			Id:          seed.Id,
			Active:      seed.Active,
			Unit:        seed.Unit,
			Amount:      value,
			PrivKey:     privKey,
			CreatedAt:   formattedTime,
			InputFeePpk: seed.InputFeePpk,
			FinalExpiry: nil,
		}

		keysets[i] = keyset
	}

	return keysets, nil
}

func DeriveKeysetId(keysets []*btcec.PublicKey) (string, error) {
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
	Pubkey *btcec.PublicKey
	Amount uint64
}

func sortPubkeyMapToOrganizedArray(pubkeyMap map[uint64]*btcec.PublicKey) []pubkeyWithAmount {
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

	preimage += fmt.Sprintf("|unit:%s", strings.ToLower(unit))
	if fee > 0 {
		preimage += fmt.Sprintf("|input_fee_ppk:%v", fee)
	}

	if finalExpiry != nil {
		preimage += fmt.Sprintf("|final_expiry:%v", finalExpiry.Unix())
	}

	return preimage
}

func DeriveKeysetIdV2(pubKeysMap map[uint64]*btcec.PublicKey, unit string, fee uint, finalExpiry *time.Time) string {
	arrayPubkeys := sortPubkeyMapToOrganizedArray(pubKeysMap)
	preimage := generateKeysetV2Preimage(arrayPubkeys, unit, fee, finalExpiry)
	hash := sha256.Sum256([]byte(preimage))
	return "01" + hex.EncodeToString(hash[:])
}

func DeriveKeyset(mintKey *hdkeychain.ExtendedKey, seed cashu.Seed) ([]cashu.MintKey, error) {
	paths, err := getDerivationSteps(seed.DerivationPath)
	if err != nil {
		return nil, fmt.Errorf("getDerivationSteps(seed.DerivationPath). %w", err)
	}
	for i := range paths {
		mintKey, err = mintKey.Derive(paths[i])
		if err != nil {
			return nil, fmt.Errorf("derivedKey.Derive(paths[i]). %w", err)
		}
	}

	keyset, err := GenerateKeysets(mintKey, seed)
	if err != nil {
		return nil, fmt.Errorf(`GenerateKeysets(versionKey,GetAmountsForKeysets(), "", unit, 0) %w`, err)
	}

	return keyset, nil
}

func getDerivationSteps(path string) ([]uint32, error) {
	derivationPathSeparation := strings.Split(path, "/")
	derivationPaths := make([]uint32, len(derivationPathSeparation))

	for i := range derivationPathSeparation {
		splitDer := strings.Split(derivationPathSeparation[i], "'")
		if len(splitDer) == 2 {
			derIndex, err := strconv.ParseUint(splitDer[0], 10, 32)
			if err != nil {
				return nil, fmt.Errorf("could not convert derivation path. %w", err)
			}

			// is hardened
			derivationPaths[i] = hdkeychain.HardenedKeyStart + uint32(derIndex)
			continue
		}
		derIndex, err := strconv.ParseUint(splitDer[0], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("could not convert derivation path number. %w", err)
		}
		derivationPaths[i] = uint32(derIndex)
		continue
	}

	return derivationPaths, nil
}

func deriveSeed(seed cashu.Seed, mintKey *hdkeychain.ExtendedKey) ([]cashu.MintKey, error) {
	if seed.Legacy {
		legacyKey, err := legacyGetMintPrivateKey()
		if err != nil {
			return nil, fmt.Errorf("legacyGetMintPrivateKey(). %w", err)
		}
		defer func() {
			legacyKey = nil
		}()
		return legacyDeriveKeyset(legacyKey, seed)
	} else {
		return DeriveKeyset(mintKey, seed)
	}
}

func GetKeysetsFromSeeds(seeds []cashu.Seed, mintKey *hdkeychain.ExtendedKey) (map[string]cashu.MintKeysMap, map[string]cashu.MintKeysMap, error) {
	newKeysets := make(map[string]cashu.MintKeysMap)
	newActiveKeysets := make(map[string]cashu.MintKeysMap)

	for _, seed := range seeds {
		keysets, err := deriveSeed(seed, mintKey)
		if err != nil {
			return newKeysets, newActiveKeysets, fmt.Errorf("deriveSeed(seed, mintKey) %w", err)
		}

		justPubkeys := make([]*btcec.PublicKey, len(keysets))
		pubkeysWithValues := make(map[uint64]*btcec.PublicKey, len(keysets))
		for i := range keysets {
			justPubkeys[i] = keysets[i].GetPubKey()
			pubkeysWithValues[keysets[i].Amount] = keysets[i].GetPubKey()
		}

		newSeedId := ""
		switch seed.Id[:2] {
		case "00":
			newSeedId, err = DeriveKeysetId(justPubkeys)
			if err != nil {
				return nil, nil, fmt.Errorf("cashu.DeriveKeysetId(justPubkeys) %w", err)
			}
		case "01":
			var finalExpiry *time.Time = nil

			if seed.FinalExpiry != nil {
				timeUnix := time.Unix(int64(*seed.FinalExpiry), 0)
				finalExpiry = &timeUnix
			}

			newSeedId = DeriveKeysetIdV2(pubkeysWithValues, seed.Unit, seed.InputFeePpk, finalExpiry)
		default:
			return nil, nil, fmt.Errorf("could not generate a seed id")
		}

		if newSeedId != seed.Id {
			log.Panicf("seed Id generated is not the same as the stored one. \n Stored: %v. \n Generated: %v", seed.Id, newSeedId)
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
