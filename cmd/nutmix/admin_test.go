// package main
//
// import (
// 	"context"
// 	"fmt"
// 	"net/http/httptest"
// 	"testing"
// 	"time"
//
// 	m "github.com/lescuer97/nutmix/internal/mint"
// 	"github.com/nbd-wtf/go-nostr"
// 	"github.com/testcontainers/testcontainers-go"
// 	"github.com/testcontainers/testcontainers-go/modules/postgres"
// 	"github.com/testcontainers/testcontainers-go/wait"
// )
//
// func TestSetupMintInfoLogin(t *testing.T) {
// 	const posgrespassword = "password"
// 	const postgresuser = "user"
// 	ctx := context.Background()
//
// 	postgresContainer, err := postgres.RunContainer(ctx,
// 		testcontainers.WithImage("postgres:16.2"),
// 		postgres.WithDatabase("postgres"),
// 		postgres.WithUsername(postgresuser),
// 		postgres.WithPassword(posgrespassword),
// 		testcontainers.WithWaitStrategy(
// 			wait.ForLog("database system is ready to accept connections").
// 				WithOccurrence(2).
// 				WithStartupTimeout(5*time.Second)),
// 	)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
//
//     admin_nostr_key := nostr.GeneratePrivateKey()
//     admin_pubKey, err := nostr.GetPublicKey(admin_nostr_key)
// 	if err != nil {
// 		t.Fatal(fmt.Errorf("nostr.GetPublicKey(admin_nostr_key). %w", err))
// 	}
//
// 	connUri, err := postgresContainer.ConnectionString(ctx)
//
// 	if err != nil {
// 		t.Fatal(fmt.Errorf("failed to get connection string: %w", err))
// 	}
//
// 	t.Setenv("DATABASE_URL", connUri)
// 	t.Setenv("MINT_PRIVATE_KEY", MintPrivateKey)
// 	t.Setenv("MINT_LIGHTNING_BACKEND", string(m.FAKE_WALLET))
// 	t.Setenv(m.NETWORK_ENV, "regtest")
//     t.Setenv("ADMIN_NOSTR_NPUB", admin_pubKey )
//
// 	router, _ := SetupRoutingForTesting(ctx)
//
// 	// request mint quote of 1000 sats
// 	w := httptest.NewRecorder()
//
// 	req := httptest.NewRequest("GET", "/admin/login",nil )
//
// 	router.ServeHTTP(w, req)
//
// 	if w.Code != 200 {
// 		t.Errorf("Expected status code 200, got %d", w.Code)
// 	}
//
//     fmt.Printf("login page: %+v", w.Body.String())
//
// }
