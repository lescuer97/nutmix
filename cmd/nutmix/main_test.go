package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	mockdb "github.com/lescuer97/nutmix/internal/database/mock_db"
	pq "github.com/lescuer97/nutmix/internal/database/postgresql"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/routes"
	"github.com/lescuer97/nutmix/internal/routes/admin"
	"github.com/lescuer97/nutmix/internal/signer"
	localsigner "github.com/lescuer97/nutmix/internal/signer/local_signer"
	"github.com/lescuer97/nutmix/internal/utils"
	"github.com/lescuer97/nutmix/pkg/crypto"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const MintPrivateKey string = "0000000000000000000000000000000000000000000000000000000000000001"

const RegtestRequest string = "lnbcrt10u1pnxrpvhpp535rl7p9ze2dpgn9mm0tljyxsm980quy8kz2eydj7p4awra453u9qdqqcqzzsxqyz5vqsp55mdr2l90rhluaz9v3cmrt0qgjusy2dxsempmees6spapqjuj9m5q9qyyssq863hqzs6lcptdt7z5w82m4lg09l2d27al2wtlade6n4xu05u0gaxfjxspns84a73tl04u3t0pv4lveya8j0eaf9w7y5pstu70grpxtcqla7sxq"

func TestMintBolt11FakeWallet(t *testing.T) {

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

	t.Setenv("DATABASE_URL", connUri)
	t.Setenv("MINT_PRIVATE_KEY", MintPrivateKey)
	t.Setenv("MINT_LIGHTNING_BACKEND", string(utils.FAKE_WALLET))
	t.Setenv(mint.NETWORK_ENV, "regtest")

	router, mint := SetupRoutingForTesting(ctx, false)

	// MINTING TESTING STARTS

	// request mint quote of 1000 sats
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

	if postMintQuoteResponse.RequestPaid {
		t.Errorf("Expected paid to be true because it's a fake wallet, got %v", postMintQuoteResponse.RequestPaid)
	}
	if postMintQuoteResponse.State != cashu.UNPAID {
		t.Errorf("Expected state to be UNPAID, got %v", postMintQuoteResponse.State)

	}

	if postMintQuoteResponse.Unit != "sat" {
		t.Errorf("Expected unit to be sat, got %v", postMintQuoteResponse.Unit)
	}

	w.Flush()

	// check quote request
	req = httptest.NewRequest("GET", "/v1/mint/quote/bolt11"+"/"+postMintQuoteResponse.Quote, strings.NewReader(string(jsonRequestBody)))

	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)
	var postMintQuoteResponseTwo cashu.MintRequestDB

	err = json.Unmarshal(w.Body.Bytes(), &postMintQuoteResponseTwo)

	if err != nil {
		t.Fatalf("Error unmarshalling response: %v", err)
	}

	if !postMintQuoteResponseTwo.RequestPaid {
		t.Errorf("Expected paid to be true because it's a fake wallet, got %v", postMintQuoteResponseTwo.RequestPaid)
	}

	if postMintQuoteResponse.State != cashu.UNPAID {
		t.Errorf("Expected state to be UNPAID, got %v", postMintQuoteResponse.State)

	}

	if postMintQuoteResponseTwo.Unit != "sat" {
		t.Errorf("Expected unit to be sat, got %v", postMintQuoteResponseTwo.Unit)
	}

	w.Flush()

	activeKeys, err := mint.Signer.GetActiveKeys()
	if err != nil {
		t.Fatalf("mint.Signer.GetKeysByUnit(cashu.Sat): %v", err)
	}

	// ASK FOR MINTING WITH TOO MANY BLINDED MESSAGES
	blindedMessages, _, _, err := CreateBlindedMessages(999999, activeKeys)
	if err != nil {
		t.Fatalf("could not createBlind message: %v", err)
	}

	mintRequestTooManyBlindMessages := cashu.PostMintBolt11Request{
		Quote:   postMintQuoteResponse.Quote,
		Outputs: blindedMessages,
	}

	jsonRequestBody, _ = json.Marshal(mintRequestTooManyBlindMessages)

	req = httptest.NewRequest("POST", "/v1/mint/bolt11", strings.NewReader(string(jsonRequestBody)))

	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Fatalf("Expected status code 200, got %d", w.Code)
	}

	if w.Body.String() != `"Amounts in outputs are not the same"` {
		t.Errorf("Expected Amounts in outputs are not the same, got %s", w.Body.String())
	}

	// ASK FOR SUCCESSFUL MINTING
	blindedMessages, mintingSecrets, mintingSecretKeys, err := CreateBlindedMessages(10000, activeKeys)
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

	var totalAmountSigned uint64 = 0

	for _, output := range postMintResponse.Signatures {
		totalAmountSigned += output.Amount
	}

	if totalAmountSigned != 10000 {
		t.Errorf("Expected total amount signed to be 1000, got %d", totalAmountSigned)
	}

	if postMintResponse.Signatures[0].Id != activeKeys.Keysets[0].Id {
		t.Errorf("Expected id to be %s, got %s", activeKeys.Keysets[0].Id, postMintResponse.Signatures[0].Id)
	}

	// lookup in the db if quote shows as issued
	req = httptest.NewRequest("GET", "/v1/mint/quote/bolt11"+"/"+postMintQuoteResponse.Quote, strings.NewReader(string(jsonRequestBody)))

	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)
	postMintQuoteResponseTwo = cashu.MintRequestDB{}

	err = json.Unmarshal(w.Body.Bytes(), &postMintQuoteResponseTwo)

	if err != nil {
		t.Fatalf("Error unmarshalling response: %v", err)
	}

	if postMintQuoteResponseTwo.State != cashu.ISSUED {
		t.Errorf("Expected state to be MINTED, got %v", postMintQuoteResponseTwo.State)
	}

	// try to remint tokens with other blinded signatures
	reMintBlindedMessages, _, _, err := CreateBlindedMessages(1000, activeKeys)
	if err != nil {
		t.Fatalf("could not createBlind message: %v", err)
	}

	reMintRequest := cashu.PostMintBolt11Request{
		Quote:   postMintQuoteResponse.Quote,
		Outputs: reMintBlindedMessages,
	}

	jsonRequestBody, _ = json.Marshal(reMintRequest)

	req = httptest.NewRequest("POST", "/v1/mint/bolt11", strings.NewReader(string(jsonRequestBody)))

	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var errorResponse cashu.ErrorResponse

	err = json.Unmarshal(w.Body.Bytes(), &errorResponse)

	if err != nil {
		t.Fatalf("Could not parse error response %s", w.Body.String())
	}

	if errorResponse.Code != 20002 {
		t.Errorf("Incorrect error code, got %v", errorResponse.Code)

	}
	if errorResponse.Error != "Tokens have already been issued for quote" {
		t.Errorf("Incorrect error string, got %s", errorResponse.Error)

	}

	// Minting with invalid signatures
	w = httptest.NewRecorder()
	mintExcessQuoteRequest := cashu.PostMintQuoteBolt11Request{
		Amount: 10000000,
		Unit:   cashu.Sat.String(),
	}
	jsonRequestBody, _ = json.Marshal(mintExcessQuoteRequest)

	req = httptest.NewRequest("POST", "/v1/mint/quote/bolt11", strings.NewReader(string(jsonRequestBody)))

	router.ServeHTTP(w, req)

	excesMintingBlindMessage, _, _, err := CreateBlindedMessages(10000000, activeKeys)

	err = json.Unmarshal(w.Body.Bytes(), &postMintQuoteResponse)

	if err != nil {
		t.Errorf("Error unmarshalling response: %v", err)
	}

	excesMintingBlindMessage[0].B_ = "badsig"

	excessMintRequest := cashu.PostMintBolt11Request{
		Quote:   postMintQuoteResponse.Quote,
		Outputs: excesMintingBlindMessage,
	}

	jsonRequestBody, _ = json.Marshal(excessMintRequest)

	req = httptest.NewRequest("POST", "/v1/mint/bolt11", strings.NewReader(string(jsonRequestBody)))

	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	errorResponse = cashu.ErrorResponse{}

	err = json.Unmarshal(w.Body.Bytes(), &errorResponse)

	if w.Code != 400 {
		t.Errorf("Expected status code 400, got %d", w.Code)
	}
	if errorResponse.Code != cashu.TOKEN_NOT_VERIFIED {
		t.Errorf(`Expected code be Minting disables. Got:  %s`, errorResponse.Code)
	}
	if errorResponse.Error != "Proof could not be verified" {
		t.Errorf(`Expected code be Minting disables. Got:  %s`, errorResponse.Error)
	}

	// MINTING TESTING ENDS

	// SWAP TESTING STARTS

	// TRY TO SWAP WITH TOO MANY BLINDED MESSAGES
	swapProofs, err := GenerateProofs(postMintResponse.Signatures, activeKeys, mintingSecrets, mintingSecretKeys)

	if err != nil {
		t.Fatalf("Error generating proofs: %v", err)
	}
	swapBlindedMessages, _, _, err := CreateBlindedMessages(1032843, activeKeys)
	if err != nil {
		t.Fatalf("could not createBlind message: %v", err)
	}

	swapRequestToManyBlindMessages := cashu.PostSwapRequest{
		Inputs:  swapProofs,
		Outputs: swapBlindedMessages,
	}

	jsonRequestBody, _ = json.Marshal(swapRequestToManyBlindMessages)

	req = httptest.NewRequest("POST", "/v1/swap", strings.NewReader(string(jsonRequestBody)))

	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	errorResponse = cashu.ErrorResponse{}

	err = json.Unmarshal(w.Body.Bytes(), &errorResponse)

	if err != nil {
		t.Fatalf("Could not parse error response %s", w.Body.String())
	}

	if errorResponse.Code != 11002 {
		t.Errorf("Incorrect error code, got %v", errorResponse.Code)

	}
	if errorResponse.Error != "Transaction is not balanced (inputs != outputs)" {
		t.Errorf("Incorrect error string, got %s", errorResponse.Error)

	}

	// TRY TO SWAP SUCCESSFULLY
	swapProofs, err = GenerateProofs(postMintResponse.Signatures, activeKeys, mintingSecrets, mintingSecretKeys)

	if err != nil {
		t.Fatalf("Error generating proofs: %v", err)
	}
	swapBlindedMessages, swapSecrets, swapPrivateKeySecrets, err := CreateBlindedMessages(2000, activeKeys)
	if err != nil {
		t.Fatalf("could not createBlind message: %v", err)
	}

	swapRequest := cashu.PostSwapRequest{
		Inputs:  swapProofs,
		Outputs: swapBlindedMessages,
	}

	jsonRequestBody, _ = json.Marshal(swapRequest)

	req = httptest.NewRequest("POST", "/v1/swap", strings.NewReader(string(jsonRequestBody)))

	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	var postSwapResponse cashu.PostSwapResponse

	if w.Code != 200 {
		t.Fatalf("Expected status code 200, got %d", w.Code)
	}

	err = json.Unmarshal(w.Body.Bytes(), &postSwapResponse)

	if err != nil {
		t.Fatalf("Error unmarshalling response: %v", err)
	}

	totalAmountSigned = 0

	for _, output := range postSwapResponse.Signatures {
		totalAmountSigned += output.Amount
	}

	if totalAmountSigned != 2000 {
		t.Errorf("Expected total amount signed to be 1000, got %d", totalAmountSigned)
	}

	if postSwapResponse.Signatures[0].Id != activeKeys.Keysets[0].Id {
		t.Errorf("Expected id to be %s, got %s", activeKeys.Keysets[0].Id, postSwapResponse.Signatures[0].Id)
	}

	w.Flush()

	// SWAP WITH INVALID PROOFS
	invalidSignatureProofs, err := GenerateProofs(postSwapResponse.Signatures, activeKeys, swapSecrets, swapPrivateKeySecrets)
	if err != nil {
		t.Fatalf("Error generating proofs: %v", err)
	}
	swapInvalidSigBlindedMessages, _, _, err := CreateBlindedMessages(1000, activeKeys)
	if err != nil {
		t.Fatalf("could not createBlind message: %v", err)
	}

	invalidSignatureProofs[0].C = "badSig"
	invalidSignatureProofs[len(invalidSignatureProofs)-1].C = "badSig"

	invalidSwapRequest := cashu.PostSwapRequest{
		Inputs:  invalidSignatureProofs,
		Outputs: swapInvalidSigBlindedMessages,
	}

	jsonRequestBody, _ = json.Marshal(invalidSwapRequest)

	req = httptest.NewRequest("POST", "/v1/swap", strings.NewReader(string(jsonRequestBody)))

	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	errorResponse = cashu.ErrorResponse{}

	err = json.Unmarshal(w.Body.Bytes(), &errorResponse)

	log.Printf("body: %v", string(w.Body.Bytes()))
	if err != nil {
		t.Fatalf("Could not parse error response %s", w.Body.String())
	}

	if errorResponse.Code != 10003 {
		t.Errorf("Incorrect error code, got %v", errorResponse.Code)

	}
	if errorResponse.Error != "Proof could not be verified" {
		t.Errorf("Incorrect error string, got %s", errorResponse.Error)

	}

	w.Flush()

	// swap with  not enought proofs for compared to signatures
	proofsForRemoving, err := GenerateProofs(postSwapResponse.Signatures, activeKeys, swapSecrets, swapPrivateKeySecrets)
	if err != nil {
		t.Fatalf("Error generating proofs: %v", err)
	}
	signaturesForRemoving, _, _, err := CreateBlindedMessages(1000, activeKeys)
	if err != nil {
		t.Fatalf("could not createBlind message: %v", err)
	}

	notEnoughtProofsSwapRequest := cashu.PostSwapRequest{
		Inputs:  proofsForRemoving[:len(proofsForRemoving)-2],
		Outputs: signaturesForRemoving,
	}

	jsonRequestBody, _ = json.Marshal(notEnoughtProofsSwapRequest)

	req = httptest.NewRequest("POST", "/v1/swap", strings.NewReader(string(jsonRequestBody)))

	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	errorResponse = cashu.ErrorResponse{}

	err = json.Unmarshal(w.Body.Bytes(), &errorResponse)

	if err != nil {
		t.Fatalf("Could not parse error response %s", w.Body.String())
	}

	if errorResponse.Code != 11002 {
		t.Errorf("Incorrect error code, got %v", errorResponse.Code)

	}
	if errorResponse.Error != "Transaction is not balanced (inputs != outputs)" {
		t.Errorf("Incorrect error string, got %s", errorResponse.Error)

	}

	w.Flush()

	// SWAP TESTING ENDS

	// MELTING TESTING STARTS

	// test melt tokens
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

	if postMeltQuoteResponse.Paid {
		t.Errorf("Expected paid to be true because it's a fake wallet, got %v", postMeltQuoteResponse.Paid)
	}

	if postMeltQuoteResponse.State != cashu.UNPAID {
		t.Errorf("Expected state to be UNPAID, got %v", postMeltQuoteResponse.State)
	}

	if postMeltQuoteResponse.Amount != 1000 {
		t.Errorf("Expected amount to be 1000, got %d", postMeltQuoteResponse.Amount)
	}

	// test melt tokens quote call
	req = httptest.NewRequest("GET", "/v1/melt/quote/bolt11"+"/"+postMeltQuoteResponse.Quote, nil)

	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var postMeltQuoteResponseTwo cashu.PostMeltQuoteBolt11Response
	err = json.Unmarshal(w.Body.Bytes(), &postMeltQuoteResponseTwo)

	if err != nil {
		t.Fatalf("Error unmarshalling response: %v", err)

	}

	if postMeltQuoteResponse.Paid {
		t.Errorf("Expected paid to be true because it's a fake wallet, got %v", postMeltQuoteResponse.Paid)
	}

	if postMeltQuoteResponse.State != cashu.UNPAID {
		t.Errorf("Expected state to be UNPAID, got %v", postMeltQuoteResponse.State)
	}

	if postMeltQuoteResponse.Amount != 1000 {
		t.Errorf("Expected amount to be 1000, got %d", postMeltQuoteResponse.Amount)
	}

	meltProofs, err := GenerateProofs(postSwapResponse.Signatures, activeKeys, swapSecrets, swapPrivateKeySecrets)

	if err != nil {
		t.Fatalf("Error generating proofs: %v", err)
	}

	// test melt with invalid proofs
	InvalidProofsMeltRequest := cashu.PostMeltBolt11Request{
		Quote:  postMeltQuoteResponse.Quote,
		Inputs: meltProofs,
	}

	InvalidProofsMeltRequest.Inputs[0].C = "badSig"

	jsonRequestBody, _ = json.Marshal(InvalidProofsMeltRequest)

	req = httptest.NewRequest("POST", "/v1/melt/bolt11", strings.NewReader(string(jsonRequestBody)))
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("Expected status code 403, got %d", w.Code)
	}
	var errorRes cashu.ErrorResponse
	err = json.Unmarshal(w.Body.Bytes(), &errorRes)
	if err != nil {
		t.Fatalf("json.Unmarshal(w.Body.Bytes(), &errorRes): %v", err)
	}

	fmt.Println("body ", string(w.Body.Bytes()))

	if errorRes.Code != cashu.TOKEN_NOT_VERIFIED {
		t.Errorf("Expected Invalid Proof, got %s", w.Body.String())
	}

	w.Flush()

	meltProofs, err = GenerateProofs(postSwapResponse.Signatures, activeKeys, swapSecrets, swapPrivateKeySecrets)

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
	if postMeltResponse.State != cashu.PAID {
		t.Errorf("Expected state to be Paid, got %v", postMintQuoteResponseTwo.State)
	}
	if postMeltResponse.PaymentPreimage != "fakewalletpreimage" {
		t.Errorf("Expected payment preimage to be empty, got %s", postMeltResponse.PaymentPreimage)
	}

	// Test melt that has already been melted

	req = httptest.NewRequest("POST", "/v1/melt/bolt11", strings.NewReader(string(jsonRequestBody)))
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	errorResponse = cashu.ErrorResponse{}

	err = json.Unmarshal(w.Body.Bytes(), &errorResponse)

	if err != nil {
		t.Fatalf("Could not parse error response %s", w.Body.String())
	}

	if errorResponse.Code != 20006 {
		t.Errorf("Incorrect error code, got %v", errorResponse.Code)

	}
	if errorResponse.Error != "Invoice already paid" {
		t.Errorf("Incorrect error string, got %s", errorResponse.Error)

	}

	// MELTING TESTING ENDS

	// Clean up the container
	defer func() {
		if err := postgresContainer.Terminate(ctx); err != nil {
			log.Fatalf("failed to terminate container: %s", err)
		}

	}()

}

