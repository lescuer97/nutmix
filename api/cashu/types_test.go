package cashu

import (
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lescuer97/nutmix/pkg/crypto"
	"github.com/tyler-smith/go-bip32"
)

func TestGenerateBlindSignatureAndCheckSignature(t *testing.T) {
	key, err := bip32.NewMasterKey([]byte("seed"))
	if err != nil {
		t.Errorf("could not setup master key %+v", err)
	}

	// Key for mint
	generatedKeysets, err := GenerateKeysets(key, GetAmountsForKeysets(), "id", Sat, 0, true)

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

	blindMessage := BlindedMessage{
		Amount: 1,
		B_:     publicKeyBlindFactor,
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

func TestCashuAmountChangeSatToMsat(t *testing.T) {
	amount := Amount{
		Amount: 1000,
		Unit:   Sat,
	}
	err := amount.To(Msat)

	if err != nil {
		t.Fatalf("amount.To(Msat). %v", err)
	}
	if amount.Amount != 1000000 {
		t.Errorf("Amount is not correct")
	}
	if amount.Unit != Msat {
		t.Errorf("unit is not correct")
	}
}

func TestCashuAmountChangeMsatToSat(t *testing.T) {
	amount := Amount{
		Amount: 1000,
		Unit:   Msat,
	}
	err := amount.To(Sat)

	if err != nil {
		t.Fatalf("amount.To(Sat). %v", err)
	}
	if amount.Amount != 1 {
		t.Errorf("Amount is not correct")
	}
	if amount.Unit != Sat {
		t.Errorf("unit is not correct")
	}
}
func TestCashuAmountChangeMsatToSatMinimum(t *testing.T) {
	amount := Amount{
		Amount: 1,
		Unit:   Msat,
	}
	err := amount.To(Sat)

	if err != nil {
		t.Fatalf("amount.To(Sat). %v", err)
	}

	if amount.Amount != 0 {
		t.Errorf("Amount is not correct")
	}
	if amount.Unit != Sat {
		t.Errorf("unit is not correct")
	}
}

func TestCashuAmountUSDtoCents(t *testing.T) {
	amount := Amount{
		Amount: 10,
		Unit:   USD,
	}
	str, err := amount.CentsToUSD()

	if err != nil {
		t.Fatalf("amount.SatToBTC(). %v", err)
	}
	if str != "0.10" {
		t.Errorf("Amount is not correct")
	}
	if amount.Unit != USD {
		t.Errorf("unit is not correct")
	}
}
func TestCashuAmountEURtoCents(t *testing.T) {
	amount := Amount{
		Amount: 10000,
		Unit:   EUR,
	}
	str, err := amount.CentsToUSD()

	if err != nil {
		t.Fatalf("amount.SatToBTC(). %v", err)
	}
	if str != "100.00" {
		t.Errorf("Amount is not correct")
	}
	if amount.Unit != EUR {
		t.Errorf("unit is not correct")
	}
}
func TestCashuAmountConvertError(t *testing.T) {
	amount := Amount{
		Amount: 10000,
		Unit:   Sat,
	}
	err := amount.To(EUR)

	if err != ErrCouldNotConvertUnit {
		t.Errorf("err != ErrCouldNotConvertUnit. %v", err)
	}
}

func TestCashuAmountConvertUSDStrError(t *testing.T) {
	amount := Amount{
		Amount: 10000,
		Unit:   Sat,
	}
	_, err := amount.CentsToUSD()

	if err != ErrCouldNotParseAmountToString {
		t.Errorf("err != ErrCouldNotParseAmountToString. %v", err)
	}
}

func TestCashuAmountConvertEURStrError(t *testing.T) {
	amount := Amount{
		Amount: 10000,
		Unit:   EUR,
	}
	_, err := amount.SatToBTC()

	if err != ErrCouldNotParseAmountToString {
		t.Errorf("err != ErrCouldNotParseAmountToString. %v", err)
	}
}

func TestBlindedMessageUnmarshalJSON(t *testing.T) {
	// Example valid hex-encoded public key (compressed format)
	validPubKeyHex := "0342e5bcc77f5b2a3c2afb40bb591a1e27da83cddc968abdc0ec4904201a201834"

	// JSON input with a valid B_ field
	jsonInput := `{
		"amount": 100,
		"keyset_id": "example-keyset-id",
		"B_": "` + validPubKeyHex + `"
	}`

	// Unmarshal the JSON into a BlindedMessage
	var msg BlindedMessage
	err := json.Unmarshal([]byte(jsonInput), &msg)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Verify the parsed public key
	pubkeyBytes, err := hex.DecodeString(validPubKeyHex)
	if err != nil {
		t.Fatalf("could not decode hex string. %v", err)
	}
	expectedPubKey, err := secp256k1.ParsePubKey(pubkeyBytes)
	if err != nil {
		t.Fatalf("Failed to parse expected public key: %v", err)
	}

	if !msg.B_.IsEqual(expectedPubKey) {
		t.Errorf("BlindedSecret does not match expected public key")
	}

	if hex.EncodeToString(msg.B_.SerializeCompressed()) != validPubKeyHex {
		t.Errorf("SerializeCompressed doesn't do what I thought")
	}

	// Test invalid JSON input
	invalidJsonInput := `{
		"amount": 100,
		"keyset_id": "example-keyset-id",
		"B_": "invalid-hex-string"
	}`

	err = json.Unmarshal([]byte(invalidJsonInput), &msg)
	if err == nil {
		t.Errorf("Expected error for invalid hex string, but got none")
	}
}

// TEST VECTORS NUT 20 - Signature on Mint Quote
// https://github.com/cashubtc/nuts/blob/main/20.md#

func TestNut20SuccessfulSignature(t *testing.T) {
	jsonStr := `{
  "quote": "9d745270-1405-46de-b5c5-e2762b4f5e00",
  "outputs": [
    {
      "amount": 1,
      "id": "00456a94ab4e1c46",
      "B_": "0342e5bcc77f5b2a3c2afb40bb591a1e27da83cddc968abdc0ec4904201a201834"
    },
    {
      "amount": 1,
      "id": "00456a94ab4e1c46",
      "B_": "032fd3c4dc49a2844a89998d5e9d5b0f0b00dde9310063acb8a92e2fdafa4126d4"
    },
    {
      "amount": 1,
      "id": "00456a94ab4e1c46",
      "B_": "033b6fde50b6a0dfe61ad148fff167ad9cf8308ded5f6f6b2fe000a036c464c311"
    },
    {
      "amount": 1,
      "id": "00456a94ab4e1c46",
      "B_": "02be5a55f03e5c0aaea77595d574bce92c6d57a2a0fb2b5955c0b87e4520e06b53"
    },
    {
      "amount": 1,
      "id": "00456a94ab4e1c46",
      "B_": "02209fc2873f28521cbdde7f7b3bb1521002463f5979686fd156f23fe6a8aa2b79"
    }
  ],
  "signature": "d4b386f21f7aa7172f0994ee6e4dd966539484247ea71c99b81b8e09b1bb2acbc0026a43c221fd773471dc30d6a32b04692e6837ddaccf0830a63128308e4ee0"
}`
	bytes := []byte(jsonStr)

	pubkeyStr := "03d56ce4e446a85bbdaa547b4ec2b073d40ff802831352b8272b7dd7a4de5a7cac"
	pubkeyBytes, err := hex.DecodeString(pubkeyStr)
	if err != nil {
		t.Fatalf("could not decode hex string. %v", err)
	}

	pubkey, err := secp256k1.ParsePubKey(pubkeyBytes)
	if err != nil {
		t.Fatalf("could not parse pubkey bytes correctly. %v", err)
	}

	var request PostMintBolt11Request
	err = json.Unmarshal(bytes, &request)
	if err != nil {
		t.Fatalf("could not marshal to correct PostMintBolt11Request struct. %v", err)
	}

	if request.Quote != "9d745270-1405-46de-b5c5-e2762b4f5e00" {
		t.Errorf("quote not parsed correctly")
	}
	if hex.EncodeToString(request.Outputs[0].B_.SerializeCompressed()) != "0342e5bcc77f5b2a3c2afb40bb591a1e27da83cddc968abdc0ec4904201a201834" {
		t.Errorf("First output not parsed correctly")
	}
	if hex.EncodeToString(request.Outputs[len(request.Outputs)-1].B_.SerializeCompressed()) != "02209fc2873f28521cbdde7f7b3bb1521002463f5979686fd156f23fe6a8aa2b79" {
		t.Errorf("last output not parsed correctly")
	}

	valid, err := request.VerifyPubkey(pubkey)
	if err != nil {
		t.Fatalf("Something happened while verifying. %v", err)
	}
	if !valid {
		t.Error("signature should be valid")
	}

}
func TestNut20FailureSignature(t *testing.T) {
	jsonStr := `{
  "quote": "9d745270-1405-46de-b5c5-e2762b4f5e00",
  "outputs": [
    {
      "amount": 1,
      "id": "00456a94ab4e1c46",
      "B_": "0342e5bcc77f5b2a3c2afb40bb591a1e27da83cddc968abdc0ec4904201a201834"
    },
    {
      "amount": 1,
      "id": "00456a94ab4e1c46",
      "B_": "032fd3c4dc49a2844a89998d5e9d5b0f0b00dde9310063acb8a92e2fdafa4126d4"
    },
    {
      "amount": 1,
      "id": "00456a94ab4e1c46",
      "B_": "033b6fde50b6a0dfe61ad148fff167ad9cf8308ded5f6f6b2fe000a036c464c311"
    },
    {
      "amount": 1,
      "id": "00456a94ab4e1c46",
      "B_": "02be5a55f03e5c0aaea77595d574bce92c6d57a2a0fb2b5955c0b87e4520e06b53"
    },
    {
      "amount": 1,
      "id": "00456a94ab4e1c46",
      "B_": "02209fc2873f28521cbdde7f7b3bb1521002463f5979686fd156f23fe6a8aa2b79"
    }
  ],
  "signature": "cb2b8e7ea69362dfe2a07093f2bbc319226db33db2ef686c940b5ec976bcbfc78df0cd35b3e998adf437b09ee2c950bd66dfe9eb64abd706e43ebc7c669c36c3"
}`
	bytes := []byte(jsonStr)

	pubkeyStr := "03d56ce4e446a85bbdaa547b4ec2b073d40ff802831352b8272b7dd7a4de5a7cac"
	pubkeyBytes, err := hex.DecodeString(pubkeyStr)
	if err != nil {
		t.Fatalf("could not decode hex string. %v", err)
	}

	pubkey, err := secp256k1.ParsePubKey(pubkeyBytes)
	if err != nil {
		t.Fatalf("could not parse pubkey bytes correctly. %v", err)
	}

	var request PostMintBolt11Request
	err = json.Unmarshal(bytes, &request)
	if err != nil {
		t.Fatalf("could not marshal to correct PostMintBolt11Request struct. %v", err)
	}

	if request.Quote != "9d745270-1405-46de-b5c5-e2762b4f5e00" {
		t.Errorf("quote not parsed correctly")
	}
	if hex.EncodeToString(request.Outputs[0].B_.SerializeCompressed()) != "0342e5bcc77f5b2a3c2afb40bb591a1e27da83cddc968abdc0ec4904201a201834" {
		t.Errorf("First output not parsed correctly")
	}
	if hex.EncodeToString(request.Outputs[len(request.Outputs)-1].B_.SerializeCompressed()) != "02209fc2873f28521cbdde7f7b3bb1521002463f5979686fd156f23fe6a8aa2b79" {
		t.Errorf("last output not parsed correctly")
	}

	valid, err := request.VerifyPubkey(pubkey)
	if err != nil {
		t.Fatalf("Something happened while verifying. %v", err)
	}
	if valid {
		t.Error("signature should be valid")
	}

}
