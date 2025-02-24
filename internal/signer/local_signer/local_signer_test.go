package localsigner

import (
	"github.com/lescuer97/nutmix/api/cashu"
	mockdb "github.com/lescuer97/nutmix/internal/database/mock_db"
	"testing"
)

const MintPrivateKey string = "0000000000000000000000000000000000000000000000000000000000000001"

func TestRotateUnexistingSeedUnit(t *testing.T) {
	db := mockdb.MockDB{}
	t.Setenv("MINT_PRIVATE_KEY", MintPrivateKey)
	localsigner, err := SetupLocalSigner(&db)
	if err != nil {
		t.Fatalf("SetupLocalSigner(&db) %+v", err)
	}
	localsigner.getSignerPrivateKey()

	err = localsigner.RotateKeyset(cashu.Msat, uint(100))
	if err != nil {
		t.Fatalf("localsigner.RotateKeyset(cashu.Msat, uint(100)) %+v", err)
	}

	err = localsigner.RotateKeyset(cashu.Sat, uint(100))
	if err != nil {
		t.Fatalf("localsigner.RotateKeyset(cashu.Sat, uint(100)) %+v", err)
	}

	keys, err := localsigner.GetKeys()
	if err != nil {
		t.Fatalf("localsigner.GetKeys() %+v", err)

	}
	if len(keys.Keysets) != 3 {
		t.Errorf("Version should be 3. it's %v", len(keys.Keysets))
	}

	msatSeeds, err := db.GetSeedsByUnit(cashu.Msat)
	if err != nil {
		t.Fatalf("db.GetSeedsByUnit(cashu.Msat) %+v", err)
	}

	if msatSeeds[0].Version != 1 {
		t.Error("Version should be 1")
	}
	if msatSeeds[0].InputFeePpk != uint(100) {
		t.Errorf("Input fee should be 100. its %v", msatSeeds[0].InputFeePpk)
	}

	satSeeds, err := db.GetSeedsByUnit(cashu.Sat)
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

	if keys.Keysets[0].Id != "00bfa73302d12ffd" {
		t.Errorf("seed id incorrect. %v", keys.Keysets[1].Id)
	}
}
