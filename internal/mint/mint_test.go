package mint

import (
	"context"
	"encoding/hex"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/tyler-smith/go-bip32"
	"os"
	"testing"
)

const MintPrivateKey string = "0000000000000000000000000000000000000000000000000000000000000001"

func TestSetUpMint(t *testing.T) {
	decodedPrivKey, err := hex.DecodeString(MintPrivateKey)
	if err != nil {
		t.Errorf("hex.DecodeString(masterKey) %+v", err)
	}

	parsedPrivateKey := secp256k1.PrivKeyFromBytes(decodedPrivKey)

	seedInfo := []byte("seed")

	seed := cashu.Seed{
		Seed:      seedInfo,
		Active:    true,
		CreatedAt: 12345,
		Unit:      cashu.Sat.String(),
		Id:        "id",
	}
	t.Setenv("MINT_PRIVATE_KEY", MintPrivateKey)

	seed.EncryptSeed(parsedPrivateKey)

	t.Setenv(NETWORK_ENV, "regtest")

	t.Setenv(MINT_LIGHTNING_BACKEND_ENV, "FakeWallet")

	seeds := []cashu.Seed{
		seed,
	}

	ctx := context.Background()
	ctx = context.WithValue(ctx, MINT_LIGHTNING_BACKEND_ENV, os.Getenv(MINT_LIGHTNING_BACKEND_ENV))
	ctx = context.WithValue(ctx, NETWORK_ENV, os.Getenv(NETWORK_ENV))

	config, err := SetUpConfigFile()

	if err != nil {
		t.Errorf("could not setup config file: %+v", err)
	}

	mint, err := SetUpMint(ctx, parsedPrivateKey, seeds, config)

	if err != nil {
		t.Errorf("could not setup mint: %+v", err)
	}

	// setup key to test against
	masterKey, err := bip32.NewMasterKey(seedInfo)
	if err != nil {
		t.Errorf("could not setup master key %+v", err)
	}
	childKey, err := masterKey.NewChildKey(0)
	if err != nil {
		t.Errorf("could not set lightning backend %v", err)
	}
	privKey := secp256k1.PrivKeyFromBytes(childKey.Key)

	// compare keys for value 1 sat
	if mint.ActiveKeysets[cashu.Sat.String()][1].PrivKey.Key.String() != privKey.Key.String() {
		t.Errorf("Keys are not the same. \n\n Should be: %x  \n\n Is: %x ", privKey.Key.String(), mint.ActiveKeysets[cashu.Sat.String()][1].PrivKey.Key.String())
	}

	// compare keys for value 2 sat
	childKeyTwo, err := masterKey.NewChildKey(1)
	if err != nil {
		t.Errorf("could not set lightning backend %v", err)
	}
	privKeyTwo := secp256k1.PrivKeyFromBytes(childKeyTwo.Key)

	if mint.ActiveKeysets[cashu.Sat.String()][2].PrivKey.Key.String() != privKeyTwo.Key.String() {
		t.Errorf("Keys are not the same. \n\n Should be: %x  \n\n Is: %x ", privKeyTwo.Key.String(), mint.ActiveKeysets[cashu.Sat.String()][2].PrivKey.Key.String())
	}
}

