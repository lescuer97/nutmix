package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/lightning"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/utils"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestPaymentFailureButPendingCheckPaymentMockDbFakeWallet(t *testing.T) {
	ctx := context.Background()
	t.Setenv("MINT_PRIVATE_KEY", MintPrivateKey)
	t.Setenv("MINT_LIGHTNING_BACKEND", string(utils.FAKE_WALLET))
	t.Setenv(mint.NETWORK_ENV, "regtest")

	router, mint := SetupRoutingForTestingMockDb(ctx, false)

	w := httptest.NewRecorder()

	mintQuoteRequest := cashu.PostMintQuoteBolt11Request{
		Amount: 10000,
		Unit:   cashu.Sat.String(),
	}
	jsonRequestBody, _ := json.Marshal(mintQuoteRequest)

	req := httptest.NewRequest("POST", "/v1/mint/quote/bolt11", strings.NewReader(string(jsonRequestBody)))

	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("Expected status code 200, got %d", w.Code)
	}

	var postMintQuoteResponse cashu.MintRequestDB
	err := json.Unmarshal(w.Body.Bytes(), &postMintQuoteResponse)

	if err != nil {
		t.Errorf("Error unmarshalling response: %v", err)
	}

	referenceKeyset := mint.ActiveKeysets[cashu.Sat.String()][1]

	// ASK FOR SUCCESSFUL MINTING
	blindedMessages, mintingSecrets, mintingSecretKeys, err := CreateBlindedMessages(10000, referenceKeyset)
	if err != nil {
		t.Fatalf("could not createBlind message: %v", err)
	}

	mintRequest := cashu.PostMintBolt11Request{
		Quote:   postMintQuoteResponse.Quote,
		Outputs: blindedMessages,
	}

	jsonRequestBody, _ = json.Marshal(mintRequest)

	req = httptest.NewRequest("POST", "/v1/mint/bolt11", strings.NewReader(string(jsonRequestBody)))

	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	var postMintResponse cashu.PostMintBolt11Response

	if w.Code != 200 {
		t.Fatalf("Expected status code 200, got %d", w.Code)
	}

	err = json.Unmarshal(w.Body.Bytes(), &postMintResponse)

	if err != nil {
		t.Fatalf("Error unmarshalling response: %v", err)
	}

	/// start doing melt quote
	meltQuoteRequest := cashu.PostMeltQuoteBolt11Request{
		Unit:    cashu.Sat.String(),
		Request: RegtestRequest,
	}

	jsonRequestBody, _ = json.Marshal(meltQuoteRequest)

	req = httptest.NewRequest("POST", "/v1/melt/quote/bolt11", strings.NewReader(string(jsonRequestBody)))

	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var postMeltQuoteResponse cashu.PostMeltQuoteBolt11Response
	err = json.Unmarshal(w.Body.Bytes(), &postMeltQuoteResponse)

	if err != nil {
		t.Fatalf("Error unmarshalling response: %v", err)
	}

	// try melting

	w.Flush()
	// errors to lightning to force payment checking
	fakeWallet := lightning.FakeWallet{
		Network: *mint.LightningBackend.GetNetwork(),
		UnpurposeErrors: []lightning.FakeWalletError{
			lightning.FailPaymentFailed, lightning.FailQueryPending,
		},
	}

	mint.LightningBackend = &fakeWallet

	meltProofs, err := GenerateProofs(postMintResponse.Signatures, mint.ActiveKeysets, mintingSecrets, mintingSecretKeys)

	// test melt tokens
	meltRequest := cashu.PostMeltBolt11Request{
		Quote:  postMeltQuoteResponse.Quote,
		Inputs: meltProofs,
	}

	jsonRequestBody, _ = json.Marshal(meltRequest)

	req = httptest.NewRequest("POST", "/v1/melt/bolt11", strings.NewReader(string(jsonRequestBody)))
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var postMeltResponse cashu.PostMeltQuoteBolt11Response

	err = json.Unmarshal(w.Body.Bytes(), &postMeltResponse)

	if err != nil {
		t.Fatalf("Error unmarshalling response: %v", err)
	}

	if !postMeltResponse.Paid {
		t.Errorf("Expected paid to be true because it's a fake wallet, got %v", postMeltResponse.Paid)
	}
	tx, err := mint.MintDB.GetTx(ctx)
	if err != nil {
		t.Fatalf("mint.MintDB.GetTx(): %+v", err)
	}
	defer tx.Rollback(ctx)

	proofs, _ := mint.MintDB.GetProofsFromSecret(tx, []string{meltProofs[0].Secret})

	if proofs[0].State != cashu.PROOF_PENDING {
		t.Errorf("Proof should be pending. it is now: %v", proofs[0].State)
	}

	req = httptest.NewRequest("POST", "/v1/melt/bolt11", strings.NewReader(string(jsonRequestBody)))
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var errorResponse cashu.ErrorResponse

	err = json.Unmarshal(w.Body.Bytes(), &errorResponse)

	if err != nil {
		t.Fatalf("Could not parse error response %s", w.Body.String())
	}

	if errorResponse.Code != cashu.QUOTE_PENDING {
		t.Errorf("Incorrect error code, got %v", errorResponse.Code)
	}

	secreList := []string{}
	for _, p := range meltProofs {
		secreList = append(secreList, p.Secret)
	}

	proofsDB, err := mint.MintDB.GetProofsFromSecret(tx, secreList)
	if err != nil {
		t.Fatalf("mint.MintDB.GetProofsFromSecret() %s", w.Body.String())
	}
	for _, p := range proofsDB {
		if p.State != cashu.PROOF_PENDING {
			t.Errorf("Proof is not pending %+v", p)
		}
	}
	err = tx.Commit(ctx)
	if err != nil {
		t.Fatalf("tx.Commit(ctx) %s", w.Body.String())
	}

}