func SetupRoutingForTesting(ctx context.Context, adminRoute bool) (*gin.Engine, *mint.Mint) {

	db, err := pq.DatabaseSetup(ctx, "../../migrations/")
	if err != nil {
		log.Fatal("Error conecting to db", err)
	}

	config, err := mint.SetUpConfigDB(db)

	config.MINT_LIGHTNING_BACKEND = utils.StringToLightningBackend(os.Getenv(mint.MINT_LIGHTNING_BACKEND_ENV))

	config.NETWORK = os.Getenv(mint.NETWORK_ENV)
	config.LND_GRPC_HOST = os.Getenv(utils.LND_HOST)
	config.LND_TLS_CERT = os.Getenv(utils.LND_TLS_CERT)
	config.LND_MACAROON = os.Getenv(utils.LND_MACAROON)
	config.MINT_LNBITS_KEY = os.Getenv(utils.MINT_LNBITS_KEY)
	config.MINT_LNBITS_ENDPOINT = os.Getenv(utils.MINT_LNBITS_ENDPOINT)

	if err != nil {
		log.Fatalf("could not setup config file: %+v ", err)
	}

	signer, err := localsigner.SetupLocalSigner(db)
	if err != nil {
		log.Fatalf("localsigner.SetupLocalSigner(db): %+v ", err)
	}

	mint, err := mint.SetUpMint(ctx, config, db, &signer)

	if err != nil {
		log.Fatalf("SetUpMint: %+v ", err)
	}

	r := gin.Default()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	routes.V1Routes(r, mint, logger)

	if adminRoute {
		admin.AdminRoutes(ctx, r, mint, logger)
	}

	return r, mint
}
func SetupRoutingForTestingMockDb(ctx context.Context, adminRoute bool) (*gin.Engine, *mint.Mint) {
	db := mockdb.MockDB{}

	signer, err := localsigner.SetupLocalSigner(&db)
	if err != nil {
		log.Fatalf("localsigner.SetupLocalSigner(&db): %+v ", err)
	}

	config, err := mint.SetUpConfigDB(&db)

	config.MINT_LIGHTNING_BACKEND = utils.StringToLightningBackend(os.Getenv(mint.MINT_LIGHTNING_BACKEND_ENV))

	config.NETWORK = os.Getenv(mint.NETWORK_ENV)
	config.LND_GRPC_HOST = os.Getenv(utils.LND_HOST)
	config.LND_TLS_CERT = os.Getenv(utils.LND_TLS_CERT)
	config.LND_MACAROON = os.Getenv(utils.LND_MACAROON)
	config.MINT_LNBITS_KEY = os.Getenv(utils.MINT_LNBITS_KEY)
	config.MINT_LNBITS_ENDPOINT = os.Getenv(utils.MINT_LNBITS_ENDPOINT)

	if err != nil {
		log.Fatalf("could not setup config file: %+v ", err)
	}

	mint, err := mint.SetUpMint(ctx, config, &db, &signer)

	if err != nil {
		log.Fatalf("SetUpMint: %+v ", err)
	}

	r := gin.Default()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	routes.V1Routes(r, mint, logger)

	if adminRoute {
		admin.AdminRoutes(ctx, r, mint, logger)
	}

	return r, mint
}

