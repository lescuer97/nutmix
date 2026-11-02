package cashu

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
)

//go:embed testdata/nut20/mint_quote.json
var nut20MintQuoteVector embed.FS

func TestMintQuoteSignatureMessage(t *testing.T) {
	pubkey := secp256k1.PrivKeyFromBytes([]byte{1}).PubKey()
	outputs := BlindedMessages{
		{Amount: 0, B_: WrappedPublicKey{PublicKey: pubkey}},   //nolint:exhaustruct // Id and Witness are irrelevant to the signed message.
		{Amount: 256, B_: WrappedPublicKey{PublicKey: pubkey}}, //nolint:exhaustruct // Id and Witness are irrelevant to the signed message.
	}
	message, err := MintQuoteSignatureMessage("q", outputs)
	if err != nil {
		t.Fatal(err)
	}
	want := "43617368755f4d696e7451756f74655369675f7631" +
		"0000000171" +
		"00000000" + "00000021" + "0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798" +
		"000000020100" + "00000021" + "0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"
	if got := hex.EncodeToString(message); got != want {
		t.Fatalf("message mismatch\n got: %s\nwant: %s", got, want)
	}
}

func TestVerifyPubkeyBindsQuoteOutputsAndOrder(t *testing.T) {
	key, _ := btcec.PrivKeyFromBytes([]byte{1})
	point := secp256k1.PrivKeyFromBytes([]byte{2}).PubKey()
	request := PostMintBolt11Request{Quote: "quote", Outputs: BlindedMessages{ //nolint:exhaustruct // Signature is populated after computing the signed message.
		{Amount: 1, B_: WrappedPublicKey{PublicKey: point}},          //nolint:exhaustruct // Id and Witness do not affect signature verification.
		{Amount: 256, B_: WrappedPublicKey{PublicKey: key.PubKey()}}, //nolint:exhaustruct // Id and Witness do not affect signature verification.
	}}
	message, err := MintQuoteSignatureMessage(request.Quote, request.Outputs)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(message)
	request.Signature, err = schnorr.Sign(key, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	valid, err := request.VerifyPubkey(key.PubKey())
	if err != nil || !valid {
		t.Fatalf("valid signature rejected: valid=%v err=%v", valid, err)
	}

	variants := []struct {
		mutate func(*PostMintBolt11Request)
		name   string
	}{
		{mutate: func(r *PostMintBolt11Request) { r.Quote = "other" }, name: "quote"},
		{mutate: func(r *PostMintBolt11Request) { r.Outputs[0].Amount++ }, name: "amount"},
		{mutate: func(r *PostMintBolt11Request) { r.Outputs[0].B_ = WrappedPublicKey{PublicKey: key.PubKey()} }, name: "point"},
		{mutate: func(r *PostMintBolt11Request) { r.Outputs[0], r.Outputs[1] = r.Outputs[1], r.Outputs[0] }, name: "order"},
	}
	for _, variant := range variants {
		t.Run(variant.name, func(t *testing.T) {
			changed := request
			changed.Outputs = append(BlindedMessages(nil), request.Outputs...)
			variant.mutate(&changed)
			valid, err := changed.VerifyPubkey(key.PubKey())
			if err != nil {
				t.Fatal(err)
			}
			if valid {
				t.Fatal("signature accepted after signed data changed")
			}
		})
	}
}

func TestMintQuoteSignatureMessageRejectsNilBlindedPoint(t *testing.T) {
	if _, err := MintQuoteSignatureMessage("quote", BlindedMessages{{Amount: 1}}); err != ErrInvalidBlindMessage { //nolint:exhaustruct // Missing point is the invalid input under test.
		t.Fatalf("expected ErrInvalidBlindMessage, got %v", err)
	}
}

// TestNut20PublishedMintQuoteVector is the NUT-20 signature vector from
// cashubtc/nuts@1363de413294e2ef3a57b16950f9506af38fffc2/tests/20-test.md.
// Keep the published signature and digest intact so this tests
// interoperability rather than self-signing.
func TestNut20PublishedMintQuoteVector(t *testing.T) {
	data, err := nut20MintQuoteVector.ReadFile("testdata/nut20/mint_quote.json")
	if err != nil {
		t.Fatal(err)
	}
	var vector struct {
		ExpectedSHA256 string                `json:"expected_sha256"`
		Pubkey         string                `json:"pubkey"`
		Request        PostMintBolt11Request `json:"request"`
	}
	if err := json.Unmarshal(data, &vector); err != nil {
		t.Fatal(err)
	}
	pubkeyBytes, err := hex.DecodeString(vector.Pubkey)
	if err != nil {
		t.Fatal(err)
	}
	pubkey, err := secp256k1.ParsePubKey(pubkeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	message, err := MintQuoteSignatureMessage(vector.Request.Quote, vector.Request.Outputs)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(message)
	if got := hex.EncodeToString(digest[:]); got != vector.ExpectedSHA256 {
		t.Fatalf("published NUT-20 digest mismatch: got %s, want %s", got, vector.ExpectedSHA256)
	}
	valid, err := vector.Request.VerifyPubkey(pubkey)
	if err != nil || !valid {
		t.Fatalf("published NUT-20 signature rejected: valid=%v err=%v", valid, err)
	}
}