func TestPaymentFailureButPendingCheckPaymentPostgresFakeWallet(t *testing.T) {
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

	connUri, err := postgresContainer.ConnectionString(ctx)

	if err != nil {
		t.Fatal(fmt.Errorf("failed to get connection string: %w", err))
	}
	t.Setenv("MINT_PRIVATE_KEY", MintPrivateKey)
	t.Setenv("MINT_LIGHTNING_BACKEND", string(utils.FAKE_WALLET))
	t.Setenv(mint.NETWORK_ENV, "regtest")
	t.Setenv("DATABASE_URL", connUri)

	router, mint := SetupRoutingForTesting(ctx, false)

	w := httptest.NewRecorder()

	mintQuoteRequest := cashu.PostMintQuoteBolt11Request{
		Amount: 10000,
		Unit:   cashu.Sat.String(),
	}
	jsonRequestBody, _ := json.Marshal(mintQuoteRequest)

	req := httptest.NewRequest("POST", "/v1/mint/quote/bolt11", strings.NewReader(string(jsonRequestBody)))

	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("Expected status code 200, got %d", w.Code)
	}

	var postMintQuoteResponse cashu.MintRequestDB
	err = json.Unmarshal(w.Body.Bytes(), &postMintQuoteResponse)

	if err != nil {
		t.Errorf("Error unmarshalling response: %v", err)
	}

	referenceKeyset := mint.ActiveKeysets[cashu.Sat.String()][1]

	// ASK FOR SUCCESSFUL MINTING
	blindedMessages, mintingSecrets, mintingSecretKeys, err := CreateBlindedMessages(10000, referenceKeyset)
	if err != nil {
		t.Fatalf("could not createBlind message: %v", err)
	}

	mintRequest := cashu.PostMintBolt11Request{
		Quote:   postMintQuoteResponse.Quote,
		Outputs: blindedMessages,
	}

	jsonRequestBody, _ = json.Marshal(mintRequest)

	req = httptest.NewRequest("POST", "/v1/mint/bolt11", strings.NewReader(string(jsonRequestBody)))

	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	var postMintResponse cashu.PostMintBolt11Response

	if w.Code != 200 {
		t.Fatalf("Expected status code 200, got %d", w.Code)
	}

	err = json.Unmarshal(w.Body.Bytes(), &postMintResponse)

	if err != nil {
		t.Fatalf("Error unmarshalling response: %v", err)
	}

	/// start doing melt quote
	meltQuoteRequest := cashu.PostMeltQuoteBolt11Request{
		Unit:    cashu.Sat.String(),
		Request: RegtestRequest,
	}

	jsonRequestBody, _ = json.Marshal(meltQuoteRequest)

	req = httptest.NewRequest("POST", "/v1/melt/quote/bolt11", strings.NewReader(string(jsonRequestBody)))

	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var postMeltQuoteResponse cashu.PostMeltQuoteBolt11Response
	err = json.Unmarshal(w.Body.Bytes(), &postMeltQuoteResponse)

	if err != nil {
		t.Fatalf("Error unmarshalling response: %v", err)
	}

	// try melting

	w.Flush()
	// errors to lightning to force payment checking
	fakeWallet := lightning.FakeWallet{
		Network: *mint.LightningBackend.GetNetwork(),
		UnpurposeErrors: []lightning.FakeWalletError{
			lightning.FailPaymentFailed, lightning.FailQueryPending,
		},
	}

	mint.LightningBackend = &fakeWallet

	meltProofs, err := GenerateProofs(postMintResponse.Signatures, mint.ActiveKeysets, mintingSecrets, mintingSecretKeys)

	// test melt tokens
	meltRequest := cashu.PostMeltBolt11Request{
		Quote:  postMeltQuoteResponse.Quote,
		Inputs: meltProofs,
	}

	jsonRequestBody, _ = json.Marshal(meltRequest)

	req = httptest.NewRequest("POST", "/v1/melt/bolt11", strings.NewReader(string(jsonRequestBody)))
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var postMeltResponse cashu.PostMeltQuoteBolt11Response

	err = json.Unmarshal(w.Body.Bytes(), &postMeltResponse)

	if err != nil {
		t.Fatalf("Error unmarshalling response: %v", err)
	}

	if !postMeltResponse.Paid {
		t.Errorf("Expected paid to be true because it's a fake wallet, got %v", postMeltResponse.Paid)
	}
	tx, err := mint.MintDB.GetTx(ctx)
	if err != nil {
		t.Fatalf("mint.MintDB.GetTx(): %+v", err)
	}
	defer tx.Rollback(ctx)

	proofs, _ := mint.MintDB.GetProofsFromSecret(tx, []string{meltProofs[0].Secret})

	if proofs[0].State != cashu.PROOF_PENDING {
		t.Errorf("Proof should be pending. it is now: %v", proofs[0].State)
	}

	req = httptest.NewRequest("POST", "/v1/melt/bolt11", strings.NewReader(string(jsonRequestBody)))
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var errorResponse cashu.ErrorResponse

	err = json.Unmarshal(w.Body.Bytes(), &errorResponse)

	if err != nil {
		t.Fatalf("Could not parse error response %s", w.Body.String())
	}

	if errorResponse.Code != cashu.QUOTE_PENDING {
		t.Errorf("Incorrect error code, got %v", errorResponse.Code)
	}

	secreList := []string{}
	for _, p := range meltProofs {
		secreList = append(secreList, p.Secret)
	}

	proofsDB, err := mint.MintDB.GetProofsFromSecret(tx, secreList)
	if err != nil {
		t.Fatalf("mint.MintDB.GetProofsFromSecret() %s", w.Body.String())
	}
	for _, p := range proofsDB {
		if p.State != cashu.PROOF_PENDING {
			t.Errorf("Proof is not pending %+v", p)
		}
	}
	err = tx.Commit(ctx)
	if err != nil {
		t.Fatalf("tx.Commit(ctx) %s", w.Body.String())
	}

}