func TestDeriveKeysetFromSingleSeed(t *testing.T) {
	decodedPrivKey, err := hex.DecodeString(MintPrivateKey)
	if err != nil {
		t.Errorf("hex.DecodeString(masterKey) %+v", err)
	}

	parsedPrivateKey := secp256k1.PrivKeyFromBytes(decodedPrivKey)

	seed1, err := cashu.DeriveIndividualSeedFromKey(parsedPrivateKey, 1, cashu.Sat)
	if err != nil {
		t.Errorf("cashu.DeriveIndividualSeedFromKey(parsedPrivateKey, 1, cashu.Sat) %+v", err)
	}

	seeds := []cashu.Seed{seed1}

	keyset, activeKeyset, err := DeriveKeysetFromSeeds(seeds, parsedPrivateKey)

	if err != nil {
		t.Errorf("DeriveKeysetFromSeeds(singleSeedSlice, parsedPrivateKey) %+v", err)
	}

	if keyset[cashu.Sat.String()][0].Id != "00bfa73302d12ffd" {
		t.Errorf("Incorrect keyset id %+v", keyset[cashu.Sat.String()][0].Id)
	}

	if hex.EncodeToString(keyset[cashu.Sat.String()][0].GetPubKey().SerializeCompressed()) != "028188029c28c1dc53cd1e53d1596360d1c7600a0ab0efeed0c3907f3faecbd144" {
		t.Errorf("incorrect pubkey %+v", hex.EncodeToString(keyset[cashu.Sat.String()][0].GetPubKey().SerializeCompressed()))
	}

	if activeKeyset[cashu.Sat.String()][1].Id != "00bfa73302d12ffd" {
		t.Errorf("Incorrect keyset id %+v", keyset[cashu.Sat.String()][0].Id)
	}

	if hex.EncodeToString(activeKeyset[cashu.Sat.String()][1].PrivKey.PubKey().SerializeCompressed()) != "028188029c28c1dc53cd1e53d1596360d1c7600a0ab0efeed0c3907f3faecbd144" {
		t.Errorf("incorrect pubkey %+v", hex.EncodeToString(keyset[cashu.Sat.String()][0].GetPubKey().SerializeCompressed()))
	}

}

func TestDeriveKeysetFromTwoSeeds(t *testing.T) {
	decodedPrivKey, err := hex.DecodeString(MintPrivateKey)
	if err != nil {
		t.Errorf("hex.DecodeString(masterKey) %+v", err)
	}

	parsedPrivateKey := secp256k1.PrivKeyFromBytes(decodedPrivKey)

	seed1, err := cashu.DeriveIndividualSeedFromKey(parsedPrivateKey, 1, cashu.Sat)
	if err != nil {
		t.Errorf("cashu.DeriveIndividualSeedFromKey(parsedPrivateKey, 1, cashu.Sat) %+v", err)
	}

	seed2, err := cashu.DeriveIndividualSeedFromKey(parsedPrivateKey, 2, cashu.Sat)
	if err != nil {
		t.Errorf("cashu.DeriveIndividualSeedFromKey(parsedPrivateKey, 2, cashu.Sat) %+v", err)
	}

	seeds := []cashu.Seed{seed1, seed2}

	keyset, activeKeyset, err := DeriveKeysetFromSeeds(seeds, parsedPrivateKey)

	if err != nil {
		t.Errorf("DeriveKeysetFromSeeds(singleSeedSlice, parsedPrivateKey) %+v", err)
	}

	if keyset[cashu.Sat.String()][0].Id != "00bfa73302d12ffd" {
		t.Errorf("Incorrect keyset id %+v", keyset[cashu.Sat.String()][0].Id)
	}

	if hex.EncodeToString(keyset[cashu.Sat.String()][0].GetPubKey().SerializeCompressed()) != "028188029c28c1dc53cd1e53d1596360d1c7600a0ab0efeed0c3907f3faecbd144" {
		t.Errorf("incorrect pubkey %+v", hex.EncodeToString(keyset[cashu.Sat.String()][0].GetPubKey().SerializeCompressed()))
	}

	if activeKeyset[cashu.Sat.String()][1].Id != "00ff6bfa1ff72a5c" {
		t.Errorf("Incorrect keyset id %+v", keyset[cashu.Sat.String()][0].Id)
	}

	if hex.EncodeToString(activeKeyset[cashu.Sat.String()][1].PrivKey.PubKey().SerializeCompressed()) != "02d127729e801487c422462b75e32d225c3d315811131dcea429e4630546ec98e3" {
		t.Errorf("incorrect pubkey %+v", hex.EncodeToString(keyset[cashu.Sat.String()][0].GetPubKey().SerializeCompressed()))
	}

}
