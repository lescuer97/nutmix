package main

import (
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/tyler-smith/go-bip32"
	"os"
	"testing"
)

func TestSetUpMint(t *testing.T) {

	seedInfo := []byte("seed")

	seed := cashu.Seed{
		Seed:      seedInfo,
		Active:    true,
		CreatedAt: 12345,
		Unit:      cashu.Sat.String(),
		Id:        "id",
	}

	err := os.Setenv("NETWORK", "regtest")

	if err != nil {
		t.Errorf("could not set network %v", err)

	}
	err = os.Setenv("MINT_LIGHTNING_BACKEND", "FakeWallet")
	if err != nil {
		t.Errorf("could not set lightning backend %v", err)

	}

	seeds := []cashu.Seed{
		seed,
	}

	mint, err := SetUpMint(seeds)

	if err != nil {
		t.Errorf("could not setup mint: %+v", err)
	}

	// setup key to test against
	masterKey, err := bip32.NewMasterKey(seedInfo)
	if err != nil {
		t.Errorf("could not setup master key %+v", err)
	}
	childKey, err := masterKey.NewChildKey(1)
	if err != nil {
		t.Errorf("could not set lightning backend %v", err)
	}
	privKey := secp256k1.PrivKeyFromBytes(childKey.Key)

	// compare keys for value 1 sat
	if mint.ActiveKeysets[cashu.Sat.String()][1].PrivKey.Key.String() != privKey.Key.String() {
		t.Errorf("Keys are not the same. \n\n Should be: %x  \n\n Is: %x ", privKey.Key.String(), mint.ActiveKeysets[cashu.Sat.String()][1].PrivKey.Key.String())
	}

	// compare keys for value 2 sat
	childKeyTwo, err := masterKey.NewChildKey(2)
	if err != nil {
		t.Errorf("could not set lightning backend %v", err)
	}
	privKeyTwo := secp256k1.PrivKeyFromBytes(childKeyTwo.Key)

	if mint.ActiveKeysets[cashu.Sat.String()][2].PrivKey.Key.String() != privKeyTwo.Key.String() {
		t.Errorf("Keys are not the same. \n\n Should be: %x  \n\n Is: %x ", privKeyTwo.Key.String(), mint.ActiveKeysets[cashu.Sat.String()][2].PrivKey.Key.String())
	}

	// checks for the last key available in the keyset
	childKeyLast, err := masterKey.NewChildKey(18)
	if err != nil {
		t.Errorf("could not set lightning backend %v", err)
	}
	privKeyLast := secp256k1.PrivKeyFromBytes(childKeyLast.Key)

	if mint.ActiveKeysets[cashu.Sat.String()][131072].PrivKey.Key.String() != privKeyLast.Key.String() {
		t.Errorf("Keys are not the same. \n\n Should be: %x  \n\n Is: %x ", privKeyLast.Key.String(), mint.ActiveKeysets[cashu.Sat.String()][131072].PrivKey.Key.String())
	}
}
