package signer

import (
	"testing"

	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/tyler-smith/go-bip32"
)

func TestGeneratingAuthKeyset(t *testing.T) {
	// setup key
	key, err := bip32.NewMasterKey([]byte("seed"))
	if err != nil {
		t.Errorf("could not setup master key %+v", err)
	}
	Seed := cashu.Seed{Version: 1, Unit: cashu.AUTH.String()}

	generatedKeysets, err := DeriveKeyset(key, Seed)
	if err != nil {
		t.Errorf("Error deriving keyset: %+v", err)
	}

	if len(generatedKeysets) != 1 {
		t.Errorf("There shouls only be 1 keyset for auth")
	}
	if generatedKeysets[0].Amount != 1 {
		t.Errorf("Value should be 1. %v", generatedKeysets[0].Amount)
	}
}
