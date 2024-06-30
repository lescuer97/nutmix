package mint

import (
	"context"
	"os"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/tyler-smith/go-bip32"
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

	err := os.Setenv(NETWORK_ENV, "regtest")

	if err != nil {
		t.Errorf("could not set network %v", err)

	}
	err = os.Setenv(MINT_LIGHTNING_BACKEND_ENV, "FakeWallet")
	if err != nil {
		t.Errorf("could not set lightning backend %v", err)

	}

	seeds := []cashu.Seed{
		seed,
	}


	ctx := context.Background()
	ctx = context.WithValue(ctx, MINT_LIGHTNING_BACKEND_ENV, os.Getenv(MINT_LIGHTNING_BACKEND_ENV))
	ctx = context.WithValue(ctx, NETWORK_ENV, os.Getenv(NETWORK_ENV))

	mint_privkey := os.Getenv("MINT_PRIVATE_KEY")

	mint, err := SetUpMint(ctx, mint_privkey, seeds)

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