func newBlindedMessage(id string, amount uint64, B_ *secp256k1.PublicKey) cashu.BlindedMessage {
	B_str := hex.EncodeToString(B_.SerializeCompressed())
	return cashu.BlindedMessage{Amount: amount, B_: B_str, Id: id}
}

// returns Blinded messages, secrets - [][]byte, and list of r
func CreateBlindedMessages(amount uint64, keyset signer.GetKeysResponse) ([]cashu.BlindedMessage, []string, []*secp256k1.PrivateKey, error) {
	splitAmounts := cashu.AmountSplit(amount)
	splitLen := len(splitAmounts)

	blindedMessages := make([]cashu.BlindedMessage, splitLen)
	secrets := make([]string, splitLen)
	rs := make([]*secp256k1.PrivateKey, splitLen)

	for i, amt := range splitAmounts {
		// generate new private key r
		r, err := secp256k1.GeneratePrivateKey()
		if err != nil {
			return nil, nil, nil, err
		}

		var B_ *secp256k1.PublicKey
		var secret string
		// generate random secret until it finds valid point
		for {
			secretBytes := make([]byte, 32)
			_, err = rand.Read(secretBytes)
			if err != nil {
				return nil, nil, nil, err
			}
			secret = hex.EncodeToString(secretBytes)
			B_, r, err = crypto.BlindMessage(secret, r)
			if err == nil {
				break
			}
		}

		blindedMessage := newBlindedMessage(keyset.Keysets[0].Id, amt, B_)
		blindedMessages[i] = blindedMessage
		secrets[i] = secret
		rs[i] = r
	}

	return blindedMessages, secrets, rs, nil
}