func TestPaymentPendingButPendingCheckPaymentMockDbFakeWallet(t *testing.T) {
	ctx := context.Background()
	t.Setenv("MINT_PRIVATE_KEY", MintPrivateKey)
	t.Setenv("MINT_LIGHTNING_BACKEND", string(utils.FAKE_WALLET))
	t.Setenv(mint.NETWORK_ENV, "regtest")

	router, mint := SetupRoutingForTestingMockDb(ctx, false)

	w := httptest.NewRecorder()

	mintQuoteRequest := cashu.PostMintQuoteBolt11Request{
		Amount: 10000,
		Unit:   cashu.Sat.String(),
	}
	jsonRequestBody, _ := json.Marshal(mintQuoteRequest)

	req := httptest.NewRequest("POST", "/v1/mint/quote/bolt11", strings.NewReader(string(jsonRequestBody)))

	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("Expected status code 200, got %d", w.Code)
	}

	var postMintQuoteResponse cashu.MintRequestDB
	err := json.Unmarshal(w.Body.Bytes(), &postMintQuoteResponse)

	if err != nil {
		t.Errorf("Error unmarshalling response: %v", err)
	}

	referenceKeyset := mint.ActiveKeysets[cashu.Sat.String()][1]

	// ASK FOR SUCCESSFUL MINTING
	blindedMessages, mintingSecrets, mintingSecretKeys, err := CreateBlindedMessages(10000, referenceKeyset)
	if err != nil {
		t.Fatalf("could not createBlind message: %v", err)
	}

	mintRequest := cashu.PostMintBolt11Request{
		Quote:   postMintQuoteResponse.Quote,
		Outputs: blindedMessages,
	}

	jsonRequestBody, _ = json.Marshal(mintRequest)

	req = httptest.NewRequest("POST", "/v1/mint/bolt11", strings.NewReader(string(jsonRequestBody)))

	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	var postMintResponse cashu.PostMintBolt11Response

	if w.Code != 200 {
		t.Fatalf("Expected status code 200, got %d", w.Code)
	}

	err = json.Unmarshal(w.Body.Bytes(), &postMintResponse)

	if err != nil {
		t.Fatalf("Error unmarshalling response: %v", err)
	}

	/// start doing melt quote
	meltQuoteRequest := cashu.PostMeltQuoteBolt11Request{
		Unit:    cashu.Sat.String(),
		Request: RegtestRequest,
	}

	jsonRequestBody, _ = json.Marshal(meltQuoteRequest)

	req = httptest.NewRequest("POST", "/v1/melt/quote/bolt11", strings.NewReader(string(jsonRequestBody)))

	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var postMeltQuoteResponse cashu.PostMeltQuoteBolt11Response
	err = json.Unmarshal(w.Body.Bytes(), &postMeltQuoteResponse)

	if err != nil {
		t.Fatalf("Error unmarshalling response: %v", err)
	}

	// try melting

	w.Flush()
	// errors to lightning to force payment checking
	fakeWallet := lightning.FakeWallet{
		Network: *mint.LightningBackend.GetNetwork(),
		UnpurposeErrors: []lightning.FakeWalletError{
			lightning.FailPaymentFailed, lightning.FailQueryPending,
		},
	}

	mint.LightningBackend = &fakeWallet

	meltProofs, err := GenerateProofs(postMintResponse.Signatures, mint.ActiveKeysets, mintingSecrets, mintingSecretKeys)

	// test melt tokens
	meltRequest := cashu.PostMeltBolt11Request{
		Quote:  postMeltQuoteResponse.Quote,
		Inputs: meltProofs,
	}

	jsonRequestBody, _ = json.Marshal(meltRequest)

	req = httptest.NewRequest("POST", "/v1/melt/bolt11", strings.NewReader(string(jsonRequestBody)))
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var postMeltResponse cashu.PostMeltQuoteBolt11Response

	err = json.Unmarshal(w.Body.Bytes(), &postMeltResponse)

	if err != nil {
		t.Fatalf("Error unmarshalling response: %v", err)
	}

	if !postMeltResponse.Paid {
		t.Errorf("Expected paid to be true because it's a fake wallet, got %v", postMeltResponse.Paid)
	}

	secreList := []string{}
	for _, p := range meltProofs {
		secreList = append(secreList, p.Secret)
	}
	tx, err := mint.MintDB.GetTx(ctx)
	if err != nil {
		t.Fatalf("mint.MintDB.GetTx(): %+v", err)
	}
	defer tx.Rollback(ctx)

	proofsDB, err := mint.MintDB.GetProofsFromSecret(tx, secreList)
	if err != nil {
		t.Fatalf("mint.MintDB.GetProofsFromSecret() %s", w.Body.String())
	}
	for _, p := range proofsDB {
		if p.State != cashu.PROOF_PENDING {
			t.Errorf("Proof is not pending %+v", p)
		}
	}
	err = tx.Commit(ctx)
	if err != nil {
		t.Fatalf("tx.Commit(ctx) %s", w.Body.String())
	}
}
