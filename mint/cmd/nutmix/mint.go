package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/comms"
	"github.com/lescuer97/nutmix/pkg/crypto"
	"github.com/tyler-smith/go-bip32"
)

type KeysetMap map[int]cashu.Keyset

type Mint struct {
	ActiveKeysets map[string]KeysetMap
	Keysets       map[string][]cashu.Keyset
	LightningComs comms.LightingComms
	Network       chaincfg.Params
}

// errors types for validation

var (
	ErrKeysetNotFound         = errors.New("Keyset not found")
	ErrKeysetForProofNotFound = errors.New("Keyset for proof not found")
	ErrInvalidProof           = errors.New("Invalid proof")
)

func (m *Mint) ValidateProof(proof cashu.Proof) error {
	var keysetToUse cashu.Keyset
	for _, keyset := range m.Keysets[cashu.Sat.String()] {
		if keyset.Amount == int(proof.Amount) && keyset.Id == proof.Id {
			keysetToUse = keyset
			break
		}
	}

	// check if keysetToUse is not assigned
	if keysetToUse.Id == "" {
		return ErrKeysetForProofNotFound
	}

	parsedBlinding, err := hex.DecodeString(proof.C)

	if err != nil {
		log.Printf("hex.DecodeString: %+v", err)
		return err
	}

	pubkey, err := secp256k1.ParsePubKey(parsedBlinding)
	if err != nil {
		log.Printf("secp256k1.ParsePubKey: %+v", err)
		return err
	}

	verified := crypto.Verify(proof.Secret, keysetToUse.PrivKey, pubkey)

	if !verified {
		return ErrInvalidProof
	}

	return nil
}

func (m *Mint) SignBlindedMessages(outputs []cashu.BlindedMessage, unit string) ([]cashu.BlindSignature, error) {
	var blindedSignatures []cashu.BlindSignature
	for _, output := range outputs {

		correctKeyset := m.ActiveKeysets[unit][int(output.Amount)]

		blindSignature, err := output.GenerateBlindSignature(correctKeyset.PrivKey)

		if err != nil {
			log.Println(fmt.Errorf("GenerateBlindSignature: %w", err))
			return nil, err
		}

		blindedSignatures = append(blindedSignatures, blindSignature)

	}
	return blindedSignatures, nil
}

func (m *Mint) GetKeysetById(unit string, id string) ([]cashu.Keyset, error) {

	allKeys := m.Keysets[unit]
	var keyset []cashu.Keyset

	for _, key := range allKeys {

		if key.Id == id {
			keyset = append(keyset, key)
		}
	}

	return keyset, nil
}

func (m *Mint) OrderActiveKeysByUnit() cashu.KeysResponse {
	// convert map to slice
	var keys []cashu.Keyset
	for _, keyset := range m.ActiveKeysets {
		for _, key := range keyset {
			keys = append(keys, key)
		}
	}

	orderedKeys := cashu.OrderKeysetByUnit(keys)

	return orderedKeys
}

func SetUpMint(seeds []cashu.Seed) (Mint, error) {
	mint := Mint{
		ActiveKeysets: make(map[string]KeysetMap),
		Keysets:       make(map[string][]cashu.Keyset),
	}

	network := os.Getenv("NETWORK")
	switch network {
	case "testnet":
		mint.Network = chaincfg.TestNet3Params
	case "mainnet":
		mint.Network = chaincfg.MainNetParams
	case "regtest":
		mint.Network = chaincfg.RegressionNetParams
	case "signet":
		mint.Network = chaincfg.SigNetParams
	default:
		return mint, fmt.Errorf("Invalid network: %s", network)
	}

	lightningComs, err := comms.SetupLightingComms()

	if err != nil {
		return mint, err
	}

	mint.LightningComs = *lightningComs

	// uses seed to generate the keysets
	for _, seed := range seeds {
		masterKey, err := bip32.NewMasterKey(seed.Seed)
		if err != nil {
			log.Println(fmt.Errorf("NewMasterKey: %v", err))
			return mint, err
		}
		keysets, err := cashu.GenerateKeysets(masterKey, cashu.PosibleKeysetValues, seed.Id)

		if err != nil {
			return mint, fmt.Errorf("GenerateKeysets: %v", err)
		}

		if seed.Active {
			mint.ActiveKeysets[seed.Unit] = make(KeysetMap)
			for _, keyset := range keysets {
				mint.ActiveKeysets[seed.Unit][keyset.Amount] = keyset
			}

		}

		mint.Keysets[seed.Unit] = append(mint.Keysets[seed.Unit], keysets...)
	}

	return mint, nil
}
