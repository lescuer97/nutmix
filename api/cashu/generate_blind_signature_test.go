package cashu_test

import (
	"encoding/hex"
	"testing"

	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lescuer97/nutmix/api/cashu"
	localsigner "github.com/lescuer97/nutmix/internal/signer/local_signer"
	"github.com/lescuer97/nutmix/pkg/crypto"
	"github.com/tyler-smith/go-bip32"
)

func TestGenerateBlindSignatureAndCheckSignature(t *testing.T) {
	keyBytes, err := hex.DecodeString("0000000000000000000000000000000000000000000000000000000000000001")
	if err != nil {
		t.Errorf("could not decode key %+v", err)
	}
	key, err := hdkeychain.NewMaster(keyBytes, &chaincfg.MainNetParams)
	if err != nil {
		t.Errorf("could not setup master key %+v", err)
	}

	seed := cashu.Seed{
		Id:          "id",
		Unit:        cashu.Sat.String(),
		Version:     0,
		InputFeePpk: 0,
		Amounts:     cashu.GetAmountsForKeysets(cashu.LegacyMaxKeysetAmount),
		Legacy:      true,
	}

	generatedKeysets, err := localsigner.GenerateKeysets(key, seed)
	if err != nil {
		t.Errorf("could not generate keyset %+v", err)
	}

	walletKey, err := bip32.NewMasterKey([]byte("walletseed"))
	if err != nil {
		t.Errorf("could not setup wallet key %+v", err)
	}

	parsedKey := secp256k1.PrivKeyFromBytes(walletKey.Key)

	// Create BlindedMessage
	publicKeyBlindFactor, privateKeyBlindFactor, err := crypto.BlindMessage("secret", parsedKey)
	if err != nil {
		t.Errorf("could not create blindmessage %+v", err)
	}

	justPubkeys := []*secp256k1.PublicKey{}
	for i := range generatedKeysets {
		justPubkeys = append(justPubkeys, generatedKeysets[i].GetPubKey())
	}

	keysetId, err := localsigner.DeriveKeysetId(justPubkeys)
	if err != nil {
		t.Errorf("could not derive keyset id %+v", err)
	}

	blindMessage := cashu.BlindedMessage{
		Amount: 1,
		B_:     cashu.WrappedPublicKey{PublicKey: publicKeyBlindFactor},
		Id:     keysetId,
	}

	// Create BlindSignature
	blindSignature, err := blindMessage.GenerateBlindSignature(generatedKeysets[1].PrivKey)
	if err != nil {
		t.Errorf("could GenerateBlindSignature %+v", err)
	}

	if blindSignature.C_.String() != "027184da18bdc8c225093c299062fe0a3658122db41e6ef72258b83df52709a6b6" {
		t.Errorf("blindSignature is not correct. %v", blindSignature.C_.String())
	}

	if blindSignature.Id != "000fc082ba6bd376" {
		t.Errorf("blindSignature id is not correct. %v", blindSignature.Id)
	}

	unblindedFactor := crypto.UnblindSignature(blindSignature.C_.PublicKey, privateKeyBlindFactor, generatedKeysets[1].PrivKey.PubKey())

	proof := cashu.Proof{
		Amount: 1,
		C:      cashu.WrappedPublicKey{PublicKey: unblindedFactor},
		Secret: "secret",
		Id:     keysetId,
	}

	proof, err = proof.HashSecretToCurve()
	if err != nil {
		t.Errorf("could not proof.HashSecretToCurve %+v", err)
	}

	if proof.Y.ToHex() != "025dccd27047d10d4900b8d2c4ea6795702c2d1fbe1d3fd0d1cd4b18776b12ddc0" {
		t.Errorf("proof.Y is not correct")
	}
}