func TestMintBolt11LndLigthning(t *testing.T) {

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

	t.Setenv("DATABASE_URL", connUri)
	t.Setenv("MINT_PRIVATE_KEY", MintPrivateKey)
	t.Setenv("MINT_LIGHTNING_BACKEND", "LndGrpcWallet")
	t.Setenv(mint.NETWORK_ENV, "regtest")

	ctx = context.WithValue(ctx, mint.NETWORK_ENV, os.Getenv(mint.NETWORK_ENV))
	ctx = context.WithValue(ctx, mint.MINT_LIGHTNING_BACKEND_ENV, os.Getenv(mint.MINT_LIGHTNING_BACKEND_ENV))
	ctx = context.WithValue(ctx, database.DATABASE_URL_ENV, os.Getenv(database.DATABASE_URL_ENV))
	ctx = context.WithValue(ctx, mint.NETWORK_ENV, os.Getenv(mint.NETWORK_ENV))

	_, bobLnd, _, _, err := utils.SetUpLightingNetworkTestEnviroment(ctx, "bolt11-tests")

	ctx = context.WithValue(ctx, utils.LND_HOST, os.Getenv(utils.LND_HOST))
	ctx = context.WithValue(ctx, utils.LND_TLS_CERT, os.Getenv(utils.LND_TLS_CERT))
	ctx = context.WithValue(ctx, utils.LND_MACAROON, os.Getenv(utils.LND_MACAROON))

	if err != nil {
		t.Fatalf("Error setting up lightning network enviroment: %+v", err)
	}

	LightningBolt11Test(t, ctx, bobLnd)

	// Clean up the container
	defer func() {
		if err := postgresContainer.Terminate(ctx); err != nil {
			log.Fatalf("failed to terminate container: %s", err)
		}

	}()

}
func TestMintBolt11LNBITSLigthning(t *testing.T) {

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

	t.Setenv("DATABASE_URL", connUri)
	t.Setenv("MINT_PRIVATE_KEY", MintPrivateKey)
	t.Setenv("MINT_LIGHTNING_BACKEND", "LNbitsWallet")
	t.Setenv(mint.NETWORK_ENV, "regtest")

	ctx = context.WithValue(ctx, mint.NETWORK_ENV, os.Getenv(mint.NETWORK_ENV))
	ctx = context.WithValue(ctx, mint.MINT_LIGHTNING_BACKEND_ENV, os.Getenv(mint.MINT_LIGHTNING_BACKEND_ENV))
	ctx = context.WithValue(ctx, database.DATABASE_URL_ENV, os.Getenv(database.DATABASE_URL_ENV))
	ctx = context.WithValue(ctx, mint.NETWORK_ENV, os.Getenv(mint.NETWORK_ENV))

	_, bobLnd, _, _, err := utils.SetUpLightingNetworkTestEnviroment(ctx, "lnbits-bolt11-tests")

	ctx = context.WithValue(ctx, utils.MINT_LNBITS_ENDPOINT, os.Getenv(utils.MINT_LNBITS_ENDPOINT))
	ctx = context.WithValue(ctx, utils.MINT_LNBITS_KEY, os.Getenv(utils.MINT_LNBITS_KEY))

	if err != nil {
		t.Fatalf("Error setting up lightning network enviroment: %+v", err)
	}

	LightningBolt11Test(t, ctx, bobLnd)

	// Clean up the container
	defer func() {
		if err := postgresContainer.Terminate(ctx); err != nil {
			log.Fatalf("failed to terminate container: %s", err)
		}

	}()

}

func GenerateProofs(signatures []cashu.BlindSignature, keyset signer.GetKeysResponse, secrets []string, secretsKey []*secp256k1.PrivateKey) ([]cashu.Proof, error) {

	// try to swap tokens
	var proofs []cashu.Proof
	// unblid the signatures and make proofs
	for i, output := range signatures {

		parsedBlindFactor, err := hex.DecodeString(output.C_)
		if err != nil {
			return nil, fmt.Errorf("Error decoding hex: %w", err)
		}
		blindedFactor, err := secp256k1.ParsePubKey(parsedBlindFactor)
		if err != nil {
			return nil, fmt.Errorf("Error parsing pubkey: %w", err)
		}

		amountStr := strconv.FormatUint(output.Amount, 10)
		pubkeyStr := keyset.Keysets[0].Keys[amountStr]
		pubkeyBytes, err := hex.DecodeString(pubkeyStr)
		if err != nil {
			return nil, fmt.Errorf("hex.DecodeString(pubkeyStr): %w", err)
		}
		mintPublicKey, err := secp256k1.ParsePubKey(pubkeyBytes)
		if err != nil {
			return nil, fmt.Errorf("Error parsing pubkey: %w", err)
		}

		C := crypto.UnblindSignature(blindedFactor, secretsKey[i], mintPublicKey)

		hexC := hex.EncodeToString(C.SerializeCompressed())

		proofs = append(proofs, cashu.Proof{Id: output.Id, Amount: output.Amount, C: hexC, Secret: secrets[i]})
	}

	return proofs, nil
}

