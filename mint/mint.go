package main

import (
	"fmt"
	"log"
	"os"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lescuer97/nutmix/cashu"
	"github.com/lescuer97/nutmix/comms"
	"github.com/tyler-smith/go-bip32"
)

type KeysetMap map[int]cashu.Keyset

type Mint struct {
	ActiveKeysets map[string]KeysetMap
	Keysets       map[string][]cashu.Keyset
	LightningComs comms.LightingComms
	Network       chaincfg.Params
}

func (m *Mint) SignBlindedMessages(outputs []cashu.BlindedMessage, unit string) ([]cashu.BlindSignature, error) {
	var blindedSignatures []cashu.BlindSignature
	for _, output := range outputs {

		correctKeyset := m.ActiveKeysets[unit][int(output.Amount)]

		blindSignature, err := cashu.GenerateBlindSignature(correctKeyset.PrivKey, output)

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
	default:
		log.Fatalf("Invalid network: %s", network)
	}

	lightningComs, err := comms.SetupLightingComms()

	if err != nil {
		return mint, err
	}

	mint.LightningComs = *lightningComs

	for _, seed := range seeds {
		masterKey, err := bip32.NewMasterKey(seed.Seed)
		if err != nil {
			log.Println(fmt.Errorf("NewMasterKey: %v", err))
			return mint, err
		}
		keysets := cashu.GenerateKeysets(masterKey, cashu.PosibleKeysetValues, seed.Id)

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
