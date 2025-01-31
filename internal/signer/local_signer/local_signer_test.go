package localsigner

// import (
// 	"encoding/hex"
// 	"github.com/decred/dcrd/dcrec/secp256k1/v4"
// 	"github.com/lescuer97/nutmix/api/cashu"
// 	"testing"
// )

const MintPrivateKey string = "0000000000000000000000000000000000000000000000000000000000000001"


func TestMint
// func TestCreateNewSeed(t *testing.T) {
// 	decodedPrivKey, err := hex.DecodeString(MintPrivateKey)
// 	if err != nil {
// 		t.Fatal("hex.DecodeString(mint_privkey)")
// 	}
//
// 	parsedPrivateKey := secp256k1.PrivKeyFromBytes(decodedPrivKey)
// 	masterKey, err := MintPrivateKeyToBip32(parsedPrivateKey)
// 	if err != nil {
// 		t.Fatal("mint.MintPrivateKeyToBip32(parsedPrivateKey)")
// 	}
//
// 	seed1, err := CreateNewSeed(masterKey, 1, 0)
// 	if err != nil {
// 		t.Fatal("CreateNewSeed(masterKey, 1, 0)")
// 	}
//
// 	if seed1.Id != "00bfa73302d12ffd" {
// 		t.Errorf("seed id incorrect. %v", seed1.Id)
//
// 	}
//
// }
// func TestGeneratedKeysetsMakeTheCorrectIds(t *testing.T) {
// 	decodedPrivKey, err := hex.DecodeString(MintPrivateKey)
// 	if err != nil {
// 		t.Fatal("hex.DecodeString(mint_privkey)")
// 	}
//
// 	parsedPrivateKey := secp256k1.PrivKeyFromBytes(decodedPrivKey)
// 	masterKey, err := MintPrivateKeyToBip32(parsedPrivateKey)
// 	if err != nil {
// 		t.Fatal("mint.MintPrivateKeyToBip32(parsedPrivateKey)")
// 	}
// 	seed1, err := CreateNewSeed(masterKey, 1, 0)
// 	if err != nil {
// 		t.Fatal("CreateNewSeed(masterKey, 1, 0)")
// 	}
//
// 	keyset, err := DeriveKeyset(masterKey, seed1)
// 	if err != nil {
// 		t.Fatal("DeriveKeyset(masterKey,seed1 )")
// 	}
// 	newSeedId, err := cashu.DeriveKeysetId(keyset)
// 	if err != nil {
// 		t.Fatal("cashu.DeriveKeysetId(keyset)")
// 	}
//
// 	if newSeedId != "00bfa73302d12ffd" {
// 		t.Errorf("seed id incorrect. %v", seed1.Id)
//
// 	}
//
// }
