package localsigner

import (
	"encoding/hex"
	"reflect"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lescuer97/nutmix/api/cashu"
)

func TestGenerateKeysetsAndIdGeneration(t *testing.T) {
	keyBytes, err := hex.DecodeString("0000000000000000000000000000000000000000000000000000000000000001")
	if err != nil {
		t.Fatalf("could not decode key %+v", err)
	}
	key, err := hdkeychain.NewMaster(keyBytes, &chaincfg.MainNetParams)
	if err != nil {
		t.Fatalf("could not setup master key %+v", err)
	}

	seed := cashu.Seed{
		Id:          "id",
		Unit:        cashu.Sat.String(),
		Version:     0,
		InputFeePpk: 0,
		Amounts:     cashu.GetAmountsForKeysets(cashu.LegacyMaxKeysetAmount),
		Legacy:      true,
	}

	generatedKeysets, err := GenerateKeysets(key, seed)
	if err != nil {
		t.Fatalf("could not generate keyset %+v", err)
	}

	if len(generatedKeysets) != len(cashu.GetAmountsForKeysets(cashu.LegacyMaxKeysetAmount)) {
		t.Errorf("keyset length is not the same as PossibleKeysetValues length")
	}

	// check if the keyset amount is 0
	if generatedKeysets[0].Amount != 1 {
		t.Errorf("keyset amount is not 0")
	}
	if generatedKeysets[0].Unit != cashu.Sat.String() {
		t.Errorf("keyset unit is not Sat")
	}

	if hex.EncodeToString(generatedKeysets[0].PrivKey.PubKey().SerializeCompressed()) != "03a524f43d6166ad3567f18b0a5c769c6ab4dc02149f4d5095ccf4e8ffa293e785" {
		t.Errorf("keyset id PrivKey is not correct. %+v", hex.EncodeToString(generatedKeysets[0].PrivKey.PubKey().SerializeCompressed()))
	}
	justPubkeys := []*btcec.PublicKey{}

	for i := range generatedKeysets {
		justPubkeys = append(justPubkeys, generatedKeysets[i].GetPubKey())
	}

	keysetId, err := DeriveKeysetId(justPubkeys)
	if err != nil {
		t.Fatalf("could not derive keyset id %+v", err)
	}

	if keysetId != "000fc082ba6bd376" {
		t.Errorf("keyset id is not correct. %v", keysetId)
	}
}

func TestGeneratingAuthKeyset(t *testing.T) {
	seed := make([]byte, 32)
	key, err := hdkeychain.NewMaster(seed, &chaincfg.MainNetParams)
	if err != nil {
		t.Fatalf("could not setup master key %+v", err)
	}
	seedConfig := cashu.Seed{Version: 1, Legacy: true, Unit: cashu.AUTH.String(), DerivationPath: "0/0/0/0", Amounts: []uint64{1}}

	generatedKeysets, err := DeriveKeyset(key, seedConfig)
	if err != nil {
		t.Fatalf("error deriving keyset: %+v", err)
	}

	if len(generatedKeysets) != 1 {
		t.Errorf("there should only be 1 keyset for auth")
	}
	if generatedKeysets[0].Amount != 1 {
		t.Errorf("value should be 1. %v", generatedKeysets[0].Amount)
	}
}

func TestGetDerivationSteps(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		want    []uint32
		wantErr bool
	}{
		{
			name:    "Simple unhardened path",
			path:    "0/0",
			want:    []uint32{0, 0},
			wantErr: false,
		},
		{
			name:    "Hardened path",
			path:    "44'/0'",
			want:    []uint32{hdkeychain.HardenedKeyStart + 44, hdkeychain.HardenedKeyStart + 0},
			wantErr: false,
		},
		{
			name:    "Mixed path",
			path:    "44'/0/1'",
			want:    []uint32{hdkeychain.HardenedKeyStart + 44, 0, hdkeychain.HardenedKeyStart + 1},
			wantErr: false,
		},
		{
			name:    "Invalid path with m prefix",
			path:    "m/44'/0'",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Invalid path format",
			path:    "invalid",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Empty path",
			path:    "",
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getDerivationSteps(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("getDerivationSteps() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getDerivationSteps() = %v, want %v", got, tt.want)
			}
		})
	}
}
