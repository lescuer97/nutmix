package localsigner

import (
	"context"
	"testing"

	"github.com/lescuer97/nutmix/api/cashu"
	mockdb "github.com/lescuer97/nutmix/internal/database/mock_db"
)

const MintPrivateKey string = "0000000000000000000000000000000000000000000000000000000000000001"

func TestRotateUnexistingSeedUnit(t *testing.T) {
	db := mockdb.MockDB{}
	t.Setenv("MINT_PRIVATE_KEY", MintPrivateKey)
	localsigner, err := SetupLocalSigner(&db)
	if err != nil {
		t.Fatalf("SetupLocalSigner(&db) %+v", err)
	}
	_, err = localsigner.getSignerPrivateKey()
	if err != nil {
		t.Fatalf("getSignerPrivateKey failed: %v", err)
	}

	err = localsigner.RotateKeyset(cashu.Msat, uint(100), 240)
	if err != nil {
		t.Fatalf("localsigner.RotateKeyset(cashu.Msat, uint(100)) %+v", err)
	}

	err = localsigner.RotateKeyset(cashu.Sat, uint(100), 240)
	if err != nil {
		t.Fatalf("localsigner.RotateKeyset(cashu.Sat, uint(100)) %+v", err)
	}

	keys, err := localsigner.GetKeysets()
	if err != nil {
		t.Fatalf("localsigner.GetKeys() %+v", err)

	}
	if len(keys.Keysets) != 3 {
		t.Errorf("Version should be 3. it's %v", len(keys.Keysets))
	}
	ctx := context.Background()
	tx, err := localsigner.db.GetTx(ctx)
	if err != nil {
		t.Fatalf("localsigner.db.GetTx(ctx) %+v", err)
	}

	msatSeeds, err := db.GetSeedsByUnit(tx, cashu.Msat)
	if err != nil {
		t.Fatalf("db.GetSeedsByUnit(cashu.Msat) %+v", err)
	}

	if msatSeeds[0].Version != 1 {
		t.Error("Version should be 1")
	}
	if msatSeeds[0].InputFeePpk != uint(100) {
		t.Errorf("Input fee should be 100. its %v", msatSeeds[0].InputFeePpk)
	}

	satSeeds, err := db.GetSeedsByUnit(tx, cashu.Sat)
	if err != nil {
		t.Fatalf("db.GetSeedsByUnit(cashu.Sat) %+v", err)
	}

	if satSeeds[1].Version != 2 {
		t.Error("Version should be 2")
	}
	if satSeeds[1].InputFeePpk != uint(100) {
		t.Errorf("Input fee should be 100. its %v", msatSeeds[0].InputFeePpk)
	}
	if len(satSeeds) != 2 {
		t.Errorf("Version should be 2 seeds. it's %v", len(keys.Keysets))
	}
}

func TestCreateNewSeed(t *testing.T) {
	db := mockdb.MockDB{}
	t.Setenv("MINT_PRIVATE_KEY", MintPrivateKey)
	localsigner, err := SetupLocalSigner(&db)
	if err != nil {
		t.Fatalf("SetupLocalSigner(&db) %+v", err)
	}

	keys, err := localsigner.GetActiveKeys()
	if err != nil {
		t.Fatalf("localsigner.GetActiveKeys() %+v", err)
	}

	// V2 keyset ID should start with "01" and be 66 characters long
	if len(keys.Keysets[0].Id) != 66 {
		t.Errorf("V2 keyset ID should be 66 characters long, got %d", len(keys.Keysets[0].Id))
	}
	if keys.Keysets[0].Id[:2] != "01" {
		t.Errorf("V2 keyset ID should start with '01', got %s", keys.Keysets[0].Id[:2])
	}
}
func TestRotateAuthSeedUnit(t *testing.T) {
	db := mockdb.MockDB{}
	t.Setenv("MINT_PRIVATE_KEY", MintPrivateKey)
	localsigner, err := SetupLocalSigner(&db)
	if err != nil {
		t.Fatalf("SetupLocalSigner(&db) %+v", err)
	}
	_, err = localsigner.getSignerPrivateKey()
	if err != nil {
		t.Fatalf("getSignerPrivateKey failed: %v", err)
	}

	err = localsigner.RotateKeyset(cashu.AUTH, uint(100), 240)
	if err != nil {
		t.Fatalf("localsigner.RotateKeyset(cashu.Msat, uint(100)) %+v", err)
	}

	keys, err := localsigner.GetAuthActiveKeys()
	if err != nil {
		t.Fatalf("localsigner.GetKeys() %+v", err)

	}
	if len(keys.Keysets) != 1 {

		t.Errorf("There should only be one keyset for auth. there is: %v", len(keys.Keysets))
	}

	if keys.Keysets[0].Unit != cashu.AUTH.String() {
		t.Errorf("Should be Auth key: it is %v", keys.Keysets[0].Unit)
	}

	_, ok := keys.Keysets[0].Keys[1]
	if !ok {
		t.Errorf("We should have a keysey of value 1. %+v", keys.Keysets[0])
	}
	// if keys.Keysets[1].Keys[] == 1 {
	//     t.Errorf("Should be Auth key %v",keys.Keysets[1].Unit)
	// }
}