func LightningBolt11Test(t *testing.T, ctx context.Context, bobLnd testcontainers.Container) {
	router, mint := SetupRoutingForTesting(ctx, false)

	// MINTING TESTING STARTS

	// request mint quote of 1000 sats
	w := httptest.NewRecorder()

	mintQuoteRequest := cashu.PostMintQuoteBolt11Request{
		Amount: 1000,
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

	if postMintQuoteResponse.RequestPaid {
		t.Errorf("Expected paid to be false because it's a lnd node, got %v", postMintQuoteResponse.RequestPaid)
	}
	if postMintQuoteResponse.State != cashu.UNPAID {
		t.Errorf("Expected to not be paid have: %s ", postMintQuoteResponse.State)
	}

	if postMintQuoteResponse.Unit != "sat" {
		t.Errorf("Expected unit to be sat, got %v", postMintQuoteResponse.Unit)
	}

	w.Flush()

	// check quote request
	req = httptest.NewRequest("GET", "/v1/mint/quote/bolt11"+"/"+postMintQuoteResponse.Quote, strings.NewReader(string(jsonRequestBody)))

	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)
	var postMintQuoteResponseTwo cashu.MintRequestDB

	err = json.Unmarshal(w.Body.Bytes(), &postMintQuoteResponseTwo)

	if err != nil {
		t.Fatalf("Error unmarshalling response: %v", err)
	}

	if postMintQuoteResponseTwo.RequestPaid {
		t.Errorf("Expected paid to be false because it's a Lnd wallet and I have not paid the invoice yet, got %v", postMintQuoteResponseTwo.RequestPaid)
	}

	if postMintQuoteResponseTwo.State != cashu.PENDING {
		t.Errorf("Expected to not be unpaid have: %s ", postMintQuoteResponseTwo.State)
	}

	if postMintQuoteResponseTwo.Unit != "sat" {
		t.Errorf("Expected unit to be sat, got %v", postMintQuoteResponseTwo.Unit)
	}

	w.Flush()

	activeKeys, err := mint.Signer.GetActiveKeys()
	if err != nil {
		t.Fatalf("mint.Signer.GetKeysByUnit(cashu.Sat): %v", err)
	}

	// MINTING WITHOUT PAYING THE INVOICE
	beforeMintBlindedMessages, _, _, err := CreateBlindedMessages(1000, activeKeys)

	if err != nil {
		t.Fatalf("could not createBlind message: %v", err)
	}

	mintRequest := cashu.PostMintBolt11Request{
		Quote:   postMintQuoteResponse.Quote,
		Outputs: beforeMintBlindedMessages,
	}

	jsonRequestBody, _ = json.Marshal(mintRequest)

	req = httptest.NewRequest("POST", "/v1/mint/bolt11", strings.NewReader(string(jsonRequestBody)))

	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	var errorResponse cashu.ErrorResponse

	err = json.Unmarshal(w.Body.Bytes(), &errorResponse)

	if err != nil {
		t.Fatalf("Could not parse error response %s", w.Body.String())
	}

	if errorResponse.Code != 20001 {
		t.Errorf("Incorrect error code, got %v", errorResponse.Code)

	}
	if errorResponse.Error != "Quote request is not paid" {
		t.Errorf("Incorrect error string, got %s", errorResponse.Error)

	}

	// needs to wait a second for the containers to catch up
	time.Sleep(1000 * time.Millisecond)
	// Lnd BOB pays the invoice
	_, _, err = bobLnd.Exec(ctx, []string{"lncli", "--tlscertpath", "/home/lnd/.lnd/tls.cert", "--macaroonpath", "home/lnd/.lnd/data/chain/bitcoin/regtest/admin.macaroon", "payinvoice", postMintQuoteResponse.Request, "--force"})

	if err != nil {
		fmt.Errorf("Error paying invoice %+v", err)
	}

	// Minting with invalid signatures
	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	excesMintingBlindMessage, _, _, err := CreateBlindedMessages(1000, activeKeys)

	excesMintingBlindMessage[0].B_ = "badsig"

	excessMintRequest := cashu.PostMintBolt11Request{
		Quote:   postMintQuoteResponse.Quote,
		Outputs: excesMintingBlindMessage,
	}

	jsonRequestBody, _ = json.Marshal(excessMintRequest)

	req = httptest.NewRequest("POST", "/v1/mint/bolt11", strings.NewReader(string(jsonRequestBody)))

	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)
	errorResponse = cashu.ErrorResponse{}

	err = json.Unmarshal(w.Body.Bytes(), &errorResponse)

	if errorResponse.Code != cashu.TOKEN_NOT_VERIFIED {
		t.Errorf(`Expected code be Minting disables. Got:  %s`, errorResponse.Code)
	}
	if errorResponse.Error != "Proof could not be verified" {
		t.Errorf(`Expected code be Minting disables. Got:  %s`, errorResponse.Error)
	}

	// ASK FOR MINTING WITH TOO MANY BLINDED MESSAGES
	blindedMessages, _, _, err := CreateBlindedMessages(999999, activeKeys)
	if err != nil {
		t.Fatalf("could not createBlind message: %v", err)
	}

	mintRequestTooManyBlindMessages := cashu.PostMintBolt11Request{
		Quote:   postMintQuoteResponse.Quote,
		Outputs: blindedMessages,
	}

	jsonRequestBody, _ = json.Marshal(mintRequestTooManyBlindMessages)

	req = httptest.NewRequest("POST", "/v1/mint/bolt11", strings.NewReader(string(jsonRequestBody)))

	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Fatalf("Expected status code 200, got %d", w.Code)
	}

	if w.Body.String() != `"Amounts in outputs are not the same"` {
		t.Errorf("Expected Amounts in outputs are not the same, got %s", w.Body.String())
	}

	// MINT SUCCESSFULY
	blindedMessages, mintingSecrets, mintingSecretKeys, err := CreateBlindedMessages(1000, activeKeys)
	if err != nil {
		t.Fatalf("could not createBlind message: %v", err)
	}

	mintRequest = cashu.PostMintBolt11Request{
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

	var totalAmountSigned uint64 = 0

	for _, output := range postMintResponse.Signatures {
		totalAmountSigned += output.Amount
	}

	if totalAmountSigned != 1000 {
		t.Errorf("Expected total amount signed to be 1000, got %d", totalAmountSigned)
	}

	if postMintResponse.Signatures[0].Id != activeKeys.Keysets[0].Id {
		t.Errorf("Expected id to be %s, got %s", activeKeys.Keysets[0].Id, postMintResponse.Signatures[0].Id)
	}

	// lookup in the db if quote shows as issued
	req = httptest.NewRequest("GET", "/v1/mint/quote/bolt11"+"/"+postMintQuoteResponse.Quote, strings.NewReader(string(jsonRequestBody)))

	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)
	postMintQuoteResponseTwo = cashu.MintRequestDB{}

	err = json.Unmarshal(w.Body.Bytes(), &postMintQuoteResponseTwo)

	if err != nil {
		t.Fatalf("Error unmarshalling response: %v", err)
	}

	if postMintQuoteResponseTwo.State != cashu.ISSUED {
		t.Errorf("Expected state to be MINTED, got %v", postMintQuoteResponseTwo.State)
	}

	// try to remint tokens with other blinded signatures
	reMintBlindedMessages, _, _, err := CreateBlindedMessages(1000, activeKeys)
	if err != nil {
		t.Fatalf("could not createBlind message: %v", err)
	}

	reMintRequest := cashu.PostMintBolt11Request{
		Quote:   postMintQuoteResponse.Quote,
		Outputs: reMintBlindedMessages,
	}

	jsonRequestBody, _ = json.Marshal(reMintRequest)

	req = httptest.NewRequest("POST", "/v1/mint/bolt11", strings.NewReader(string(jsonRequestBody)))

	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	errorResponse = cashu.ErrorResponse{}

	err = json.Unmarshal(w.Body.Bytes(), &errorResponse)

	if err != nil {
		t.Fatalf("Could not parse error response %s", w.Body.String())
	}

	if errorResponse.Code != 20002 {
		t.Errorf("Incorrect error code, got %v", errorResponse.Code)

	}
	if errorResponse.Error != "Tokens have already been issued for quote" {
		t.Errorf("Incorrect error string, got %s", errorResponse.Error)

	}

	// MINTING TESTING ENDS

	// SWAP TESTING STARTS

	// TRY TO SWAP WITH TOO MANY BLINDED MESSAGES
	swapProofs, err := GenerateProofs(postMintResponse.Signatures, activeKeys, mintingSecrets, mintingSecretKeys)

	if err != nil {
		t.Fatalf("Error generating proofs: %v", err)
	}
	swapBlindedMessages, _, _, err := CreateBlindedMessages(1032843, activeKeys)
	if err != nil {
		t.Fatalf("could not createBlind message: %v", err)
	}

	swapRequestToManyBlindMessages := cashu.PostSwapRequest{
		Inputs:  swapProofs,
		Outputs: swapBlindedMessages,
	}

	jsonRequestBody, _ = json.Marshal(swapRequestToManyBlindMessages)

	req = httptest.NewRequest("POST", "/v1/swap", strings.NewReader(string(jsonRequestBody)))

	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	errorResponse = cashu.ErrorResponse{}

	err = json.Unmarshal(w.Body.Bytes(), &errorResponse)

	if err != nil {
		t.Fatalf("Could not parse error response %s", w.Body.String())
	}

	if errorResponse.Code != 11002 {
		t.Errorf("Incorrect error code, got %v", errorResponse.Code)

	}
	if errorResponse.Error != "Transaction is not balanced (inputs != outputs)" {
		t.Errorf("Incorrect error string, got %s", errorResponse.Error)

	}

	// try to swap tokens
	swapProofs, err = GenerateProofs(postMintResponse.Signatures, activeKeys, mintingSecrets, mintingSecretKeys)

	if err != nil {
		t.Fatalf("Error generating proofs: %v", err)
	}
	swapBlindedMessages, swapSecrets, swapPrivateKeySecrets, err := CreateBlindedMessages(1000, activeKeys)
	if err != nil {
		t.Fatalf("could not createBlind message: %v", err)
	}

	swapRequest := cashu.PostSwapRequest{
		Inputs:  swapProofs,
		Outputs: swapBlindedMessages,
	}

	jsonRequestBody, _ = json.Marshal(swapRequest)

	req = httptest.NewRequest("POST", "/v1/swap", strings.NewReader(string(jsonRequestBody)))

	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	var postSwapResponse cashu.PostSwapResponse

	if w.Code != 200 {
		t.Fatalf("Expected status code 200, got %d", w.Code)
	}

	err = json.Unmarshal(w.Body.Bytes(), &postSwapResponse)

	if err != nil {
		t.Fatalf("Error unmarshalling response: %v", err)
	}

	totalAmountSigned = 0

	for _, output := range postSwapResponse.Signatures {
		totalAmountSigned += output.Amount
	}

	if totalAmountSigned != 1000 {
		t.Errorf("Expected total amount signed to be 1000, got %d", totalAmountSigned)
	}

	if postSwapResponse.Signatures[0].Id != activeKeys.Keysets[0].Id {
		t.Errorf("Expected id to be %s, got %s", activeKeys.Keysets[0].Id, postSwapResponse.Signatures[0].Id)
	}

	w.Flush()

	// Swap with invalid Proofs
	invalidSignatureProofs, err := GenerateProofs(postSwapResponse.Signatures, activeKeys, swapSecrets, swapPrivateKeySecrets)
	if err != nil {
		t.Fatalf("Error generating proofs: %v", err)
	}
	swapInvalidSigBlindedMessages, _, _, err := CreateBlindedMessages(1000, activeKeys)
	if err != nil {
		t.Fatalf("could not createBlind message: %v", err)
	}

	invalidSignatureProofs[0].C = "badSig"
	invalidSignatureProofs[len(invalidSignatureProofs)-1].C = "badSig"

	invalidSwapRequest := cashu.PostSwapRequest{
		Inputs:  invalidSignatureProofs,
		Outputs: swapInvalidSigBlindedMessages,
	}

	jsonRequestBody, _ = json.Marshal(invalidSwapRequest)

	req = httptest.NewRequest("POST", "/v1/swap", strings.NewReader(string(jsonRequestBody)))

	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	errorResponse = cashu.ErrorResponse{}

	err = json.Unmarshal(w.Body.Bytes(), &errorResponse)

	if err != nil {
		t.Fatalf("Could not parse error response %s", w.Body.String())
	}

	if errorResponse.Code != 10003 {
		t.Errorf("Incorrect error code, got %v", errorResponse.Code)

	}
	if errorResponse.Error != "Proof could not be verified" {
		t.Errorf("Incorrect error string, got %s", errorResponse.Error)

	}

	w.Flush()

	// swap with  not enought proofs for compared to signatures
	proofsForRemoving, err := GenerateProofs(postSwapResponse.Signatures, activeKeys, swapSecrets, swapPrivateKeySecrets)
	if err != nil {
		t.Fatalf("Error generating proofs: %v", err)
	}
	signaturesForRemoving, _, _, err := CreateBlindedMessages(1000, activeKeys)
	if err != nil {
		t.Fatalf("could not createBlind message: %v", err)
	}

	notEnoughtProofsSwapRequest := cashu.PostSwapRequest{
		Inputs:  proofsForRemoving[:len(proofsForRemoving)-2],
		Outputs: signaturesForRemoving,
	}

	jsonRequestBody, _ = json.Marshal(notEnoughtProofsSwapRequest)

	req = httptest.NewRequest("POST", "/v1/swap", strings.NewReader(string(jsonRequestBody)))

	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	errorResponse = cashu.ErrorResponse{}

	err = json.Unmarshal(w.Body.Bytes(), &errorResponse)

	if err != nil {
		t.Fatalf("Could not parse error response %s", w.Body.String())
	}

	if errorResponse.Code != 11002 {
		t.Errorf("Incorrect error code, got %v", errorResponse.Code)

	}
	if errorResponse.Error != "Transaction is not balanced (inputs != outputs)" {
		t.Errorf("Incorrect error string, got %s", errorResponse.Error)

	}

	w.Flush()

	// SWAP TESTING ENDS

	// MELTING TESTING STARTS
	_, invoiceReader, err := bobLnd.Exec(ctx, []string{"lncli", "--tlscertpath", "/home/lnd/.lnd/tls.cert", "--macaroonpath", "home/lnd/.lnd/data/chain/bitcoin/regtest/admin.macaroon", "addinvoice", "--amt", "900"})

	if err != nil {
		t.Fatalf("Error adding invoice: %+v", err)
	}

	// I have to grab the Payment request from the cli reader
	reader := io.Reader(invoiceReader)
	buf := make([]byte, 3024)
	type LncliInvoice struct {
		PaymentRequest string `json:"payment_request"`
	}

	var invoice LncliInvoice
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			index := strings.Index(string(buf[:n]), "{")
			err := json.Unmarshal(buf[index:n], &invoice)
			if err != nil {
				t.Fatal("json.Unmarshal: ", err)
			}
		}
		if err != nil {
			break
		}
	}

	// test melt tokens
	meltQuoteRequest := cashu.PostMeltQuoteBolt11Request{
		Unit:    cashu.Sat.String(),
		Request: invoice.PaymentRequest,
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

	if postMeltQuoteResponse.Paid {
		t.Errorf("Expected paid to be false because it's a LND Node, got %v", postMeltQuoteResponse.Paid)
	}

	if postMeltQuoteResponse.State != cashu.UNPAID {
		t.Errorf("Expected to not be paid have: %s ", postMeltQuoteResponse.State)
	}

	if postMeltQuoteResponse.Amount != 900 {
		t.Errorf("Expected amount to be 900, got %d", postMeltQuoteResponse.Amount)
	}

	// test melt tokens quote call
	req = httptest.NewRequest("GET", "/v1/melt/quote/bolt11"+"/"+postMeltQuoteResponse.Quote, nil)

	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var postMeltQuoteResponseTwo cashu.PostMeltQuoteBolt11Response
	err = json.Unmarshal(w.Body.Bytes(), &postMeltQuoteResponseTwo)

	if err != nil {
		t.Fatalf("Error unmarshalling response: %v", err)
	}

	if postMeltQuoteResponse.Paid {
		t.Errorf("Expected paid to be false because it's a Lnd Node, got %v", postMeltQuoteResponse.Paid)
	}
	if postMeltQuoteResponse.State != cashu.UNPAID {

		t.Errorf("Expected to not be paid have: %s ", postMintQuoteResponseTwo.State)
	}

	if postMeltQuoteResponse.Amount != 900 {
		t.Errorf("Expected amount to be 900, got %d", postMeltQuoteResponse.Amount)
	}

	// test melt with invalid proofs
	meltProofs, err := GenerateProofs(postSwapResponse.Signatures, activeKeys, swapSecrets, swapPrivateKeySecrets)

	if err != nil {
		t.Fatalf("Error generating proofs: %v", err)
	}

	InvalidProofsMeltRequest := cashu.PostMeltBolt11Request{
		Quote:  postMeltQuoteResponse.Quote,
		Inputs: meltProofs,
	}

	InvalidProofsMeltRequest.Inputs[0].C = "badSig"

	jsonRequestBody, _ = json.Marshal(InvalidProofsMeltRequest)

	req = httptest.NewRequest("POST", "/v1/melt/bolt11", strings.NewReader(string(jsonRequestBody)))
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("Expected status code 403, got %d", w.Code)
	}

	var errorRes cashu.ErrorResponse
	err = json.Unmarshal(w.Body.Bytes(), &errorRes)
	if err != nil {
		t.Fatalf("json.Unmarshal(w.Body.Bytes(), &errorRes): %v", err)
	}

	if errorRes.Code != cashu.TOKEN_NOT_VERIFIED {
		t.Errorf("Expected Invalid Proof, got %s", w.Body.String())
	}

	w.Flush()

	meltProofs, err = GenerateProofs(postSwapResponse.Signatures, activeKeys, swapSecrets, swapPrivateKeySecrets)

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

	if postMeltResponse.State != cashu.PAID {
		t.Errorf("Expected state to be PAID, got %v", postMintQuoteResponseTwo.State)
	}

	if !postMeltResponse.Paid {
		t.Errorf("Expected paid to be true because it's a fake wallet, got %v", postMeltResponse.Paid)
	}

	// Test melt that has already been melted

	req = httptest.NewRequest("POST", "/v1/melt/bolt11", strings.NewReader(string(jsonRequestBody)))
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	errorResponse = cashu.ErrorResponse{}

	err = json.Unmarshal(w.Body.Bytes(), &errorResponse)

	if err != nil {
		t.Fatalf("Could not parse error response %s", w.Body.String())
	}

	if errorResponse.Code != 20006 {
		t.Errorf("Incorrect error code, got %v", errorResponse.Code)

	}
	if errorResponse.Error != "Invoice already paid" {
		t.Errorf("Incorrect error string, got %s", errorResponse.Error)

	}

	// MELTING TESTING ENDS
}

