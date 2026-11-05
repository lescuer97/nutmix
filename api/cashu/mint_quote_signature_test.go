package cashu

import (
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

// The PostMintBolt11Request test vector from cashubtc/nuts tests/20-test.md.
const nut20VectorRequest = `{
  "quote": "0192d3c0-7e8a-7c3d-8e9f-1a2b3c4d5e6f",
  "outputs": [
    {"amount": 1, "id": "009a1f293253e41e", "B_": "036d6caac248af96f6afa7f904f550253a0f3ef3f5aa2fe6838a95b216691468e2"},
    {"amount": 1, "id": "009a1f293253e41e", "B_": "021f8a566c205633d029094747d2e18f44e05993dda7a5f88f496078205f656e59"}
  ],
  "signature": "4881093a332ff7c79f3e598ce5b249d64978b47165a0b19c18adf0ced0246228e61e702f0abaf1bf27b92be4336bdbabacfbe4c914076386b3c66fdcd0b3480e"
}`

func nut20Key(t *testing.T) *btcec.PrivateKey {
	t.Helper()
	keyBytes := make([]byte, 32)
	keyBytes[31] = 1
	key, _ := btcec.PrivKeyFromBytes(keyBytes)
	return key
}

func TestMintQuoteSignatureV1Vector(t *testing.T) {
	var request PostMintBolt11Request
	if err := json.Unmarshal([]byte(nut20VectorRequest), &request); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	hash := request.mintQuoteMessageHash()
	if got := hex.EncodeToString(hash[:]); got != "c164fd384879f74ab6ea2e7cf13d90ed42e6df9d5de607eeb5c9cc7d36fb1c21" {
		t.Fatalf("msg_to_sign hash = %s", got)
	}

	valid, err := request.VerifyPubkey(nut20Key(t).PubKey())
	if err != nil || !valid {
		t.Fatalf("VerifyPubkey = %v, %v; want true, nil", valid, err)
	}

	other := nut20Key(t)
	otherBytes := other.Serialize()
	otherBytes[31] = 2
	otherKey, _ := btcec.PrivKeyFromBytes(otherBytes)
	if valid, _ := request.VerifyPubkey(otherKey.PubKey()); valid {
		t.Fatal("signature verified under the wrong key")
	}

	request.Outputs[1].Amount = 2
	if valid, _ := request.VerifyPubkey(nut20Key(t).PubKey()); valid {
		t.Fatal("signature verified after an output amount changed")
	}
}

func TestMintQuoteSignatureLegacyStillAccepted(t *testing.T) {
	var request PostMintBolt11Request
	if err := json.Unmarshal([]byte(nut20VectorRequest), &request); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	legacy := request.legacyMintQuoteMessageHash()
	sig, err := schnorr.Sign(nut20Key(t), legacy[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	request.Signature = sig

	valid, err := request.VerifyPubkey(nut20Key(t).PubKey())
	if err != nil || !valid {
		t.Fatalf("VerifyPubkey(legacy) = %v, %v; want true, nil", valid, err)
	}
}
