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

	if keys.Keysets[0].Id != "00bfa73302d12ffd" {
		t.Errorf("seed id incorrect. %v", keys.Keysets[1].Id)
	}
}
func TestRotateAuthSeedUnit(t *testing.T) {
	db := mockdb.MockDB{}
	t.Setenv("MINT_PRIVATE_KEY", MintPrivateKey)
	localsigner, err := SetupLocalSigner(&db)
	if err != nil {
		t.Fatalf("SetupLocalSigner(&db) %+v", err)
	}
	localsigner.getSignerPrivateKey()

	err = localsigner.RotateKeyset(cashu.AUTH, uint(100))
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

	_, ok := keys.Keysets[0].Keys["1"]
	if !ok {
		t.Errorf("We should have a keysey of value 1. %+v", keys.Keysets[0])
	}
	// if keys.Keysets[1].Keys[] == 1 {
	//     t.Errorf("Should be Auth key %v",keys.Keysets[1].Unit)
	// }
}
