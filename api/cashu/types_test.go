package cashu

import (
	"encoding/hex"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lescuer97/nutmix/pkg/crypto"
	"github.com/tyler-smith/go-bip32"
	"testing"
)

func TestGenerateBlindSignatureAndCheckSignature(t *testing.T) {
	key, err := bip32.NewMasterKey([]byte("seed"))
	if err != nil {
		t.Errorf("could not setup master key %+v", err)
	}

	// Key for mint
	generatedKeysets, err := GenerateKeysets(key, GetAmountsForKeysets(), "id", Sat)

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

	keysetId, err := DeriveKeysetId(generatedKeysets)

	if err != nil {
		t.Errorf("could not derive keyset id %+v", err)
	}

	blindingFactor := hex.EncodeToString((publicKeyBlindFactor.SerializeCompressed()))

	blindMessage := BlindedMessage{
		Amount: 1,
		B_:     blindingFactor,
		Id:     keysetId,
	}

	// Create BlindSignature

	blindSignature, err := blindMessage.GenerateBlindSignature(generatedKeysets[1].PrivKey)
	if err != nil {
		t.Errorf("could GenerateBlindSignature %+v", err)
	}

	if blindSignature.C_ != "024b60ff45c5a4ef4630072a03eaabcb948beae56d034a3bba68dc8cda68845c5d" {
		t.Errorf("blindSignature is not correct")
	}

	if blindSignature.Id != "0014d74f728e80b8" {
		t.Errorf("blindSignature id is not correct")
	}

	bytesC_, err := hex.DecodeString(blindSignature.C_)
	if err != nil {
		t.Errorf("could not decode hex %+v", err)
	}

	pubkeyC_, err := secp256k1.ParsePubKey(bytesC_)
	if err != nil {
		t.Errorf("could not secp256k1.ParsePubKey %+v", err)
	}

	unblindedFactor := crypto.UnblindSignature(pubkeyC_, privateKeyBlindFactor, generatedKeysets[1].PrivKey.PubKey())

	proof := Proof{
		Amount: 1,
		C:      hex.EncodeToString(unblindedFactor.SerializeCompressed()),
		Secret: "secret",
		Id:     keysetId,
		Y:      "",
	}

	proof, err = proof.HashSecretToCurve()
	if err != nil {
		t.Errorf("could not proof.HashSecretToCurve %+v", err)
	}

	if proof.Y != "025dccd27047d10d4900b8d2c4ea6795702c2d1fbe1d3fd0d1cd4b18776b12ddc0" {
		t.Errorf("proof.Y is not correct")
	}

}

func TestGenerateDLEQ(t *testing.T) {
	a_bytes, err := hex.DecodeString("0000000000000000000000000000000000000000000000000000000000000001")
	if err != nil {
		t.Errorf("error decoding R1: %v", err)
	}

	a := secp256k1.PrivKeyFromBytes(a_bytes)

	b_bytes, err := hex.DecodeString("02a9acc1e48c25eeeb9289b5031cc57da9fe72f3fe2861d264bdc074209b107ba2")
	if err != nil {
		t.Errorf("error decoding b_: %v", err)
	}

	B_, err := secp256k1.ParsePubKey(b_bytes)

	if err != nil {
		t.Errorf("secp256k1.ParsePubKey: %v", err)
	}

	C_ := "02a9acc1e48c25eeeb9289b5031cc57da9fe72f3fe2861d264bdc074209b107ba2"

	blindSignature := BlindSignature{
		C_: C_,
	}

	err = blindSignature.GenerateDLEQ(B_, a)
	if err != nil {
		t.Errorf("could not GenerateDLEQ %+v", err)
	}

	verify, err := blindSignature.VerifyDLEQ(B_, blindSignature.Dleq.E, blindSignature.Dleq.S, a.PubKey())
	if err != nil {
		t.Errorf("could not VerifyDLEQ %+v", err)
	}

	if !verify {
		t.Errorf("DLEQ is not correct")
	}

}
