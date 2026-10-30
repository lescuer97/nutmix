package cashu

import (
	"encoding/base64"
	"errors"
	"testing"
)

func TestDecodeAuthTokenShortInputs(t *testing.T) {
	for _, token := range []string{"", "a", "au", "aut", "auth", "é", "éé"} {
		t.Run(token, func(t *testing.T) {
			_, err := DecodeAuthToken(token)
			if !errors.Is(err, ErrInvalidAuthToken) {
				t.Fatalf("DecodeAuthToken(%q) error = %v, want %v", token, err, ErrInvalidAuthToken)
			}
		})
	}
}

func TestDecodeAuthTokenInvalidPrefix(t *testing.T) {
	_, err := DecodeAuthToken("nope!")
	if !errors.Is(err, ErrInvalidAuthToken) {
		t.Fatalf("DecodeAuthToken error = %v, want %v", err, ErrInvalidAuthToken)
	}
}

func TestDecodeAuthTokenValid(t *testing.T) {
	const payload = `{"id":"fixture","secret":"secret","amount":1}`
	token := "authA" + base64.URLEncoding.EncodeToString([]byte(payload))
	proof, err := DecodeAuthToken(token)
	if err != nil {
		t.Fatalf("DecodeAuthToken error = %v", err)
	}
	if proof.Id != "fixture" || proof.Secret != "secret" || proof.Amount != 1 {
		t.Fatalf("DecodeAuthToken proof = %#v", proof)
	}
}
