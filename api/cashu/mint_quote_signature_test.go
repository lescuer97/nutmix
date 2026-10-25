package cashu

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
)

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