func TestWrongUnitOnMeltAndMint(t *testing.T) {
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

	t.Setenv("DATABASE_URL", connUri)
	t.Setenv("MINT_PRIVATE_KEY", MintPrivateKey)
	t.Setenv("MINT_LIGHTNING_BACKEND", "FakeWallet")
	t.Setenv(mint.NETWORK_ENV, "regtest")

	ctx = context.WithValue(ctx, mint.NETWORK_ENV, os.Getenv(mint.NETWORK_ENV))
	ctx = context.WithValue(ctx, mint.MINT_LIGHTNING_BACKEND_ENV, os.Getenv(mint.MINT_LIGHTNING_BACKEND_ENV))
	ctx = context.WithValue(ctx, database.DATABASE_URL_ENV, os.Getenv(database.DATABASE_URL_ENV))
	ctx = context.WithValue(ctx, mint.NETWORK_ENV, os.Getenv(mint.NETWORK_ENV))

	router, _ := SetupRoutingForTesting(ctx, false)

	// Mint check incorrect unit
	w := httptest.NewRecorder()

	mintQuoteRequest := cashu.PostMintQuoteBolt11Request{
		Amount: 10000,
		Unit:   "Milsat",
	}
	jsonRequestBody, _ := json.Marshal(mintQuoteRequest)

	req := httptest.NewRequest("POST", "/v1/mint/quote/bolt11", strings.NewReader(string(jsonRequestBody)))

	router.ServeHTTP(w, req)

	errorResponse := cashu.ErrorResponse{}

	err = json.Unmarshal(w.Body.Bytes(), &errorResponse)

	if err != nil {
		t.Fatalf("Could not parse error response %s", w.Body.String())
	}

	if errorResponse.Code != 11005 {
		t.Errorf("Incorrect error code, got %v", errorResponse.Code)

	}
	if errorResponse.Error != "Unit in request is not supported" {
		t.Errorf("Incorrect error string, got %s", errorResponse.Error)

	}

	// melt quote with incorrect unit
	w = httptest.NewRecorder()
	meltQuoteRequest := cashu.PostMeltQuoteBolt11Request{
		Request: "dummyrequest",
		Unit:    "Milsat",
	}
	jsonRequestBody, _ = json.Marshal(meltQuoteRequest)

	req = httptest.NewRequest("POST", "/v1/melt/quote/bolt11", strings.NewReader(string(jsonRequestBody)))

	router.ServeHTTP(w, req)
	errorResponse = cashu.ErrorResponse{}

	err = json.Unmarshal(w.Body.Bytes(), &errorResponse)

	if err != nil {
		t.Fatalf("Could not parse error response %s", w.Body.String())
	}

	if errorResponse.Code != 11005 {
		t.Errorf("Incorrect error code, got %v", errorResponse.Code)

	}
	if errorResponse.Error != "Unit in request is not supported" {
		t.Errorf("Incorrect error string, got %s", errorResponse.Error)

	}
}

