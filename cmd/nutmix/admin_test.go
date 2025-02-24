package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/routes/admin"
	"github.com/lescuer97/nutmix/internal/utils"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestSetupMintAdminLoginSuccess(t *testing.T) {
	const posgrespassword = "password"
	const postgresuser = "user"
	ctx := context.Background()

	postgresContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:16.2"),
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

	admin_nostr_key := nostr.GeneratePrivateKey()
	admin_pubKey, err := nostr.GetPublicKey(admin_nostr_key)
	if err != nil {
		t.Fatal(fmt.Errorf("nostr.GetPublicKey(admin_nostr_key). %w", err))
	}
	nip19pubkey, err := nip19.EncodePublicKey(admin_pubKey)

	if err != nil {
		t.Fatal(fmt.Errorf("nip19.EncodePublicKey(). %w", err))
	}

	connUri, err := postgresContainer.ConnectionString(ctx)

	if err != nil {
		t.Fatal(fmt.Errorf("failed to get connection string: %w", err))
	}

	t.Setenv("DATABASE_URL", connUri)
	t.Setenv("MINT_PRIVATE_KEY", MintPrivateKey)
	t.Setenv("MINT_LIGHTNING_BACKEND", string(utils.FAKE_WALLET))
	t.Setenv(m.NETWORK_ENV, "regtest")
	t.Setenv("ADMIN_NOSTR_NPUB", nip19pubkey)
	t.Setenv("TEST_PATH", "../../internal/routes/admin/")

	router, _ := SetupRoutingForTesting(ctx, true)

	// get login nonce
	w := httptest.NewRecorder()

	req := httptest.NewRequest("GET", "/admin/login", nil)
	// set content header JSON
	req.Header.Set("Content-Type", gin.MIMEJSON)

	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("Expected status code 200, got %d", w.Code)
	}

	var loginParams admin.LoginParams
	err = json.Unmarshal(w.Body.Bytes(), &loginParams)

	if err != nil {
		t.Errorf("Error unmarshalling response: %v", err)
	}

	// sign nonce with admin nostr privkey
	eventToSign := nostr.Event{
		Kind:      27235,
		Content:   loginParams.Nonce,
		CreatedAt: nostr.Timestamp(time.Now().Unix()),
	}

	err = eventToSign.Sign(admin_nostr_key)
	if err != nil {
		t.Errorf("eventToSign.Sign(admin_nostr_key) %v", err)
	}

	w = httptest.NewRecorder()
	jsonRequestBody, err := json.Marshal(eventToSign)
	if err != nil {
		t.Errorf("json.Marshal(eventToSign) %v", err)
	}

	req = httptest.NewRequest("POST", "/admin/login", strings.NewReader(string(jsonRequestBody)))

	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("Expected status code 200, got %d", w.Code)
	}

}

func TestSetupMintAdminLoginFailure(t *testing.T) {
	const posgrespassword = "password"
	const postgresuser = "user"
	ctx := context.Background()

	postgresContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:16.2"),
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

	admin_nostr_key := nostr.GeneratePrivateKey()
	admin_pubKey, err := nostr.GetPublicKey(admin_nostr_key)
	if err != nil {
		t.Fatal(fmt.Errorf("nostr.GetPublicKey(admin_nostr_key). %w", err))
	}

	nip19pubkey, err := nip19.EncodePublicKey(admin_pubKey)

	if err != nil {
		t.Fatal(fmt.Errorf("nip19.EncodePublicKey(). %w", err))
	}
	connUri, err := postgresContainer.ConnectionString(ctx)

	if err != nil {
		t.Fatal(fmt.Errorf("failed to get connection string: %w", err))
	}

	t.Setenv("DATABASE_URL", connUri)
	t.Setenv("MINT_PRIVATE_KEY", MintPrivateKey)
	t.Setenv("MINT_LIGHTNING_BACKEND", string(utils.FAKE_WALLET))
	t.Setenv(m.NETWORK_ENV, "regtest")
	t.Setenv("ADMIN_NOSTR_NPUB", nip19pubkey)
	t.Setenv("TEST_PATH", "../../internal/routes/admin/")

	router, _ := SetupRoutingForTesting(ctx, true)

	// get login nonce
	w := httptest.NewRecorder()

	req := httptest.NewRequest("GET", "/admin/login", nil)
	// set content header JSON
	req.Header.Set("Content-Type", gin.MIMEJSON)

	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("Expected status code 200, got %d", w.Code)
	}

	var loginParams admin.LoginParams
	err = json.Unmarshal(w.Body.Bytes(), &loginParams)

	if err != nil {
		t.Errorf("Error unmarshalling response: %v", err)
	}

	// sign nonce with admin nostr privkey
	eventToSign := nostr.Event{
		Kind:      27235,
		Content:   loginParams.Nonce,
		CreatedAt: nostr.Timestamp(time.Now().Unix()),
	}

	adminWrongKey := nostr.GeneratePrivateKey()
	// using wrong key
	err = eventToSign.Sign(adminWrongKey)
	if err != nil {
		t.Errorf("eventToSign.Sign(admin_nostr_key) %v", err)
	}

	w = httptest.NewRecorder()
	jsonRequestBody, err := json.Marshal(eventToSign)
	if err != nil {
		t.Errorf("json.Marshal(eventToSign) %v", err)
	}

	req = httptest.NewRequest("POST", "/admin/login", strings.NewReader(string(jsonRequestBody)))
	req.Header.Set("Content-Type", gin.MIMEJSON)

	router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("Expected status code 400, got %d", w.Code)
	}

	var res string

	err = json.Unmarshal(w.Body.Bytes(), &res)
	if err != nil {
		t.Errorf("Error unmarshalling response: %v", err)
	}

	if res != "Private key used is not correct" {
		t.Errorf("Expected to get Private key used is not correct %s", res)
	}
}

