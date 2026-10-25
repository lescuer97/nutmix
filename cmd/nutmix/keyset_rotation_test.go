package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestSwapOutputKeysetAfterRotation(t *testing.T) {
	container, err := postgres.Run(t.Context(), "postgres:16.2",
		postgres.WithDatabase("postgres"),
		postgres.WithUsername("user"),
		postgres.WithPassword("password"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).WithStartupTimeout(60*time.Second)),
	)
	if container != nil {
		t.Cleanup(func() {
			if err := container.Terminate(context.Background()); err != nil {
				t.Errorf("terminate postgres: %v", err)
			}
		})
	}
	if err != nil {
		t.Fatal(err)
	}
	uri, err := container.ConnectionString(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DATABASE_URL", uri)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("MINT_PRIVATE_KEY", MintPrivateKey)
	t.Setenv("MINT_LIGHTNING_BACKEND", "FakeWallet")
	t.Setenv("NETWORK", "regtest")
	router, mint := SetupRoutingForTesting(t.Context(), false)
	keys, err := mint.Signer.GetActiveKeys()
	if err != nil {
		t.Fatal(err)
	}
	inputs, secrets, rs, err := CreateBlindedMessages(1, keys)
	if err != nil {
		t.Fatal(err)
	}
	signatures, _, err := mint.Signer.SignBlindMessages(inputs)
	if err != nil {
		t.Fatal(err)
	}
	proofs, err := GenerateProofs(signatures, keys, secrets, rs)
	if err != nil {
		t.Fatal(err)
	}
	outputs, _, _, err := CreateBlindedMessages(1, keys)
	if err != nil {
		t.Fatal(err)
	}
	oldID := keys.Keysets[0].Id
	if err := mint.Signer.RotateKeyset(cashu.Sat, 0, 0); err != nil {
		t.Fatal(err)
	}
	keys, err = mint.Signer.GetActiveKeys()
	if err != nil {
		t.Fatal(err)
	}
	if len(keys.Keysets) != 1 || keys.Keysets[0].Id == oldID {
		t.Fatal("rotation must produce a different active keyset")
	}
	for _, test := range []struct {
		name   string
		id     string
		status int
		code   int
	}{
		{name: "unknown", id: "00ababababababab", status: http.StatusBadRequest, code: 12001},
		{name: "inactive", id: oldID, status: http.StatusBadRequest, code: 12002},
		{name: "inactive input active output", id: keys.Keysets[0].Id, status: http.StatusOK, code: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			outputs[0].Id = test.id
			body, err := json.Marshal(cashu.PostSwapRequest{Inputs: proofs, Outputs: outputs})
			if err != nil {
				t.Fatal(err)
			}
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/swap", bytes.NewReader(body)))
			if recorder.Code != test.status {
				t.Fatalf("status = %d, want %d", recorder.Code, test.status)
			}
			if test.code != 0 {
				var response struct {
					Code int `json:"code"`
				}
				if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
					t.Fatal(err)
				}
				if response.Code != test.code {
					t.Fatalf("code = %d, want %d", response.Code, test.code)
				}
			} else {
				var response cashu.PostSwapResponse
				if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
					t.Fatal(err)
				}
				if len(response.Signatures) != 1 || response.Signatures[0].Id != test.id || response.Signatures[0].C_.PublicKey == nil {
					t.Fatal("expected a signature under the new active keyset")
				}
			}
		})
	}
}