func TestBackwardCompatibilityV1Keysets(t *testing.T) {
	// Pre-populate MockDB with a V1 keyset seed (simulating existing database with old keyset)
	v1Seed := cashu.Seed{
		Id:          "00bfa73302d12ffd", // Old V1 keyset ID
		Unit:        cashu.Sat.String(),
		Version:     1,
		InputFeePpk: 0,
		Active:      true,
	}
	db := mockdb.MockDB{
		Seeds: []cashu.Seed{v1Seed},
	}

	t.Setenv("MINT_PRIVATE_KEY", MintPrivateKey)
	localsigner, err := SetupLocalSigner(&db)
	if err != nil {
		t.Fatalf("SetupLocalSigner(&db) %+v", err)
	}

	// Verify V1 keyset is loaded correctly
	keys, err := localsigner.GetActiveKeys()
	if err != nil {
		t.Fatalf("localsigner.GetActiveKeys() %+v", err)
	}

	if len(keys.Keysets) != 1 {
		t.Errorf("Expected 1 keyset, got %d", len(keys.Keysets))
	}

	// Verify the V1 keyset ID is preserved
	if keys.Keysets[0].Id != "00bfa73302d12ffd" {
		t.Errorf("V1 keyset ID should be preserved, got %s", keys.Keysets[0].Id)
	}

	// Verify V1 keyset has correct format (starts with "00", 16 chars)
	if len(keys.Keysets[0].Id) != 16 {
		t.Errorf("V1 keyset ID should be 16 characters long, got %d", len(keys.Keysets[0].Id))
	}
	if keys.Keysets[0].Id[:2] != "00" {
		t.Errorf("V1 keyset ID should start with '00', got %s", keys.Keysets[0].Id[:2])
	}
}

func TestMixedV1AndV2Keysets(t *testing.T) {
	// Pre-populate MockDB with V1 seed (simulating existing keyset)
	v1Seed := cashu.Seed{
		Id:          "00bfa73302d12ffd",
		Unit:        cashu.Sat.String(),
		Version:     1,
		InputFeePpk: 0,
		Active:      true,
	}
	db := mockdb.MockDB{
		Seeds: []cashu.Seed{v1Seed},
	}

	t.Setenv("MINT_PRIVATE_KEY", MintPrivateKey)
	localsigner, err := SetupLocalSigner(&db)
	if err != nil {
		t.Fatalf("SetupLocalSigner(&db) %+v", err)
	}

	// Rotate the keyset - new one should use V2
	err = localsigner.RotateKeyset(cashu.Sat, uint(50), 240)
	if err != nil {
		t.Fatalf("localsigner.RotateKeyset(cashu.Sat, uint(50), 240) %+v", err)
	}

	// Get all keysets
	allKeysets, err := localsigner.GetKeysets()
	if err != nil {
		t.Fatalf("localsigner.GetKeysets() %+v", err)
	}

	if len(allKeysets.Keysets) != 2 {
		t.Errorf("Expected 2 keysets, got %d", len(allKeysets.Keysets))
	}

	// Find V1 and V2 keysets
	var v1Keyset, v2Keyset *cashu.BasicKeysetResponse
	for i := range allKeysets.Keysets {
		if allKeysets.Keysets[i].Id[:2] == "00" {
			v1Keyset = &allKeysets.Keysets[i]
		} else if allKeysets.Keysets[i].Id[:2] == "01" {
			v2Keyset = &allKeysets.Keysets[i]
		}
	}

	// Verify V1 keyset exists and is inactive
	if v1Keyset == nil {
		t.Error("V1 keyset should exist")
	} else {
		if v1Keyset.Active {
			t.Error("V1 keyset should be inactive after rotation")
		}
		if v1Keyset.Id != "00bfa73302d12ffd" {
			t.Errorf("V1 keyset ID should be preserved, got %s", v1Keyset.Id)
		}
	}

	// Verify V2 keyset exists and is active
	if v2Keyset == nil {
		t.Error("V2 keyset should exist after rotation")
	} else {
		if !v2Keyset.Active {
			t.Error("V2 keyset should be active after rotation")
		}
		if len(v2Keyset.Id) != 66 {
			t.Errorf("V2 keyset ID should be 66 characters long, got %d", len(v2Keyset.Id))
		}
	}

	// Verify active keys returns V2
	activeKeys, err := localsigner.GetActiveKeys()
	if err != nil {
		t.Fatalf("localsigner.GetActiveKeys() %+v", err)
	}

	if len(activeKeys.Keysets) != 1 {
		t.Errorf("Expected 1 active keyset, got %d", len(activeKeys.Keysets))
	}

	if activeKeys.Keysets[0].Id[:2] != "01" {
		t.Errorf("Active keyset should be V2 (starts with '01'), got %s", activeKeys.Keysets[0].Id[:2])
	}
}