func TestRotateKeyUpCall(t *testing.T) {
	const posgrespassword = "password"
	const postgresuser = "user"
	ctx := context.Background()

	postgresContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:16.2"),
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

	admin_nostr_key := nostr.GeneratePrivateKey()
	admin_pubKey, err := nostr.GetPublicKey(admin_nostr_key)
	if err != nil {
		t.Fatal(fmt.Errorf("nostr.GetPublicKey(admin_nostr_key). %w", err))
	}
	nip19pubkey, err := nip19.EncodePublicKey(admin_pubKey)

	if err != nil {
		t.Fatal(fmt.Errorf("nip19.EncodePublicKey(). %w", err))
	}

	connUri, err := postgresContainer.ConnectionString(ctx)

	if err != nil {
		t.Fatal(fmt.Errorf("failed to get connection string: %w", err))
	}

	t.Setenv("DATABASE_URL", connUri)
	t.Setenv("MINT_PRIVATE_KEY", MintPrivateKey)
	t.Setenv("MINT_LIGHTNING_BACKEND", string(utils.FAKE_WALLET))
	t.Setenv(m.NETWORK_ENV, "regtest")
	t.Setenv("ADMIN_NOSTR_NPUB", nip19pubkey)
	t.Setenv("TEST_PATH", "../../internal/routes/admin/")

	router, _ := SetupRoutingForTesting(ctx, true)

	// get login nonce
	w := httptest.NewRecorder()

	req := httptest.NewRequest("GET", "/admin/login", nil)
	// set content header JSON
	req.Header.Set("Content-Type", gin.MIMEJSON)

	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("Expected status code 200, got %d", w.Code)
	}

	var loginParams admin.LoginParams
	err = json.Unmarshal(w.Body.Bytes(), &loginParams)

	if err != nil {
		t.Errorf("Error unmarshalling response: %v", err)
	}

	// sign nonce with admin nostr privkey
	eventToSign := nostr.Event{
		Kind:      27235,
		Content:   loginParams.Nonce,
		CreatedAt: nostr.Timestamp(time.Now().Unix()),
	}

	err = eventToSign.Sign(admin_nostr_key)
	if err != nil {
		t.Errorf("eventToSign.Sign(admin_nostr_key) %v", err)
	}

	w = httptest.NewRecorder()
	jsonRequestBody, err := json.Marshal(eventToSign)
	if err != nil {
		t.Errorf("json.Marshal(eventToSign) %v", err)
	}

	req = httptest.NewRequest("POST", "/admin/login", strings.NewReader(string(jsonRequestBody)))

	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("Expected status code 200, got %d", w.Code)
	}

	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("Expected status code 200, got %d", w.Code)
	}

	// ask for fee
	w = httptest.NewRecorder()

	rotatingFeeRequest := admin.RotateRequest{
		Fee: 100,
	}

	jsonRequestBody, err = json.Marshal(rotatingFeeRequest)
	if err != nil {
		t.Errorf("json.Marshal(rotatingFeeRequest) %v", err)
	}

	req = httptest.NewRequest("POST", "/admin/rotate/sats", strings.NewReader(string(jsonRequestBody)))

	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("Expected status code 200, got %d", w.Code)
	}
}