func TestConfigMeltMintLimit(t *testing.T) {

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

	t.Setenv("DATABASE_URL", connUri)
	t.Setenv("MINT_PRIVATE_KEY", MintPrivateKey)
	t.Setenv("MINT_LIGHTNING_BACKEND", "FakeWallet")
	t.Setenv(mint.NETWORK_ENV, "regtest")

	ctx = context.WithValue(ctx, mint.NETWORK_ENV, os.Getenv(mint.NETWORK_ENV))
	ctx = context.WithValue(ctx, mint.MINT_LIGHTNING_BACKEND_ENV, os.Getenv(mint.MINT_LIGHTNING_BACKEND_ENV))
	ctx = context.WithValue(ctx, database.DATABASE_URL_ENV, os.Getenv(database.DATABASE_URL_ENV))
	ctx = context.WithValue(ctx, mint.NETWORK_ENV, os.Getenv(mint.NETWORK_ENV))

	router, mint := SetupRoutingForTesting(ctx, false)

	// MINTING TESTING STARTS

	// TEST MINT CONFIG LIMIT
	w := httptest.NewRecorder()

	mintQuoteRequest := cashu.PostMintQuoteBolt11Request{
		Amount: 1000,
		Unit:   cashu.Sat.String(),
	}
	jsonRequestBody, _ := json.Marshal(mintQuoteRequest)

	req := httptest.NewRequest("POST", "/v1/mint/quote/bolt11", strings.NewReader(string(jsonRequestBody)))

	limit := 999
	mint.Config.PEG_IN_LIMIT_SATS = &limit

	router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("Expected status code 200, got %d", w.Code)
	}
	if w.Body.String() != `"Mint amount over the limit"` {
		t.Errorf(`Expected body message to be: "Mint amount over the limit". Got:  %s`, w.Body.String())
	}

	w = httptest.NewRecorder()

	// Test mint ONLY PEGOUT check

	mint.Config.PEG_OUT_ONLY = true
	req = httptest.NewRequest("POST", "/v1/mint/quote/bolt11", strings.NewReader(string(jsonRequestBody)))
	router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("Expected status code 200, got %d", w.Code)
	}
	errorResponse := cashu.ErrorResponse{}

	err = json.Unmarshal(w.Body.Bytes(), &errorResponse)

	if errorResponse.Code != cashu.MINTING_DISABLED {
		t.Errorf(`Expected code be Minting disables. Got:  %s`, errorResponse.Code)
	}
	if errorResponse.Error != "Minting is disabled" {
		t.Errorf(`Expected code be Minting disables. Got:  %s`, errorResponse.Error)
	}

}
func TestFeeReturnAmount(t *testing.T) {
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

	t.Setenv("DATABASE_URL", connUri)
	t.Setenv("MINT_PRIVATE_KEY", MintPrivateKey)
	t.Setenv("MINT_LIGHTNING_BACKEND", "FakeWallet")
	t.Setenv(mint.NETWORK_ENV, "regtest")

	ctx = context.WithValue(ctx, mint.NETWORK_ENV, os.Getenv(mint.NETWORK_ENV))
	ctx = context.WithValue(ctx, mint.MINT_LIGHTNING_BACKEND_ENV, os.Getenv(mint.MINT_LIGHTNING_BACKEND_ENV))
	ctx = context.WithValue(ctx, database.DATABASE_URL_ENV, os.Getenv(database.DATABASE_URL_ENV))
	ctx = context.WithValue(ctx, mint.NETWORK_ENV, os.Getenv(mint.NETWORK_ENV))

	router, mint := SetupRoutingForTesting(ctx, false)

	// Mint check incorrect unit
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

	var postMintQuoteResponse cashu.PostMintQuoteBolt11Response
	err = json.Unmarshal(w.Body.Bytes(), &postMintQuoteResponse)

	if err != nil {
		t.Errorf("Error unmarshalling response: %v", err)
	}

	activeKeys, err := mint.Signer.GetActiveKeys()
	if err != nil {
		t.Fatalf("mint.Signer.GetKeysByUnit(cashu.Sat): %v", err)
	}

	// mint cashu tokens
	blindedMessages, mintingSecrets, mintingSecretKeys, err := CreateBlindedMessages(10000, activeKeys)
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

	// request melt quote for 1000 sats
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

	w.Flush()

	// test melt tokens
	meltProofs, err := GenerateProofs(postMintResponse.Signatures, activeKeys, mintingSecrets, mintingSecretKeys)

	// mint cashu tokens
	changeBlindedMessages, _, _, err := CreateBlindedMessages(10000, activeKeys)
	if err != nil {
		t.Errorf("Error CreateBlindedMessages(10000, activeKeys): %v", err)
	}

	meltRequest := cashu.PostMeltBolt11Request{
		Quote:   postMeltQuoteResponse.Quote,
		Inputs:  meltProofs,
		Outputs: changeBlindedMessages,
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

	changeAmount := uint64(0)
	for _, sig := range postMeltResponse.Change {
		changeAmount += sig.Amount
	}

	if changeAmount != 9000 {
		t.Errorf("Change amount is incorrect %v", changeAmount)

	}
}
