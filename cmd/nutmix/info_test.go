package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/utils"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestMintInfo(t *testing.T) {
	const testVersion = "test-version"
	// Mock the version
	utils.AppVersion = testVersion

	const posgrespassword = "password"
	const postgresuser = "user"
	ctx := context.Background()

	postgresContainer, err := postgres.Run(ctx, "postgres:16.2",
		postgres.WithDatabase("postgres"),
		postgres.WithUsername(postgresuser),
		postgres.WithPassword(posgrespassword),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second)),
	)
	if err != nil {
		t.Fatal(err)
	}

	connUri, err := postgresContainer.ConnectionString(ctx)
	if err != nil {
		t.Fatal(fmt.Errorf("failed to get connection string: %w", err))
	}

	t.Setenv("DATABASE_URL", connUri)
	t.Setenv("MINT_PRIVATE_KEY", MintPrivateKey)
	t.Setenv("MINT_LIGHTNING_BACKEND", "FakeWallet")
	t.Setenv(mint.NETWORK_ENV, "regtest")

	ctx = context.WithValue(ctx, ctxKeyNetwork, os.Getenv(mint.NETWORK_ENV))
	ctx = context.WithValue(ctx, ctxKeyLightningBackend, os.Getenv(mint.MINT_LIGHTNING_BACKEND_ENV))
	ctx = context.WithValue(ctx, ctxKeyDatabaseURL, os.Getenv(database.DATABASE_URL_ENV))
	ctx = context.WithValue(ctx, ctxKeyNetwork, os.Getenv(mint.NETWORK_ENV))

	router, _ := SetupRoutingForTesting(ctx, false)

	req := httptest.NewRequest("GET", "/v1/info", nil)

	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("Expected status code 200, got %d", w.Code)
	}

	var mintInfo cashu.GetInfoResponse
	err = json.Unmarshal(w.Body.Bytes(), &mintInfo)
	if err != nil {
		t.Errorf("Error unmarshalling response: %v", err)
	}

	if mintInfo.Version != "nutmix/"+testVersion {
		t.Errorf("Incorrect version  %v", mintInfo.Version)
	}

	if mintInfo.Pubkey != "032332a0f0ca8e1c7ff9a0fdc54b285940d7c3853c6118463ac6f8ae663cc703a5" {
		t.Errorf("Incorrect Pubkey  %v", mintInfo.Pubkey)
	}
}
