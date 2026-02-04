package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/signer"
	"github.com/lescuer97/nutmix/pkg/crypto"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	correctPreimage   = "0000000000000000000000000000000000000000000000000000000000000001"
	incorrectPreimage = "0000000000000000000000000000000000000000000000000000000000000002"
)

func TestRoutesHTLCSwapMelt(t *testing.T) {
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

	router, mint := SetupRoutingForTesting(ctx, false)

	lockingPrivKey := secp256k1.PrivKeyFromBytes([]byte{0x01, 0x02, 0x03, 0x04})

	wrongPrivKey := secp256k1.PrivKeyFromBytes([]byte{0x01, 0x02, 0x03, 0x05})

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
	err = json.Unmarshal(w.Body.Bytes(), &postMintQuoteResponse)

	if err != nil {
		t.Errorf("Error unmarshalling response: %v", err)
	}

	activeKeys, err := mint.Signer.GetActiveKeys()
	if err != nil {
		t.Fatalf("mint.Signer.GetKeysByUnit(cashu.Sat): %v", err)
	}

	// ask for minting
	htlcBlindedMessages, htlcMintingSecrets, HTLCMintingSecretKeys, err := CreateHTLCBlindedMessages(1000, activeKeys, correctPreimage, 1, []*secp256k1.PublicKey{lockingPrivKey.PubKey()}, nil, 0, cashu.SigInputs)

	if err != nil {
		t.Fatalf("could not createBlind message: %v", err)
	}

	mintRequest := cashu.PostMintBolt11Request{
		Quote:   postMintQuoteResponse.Quote,
		Outputs: htlcBlindedMessages,
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

	// activeKeys

	// SWAP HTLC TOKEN with other HTLC TOKENS
	swapProofs, err := GenerateProofsHTLC(postMintResponse.Signatures, correctPreimage, activeKeys, htlcMintingSecrets, HTLCMintingSecretKeys, []*secp256k1.PrivateKey{lockingPrivKey})

	if err != nil {
		t.Fatalf("Error generating proofs: %v", err)
	}

	swapBlindedMessagesHTLC, swapSecretsHTLC, swapSecretKeyHTLC, err := CreateHTLCBlindedMessages(1000, activeKeys, correctPreimage, 1, []*secp256k1.PublicKey{lockingPrivKey.PubKey()}, nil, 0, cashu.SigInputs)

	if err != nil {
		t.Fatalf("could not createBlind message: %v", err)
	}

	swapRequest := cashu.PostSwapRequest{
		Inputs:  swapProofs,
		Outputs: swapBlindedMessagesHTLC,
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

	// TRY SWAPING with WRONG SIGNATURES
	swapProofsWrongSigs, err := GenerateProofsHTLC(postSwapResponse.Signatures, correctPreimage, activeKeys, swapSecretsHTLC, swapSecretKeyHTLC, []*secp256k1.PrivateKey{wrongPrivKey})

	if err != nil {
		t.Fatalf("Error generating proofs: %v", err)
	}

	swapBlindedMessagesHTLCWrongSigs, _, _, err := CreateHTLCBlindedMessages(1000, activeKeys, correctPreimage, 1, []*secp256k1.PublicKey{lockingPrivKey.PubKey()}, nil, 0, cashu.SigInputs)

	if err != nil {
		t.Fatalf("could not createBlind message: %v", err)
	}

	swapRequestTwo := cashu.PostSwapRequest{
		Inputs:  swapProofsWrongSigs,
		Outputs: swapBlindedMessagesHTLCWrongSigs,
	}

	jsonRequestBody, _ = json.Marshal(swapRequestTwo)

	req = httptest.NewRequest("POST", "/v1/swap", strings.NewReader(string(jsonRequestBody)))

	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("Expected status code 400, got %d", w.Code)
	}

	var errorResponse cashu.ErrorResponse

	err = json.Unmarshal(w.Body.Bytes(), &errorResponse)

	if err != nil {
		t.Fatalf("Could not parse error response %s", w.Body.String())
	}

	if errorResponse.Code != cashu.PROOF_VERIFICATION_FAILED {
		t.Errorf("Incorrect error code, got %v", errorResponse.Code)

	}
	if errorResponse.Error != "Proof could not be verified" {
		t.Errorf("Incorrect error string, got %s", errorResponse.Error)

	}

	// TRY SWAPING with WRONG Preimage
	swapProofsWrongSigs, err = GenerateProofsHTLC(postSwapResponse.Signatures, incorrectPreimage, activeKeys, swapSecretsHTLC, swapSecretKeyHTLC, []*secp256k1.PrivateKey{wrongPrivKey})

	if err != nil {
		t.Fatalf("Error generating proofs: %v", err)
	}

	swapBlindedMessagesHTLCWrongSigs, _, _, err = CreateHTLCBlindedMessages(1000, activeKeys, correctPreimage, 1, []*secp256k1.PublicKey{lockingPrivKey.PubKey()}, nil, 0, cashu.SigInputs)

	if err != nil {
		t.Fatalf("could not createBlind message: %v", err)
	}

	swapRequestTwo = cashu.PostSwapRequest{
		Inputs:  swapProofsWrongSigs,
		Outputs: swapBlindedMessagesHTLCWrongSigs,
	}

	jsonRequestBody, _ = json.Marshal(swapRequestTwo)

	req = httptest.NewRequest("POST", "/v1/swap", strings.NewReader(string(jsonRequestBody)))

	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("Expected status code 400, got %d", w.Code)
	}

	var errorRes cashu.ErrorResponse
	err = json.Unmarshal(w.Body.Bytes(), &errorRes)
	if err != nil {
		t.Fatalf("json.Unmarshal(w.Body.Bytes(), &errorRes): %v", err)
	}

	if errorRes.Code != cashu.PROOF_VERIFICATION_FAILED {
		t.Errorf("Expected Invalid Proof, got %s", w.Body.String())
	}

	if *errorRes.Detail != `invalid preimage` {
		t.Fatalf("Expected response Invalid preimage, got %s", w.Body.String())
	}

}

func CreateHTLCBlindedMessages(amount uint64, keyset signer.GetKeysResponse, preimage string, nSigs uint, pubkeys []*secp256k1.PublicKey, refundPubkey []*secp256k1.PublicKey, locktime uint, sigflag cashu.SigFlag) ([]cashu.BlindedMessage, []string, []*secp256k1.PrivateKey, error) {
	splitAmounts := cashu.AmountSplit(amount)
	splitLen := len(splitAmounts)

	blindedMessages := make([]cashu.BlindedMessage, splitLen)
	secrets := make([]string, splitLen)
	rs := make([]*secp256k1.PrivateKey, splitLen)

	for i, amt := range splitAmounts {
		spendCond, err := makeHTLCSpendCondition(preimage, nSigs, pubkeys, refundPubkey, locktime, sigflag)

		if err != nil {
			return nil, nil, nil, fmt.Errorf("MakeHTLCSpendCondition: %w", err)
		}

		jsonSpend, err := spendCond.String()

		if err != nil {
			return nil, nil, nil, fmt.Errorf("json.Marshal(spendCond): %w", err)
		}

		// generate new private key r
		r, err := secp256k1.GeneratePrivateKey()
		if err != nil {
			return nil, nil, nil, err
		}

		var B_ *secp256k1.PublicKey
		var secret = jsonSpend
		// generate random secret until it finds valid point
		for {
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

func makeHTLCSpendCondition(preimage string, nSigs uint, pubkeys []*secp256k1.PublicKey, refundPubkey []*secp256k1.PublicKey, locktime uint, sigflag cashu.SigFlag) (cashu.SpendCondition, error) {

	bytesPreimage, err := hex.DecodeString(preimage)
	if err != nil {
		return cashu.SpendCondition{}, err
	}

	parsedPreimage := sha256.Sum256(bytesPreimage)

	var spendCondition cashu.SpendCondition
	spendCondition.Type = cashu.HTLC
	spendCondition.Data.Data = hex.EncodeToString(parsedPreimage[:])
	spendCondition.Data.Tags.Pubkeys = pubkeys
	spendCondition.Data.Tags.NSigs = nSigs
	spendCondition.Data.Tags.Locktime = locktime
	spendCondition.Data.Tags.Sigflag = sigflag
	spendCondition.Data.Tags.Refund = refundPubkey

	// generate random Nonce
	nonce := make([]byte, 32) // create a slice with length 16 for the nonce
	_, err = rand.Read(nonce) // read random bytes into the nonce slice
	if err != nil {
		return spendCondition, err
	}
	spendCondition.Data.Nonce = hex.EncodeToString(nonce)

	return spendCondition, nil
}

func GenerateProofsHTLC(signatures []cashu.BlindSignature, preimage string, keyset signer.GetKeysResponse, secrets []string, secretsKey []*secp256k1.PrivateKey, privkeys []*secp256k1.PrivateKey) ([]cashu.Proof, error) {
	// try to swap tokens
	var proofs []cashu.Proof
	// unblid the signatures and make proofs
	for i, output := range signatures {

		pubkeyStr := keyset.Keysets[0].Keys[output.Amount]
		pubkeyBytes, err := hex.DecodeString(pubkeyStr)
		if err != nil {
			return nil, fmt.Errorf("hex.DecodeString(pubkeyStr): %w", err)
		}

		mintPublicKey, err := secp256k1.ParsePubKey(pubkeyBytes)
		if err != nil {
			return nil, fmt.Errorf("Error parsing pubkey: %w", err)
		}

		C := crypto.UnblindSignature(output.C_.PublicKey, secretsKey[i], mintPublicKey)

		proof := cashu.Proof{Id: output.Id, Amount: output.Amount, C: cashu.WrappedPublicKey{PublicKey: C}, Secret: secrets[i]}

		for _, privkey := range privkeys {
			err = proof.Sign(privkey)
			if err != nil {
				return nil, fmt.Errorf("Error signing proof: %w", err)
			}
			err = proof.AddPreimage(preimage)
			if err != nil {
				return nil, fmt.Errorf("Error signing proof: %w", err)
			}
		}

		if err != nil {
			return nil, fmt.Errorf("Error signing proof: %w", err)
		}

		proofs = append(proofs, proof)
	}

	return proofs, nil
}

func TestHTLCMultisigSigning(t *testing.T) {
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

	t.Setenv(database.DATABASE_URL_ENV, connUri)
	t.Setenv(MINT_PRIVATE_KEY_ENV, MintPrivateKey)
	t.Setenv(mint.MINT_LIGHTNING_BACKEND_ENV, "FakeWallet")
	t.Setenv(mint.NETWORK_ENV, "regtest")

	ctx = context.WithValue(ctx, ctxKeyNetwork, os.Getenv(mint.NETWORK_ENV))
	ctx = context.WithValue(ctx, ctxKeyLightningBackend, os.Getenv(mint.MINT_LIGHTNING_BACKEND_ENV))
	ctx = context.WithValue(ctx, ctxKeyDatabaseURL, os.Getenv(database.DATABASE_URL_ENV))
	ctx = context.WithValue(ctx, ctxKeyNetwork, os.Getenv(mint.NETWORK_ENV))

	router, mint := SetupRoutingForTesting(ctx, false)

	lockingPrivKeyOne := secp256k1.PrivKeyFromBytes([]byte{0x01, 0x02, 0x03, 0x04})

	lockingPrivKeyTwo := secp256k1.PrivKeyFromBytes([]byte{0x01, 0x02, 0x03, 0x05})

	wrongPrivKey := secp256k1.PrivKeyFromBytes([]byte{0x01, 0x02, 0x03, 0x08})

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
	err = json.Unmarshal(w.Body.Bytes(), &postMintQuoteResponse)

	if err != nil {
		t.Errorf("Error unmarshalling response: %v", err)
	}
	activeKeys, err := mint.Signer.GetActiveKeys()
	if err != nil {
		t.Fatalf("mint.Signer.GetKeysByUnit(cashu.Sat): %v", err)
	}

	// ask for minting
	// Create multisig token for 2 pubkeys
	htlcBlindedMessages, htlcMintingSecrets, HTLCMintingSecretKeys, err := CreateHTLCBlindedMessages(1000, activeKeys, correctPreimage, 2, []*secp256k1.PublicKey{lockingPrivKeyOne.PubKey(), lockingPrivKeyTwo.PubKey()}, nil, 0, cashu.SigInputs)

	if err != nil {
		t.Fatalf("could not createBlind message: %v", err)
	}

	mintRequest := cashu.PostMintBolt11Request{
		Quote:   postMintQuoteResponse.Quote,
		Outputs: htlcBlindedMessages,
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

	// SWAP HTLC TOKEN with other HTLC TOKENS
	// sign multisig with correct privkeys
	swapProofs, err := GenerateProofsHTLC(postMintResponse.Signatures, correctPreimage, activeKeys, htlcMintingSecrets, HTLCMintingSecretKeys, []*secp256k1.PrivateKey{lockingPrivKeyOne, lockingPrivKeyTwo})

	if err != nil {
		t.Fatalf("Error generating proofs: %v", err)
	}

	refundPrivKey := secp256k1.PrivKeyFromBytes([]byte{0x01, 0x02, 0x03, 0x06})

	swapBlindedMessagesHTLC, swapSecretsHTLC, swapSecretKeyHTLC, err := CreateHTLCBlindedMessages(1000, activeKeys, correctPreimage, 2, []*secp256k1.PublicKey{lockingPrivKeyTwo.PubKey(), lockingPrivKeyOne.PubKey(), lockingPrivKeyTwo.PubKey()}, []*secp256k1.PublicKey{refundPrivKey.PubKey()}, 100, cashu.SigInputs)

	if err != nil {
		t.Fatalf("could not createBlind message: %v", err)
	}

	swapRequest := cashu.PostSwapRequest{
		Inputs:  swapProofs,
		Outputs: swapBlindedMessagesHTLC,
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

	// // TRY SWAPING with Timelock passed
	swapProofsTimelockNotExpiredWrongSig, err := GenerateProofsHTLC(postSwapResponse.Signatures, correctPreimage, activeKeys, swapSecretsHTLC, swapSecretKeyHTLC, []*secp256k1.PrivateKey{lockingPrivKeyTwo, wrongPrivKey})

	if err != nil {
		t.Fatalf("Error generating proofs: %v", err)
	}

	swapBlindedMessagesHTLCWrongPreimage, _, _, err := CreateHTLCBlindedMessages(1000, activeKeys, correctPreimage, 2, []*secp256k1.PublicKey{lockingPrivKeyTwo.PubKey()}, []*secp256k1.PublicKey{refundPrivKey.PubKey()}, 100, cashu.SigInputs)

	if err != nil {
		t.Fatalf("could not createBlind message: %v", err)
	}

	swapRequestTwo := cashu.PostSwapRequest{
		Inputs:  swapProofsTimelockNotExpiredWrongSig,
		Outputs: swapBlindedMessagesHTLCWrongPreimage,
	}

	jsonRequestBody, _ = json.Marshal(swapRequestTwo)

	req = httptest.NewRequest("POST", "/v1/swap", strings.NewReader(string(jsonRequestBody)))

	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("Expected status code 400, got %d", w.Code)
	}

	var errorRes cashu.ErrorResponse
	err = json.Unmarshal(w.Body.Bytes(), &errorRes)
	if err != nil {
		t.Fatalf("json.Unmarshal(w.Body.Bytes(), &errorRes): %v", err)
	}

	if errorRes.Code != cashu.PROOF_VERIFICATION_FAILED {
		t.Errorf("Expected Invalid Proof, got %s", w.Body.String())
	}

	// if *errorRes.Detail != `Locktime has passed and no refund key was found` {
	// 	t.Fatalf("Expected response No valid signatures, got %s", w.Body.String())
	// }

	// TRY SWAPPING with refund key
	swapProofsRefund, err := GenerateProofsHTLC(postSwapResponse.Signatures, correctPreimage, activeKeys, swapSecretsHTLC, swapSecretKeyHTLC, []*secp256k1.PrivateKey{lockingPrivKeyTwo, refundPrivKey})
	if err != nil {
		t.Fatalf("Error generating refund proofs: %v", err)
	}

	currentPlus15 := time.Now().Add(15 * time.Minute).Unix()

	// generate new blind signatures with timelock over 15 minutes of current time
	swapBlindedMessagesHTLCWrongSigsOverlock, swapSecretsHTLC, swapSecretKeyHTLC, err := CreateHTLCBlindedMessages(1000, activeKeys, correctPreimage, 2, []*secp256k1.PublicKey{lockingPrivKeyOne.PubKey(), lockingPrivKeyTwo.PubKey()}, []*secp256k1.PublicKey{refundPrivKey.PubKey()}, uint(currentPlus15), cashu.SigInputs)

	if err != nil {
		t.Fatalf("Error generating proofs: %v", err)
	}
	swapRequestRefund := cashu.PostSwapRequest{
		Inputs:  swapProofsRefund,
		Outputs: swapBlindedMessagesHTLCWrongSigsOverlock,
	}

	jsonRequestBody, _ = json.Marshal(swapRequestRefund)

	req = httptest.NewRequest("POST", "/v1/swap", strings.NewReader(string(jsonRequestBody)))

	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("Expected status code 200, got %d", w.Code)
	}

	err = json.Unmarshal(w.Body.Bytes(), &postSwapResponse)

	if err != nil {
		t.Fatalf("Error unmarshalling response: %v", err)
	}

	// try swapping with wrong refund key and timelock not yet expired

	swapProofsTimelockNotExpiredWrongSig, err = GenerateProofsHTLC(postSwapResponse.Signatures, correctPreimage, activeKeys, swapSecretsHTLC, swapSecretKeyHTLC, []*secp256k1.PrivateKey{lockingPrivKeyTwo, wrongPrivKey})

	if err != nil {
		t.Fatalf("Error generating proofs: %v", err)
	}

	swapBlindedMessagesHTLCWrongPreimage, _, _, err = CreateHTLCBlindedMessages(1000, activeKeys, correctPreimage, 2, []*secp256k1.PublicKey{lockingPrivKeyTwo.PubKey()}, []*secp256k1.PublicKey{refundPrivKey.PubKey()}, 100, cashu.SigInputs)

	if err != nil {
		t.Fatalf("could not createBlind message: %v", err)
	}

	swapRequestTwo = cashu.PostSwapRequest{
		Inputs:  swapProofsTimelockNotExpiredWrongSig,
		Outputs: swapBlindedMessagesHTLCWrongPreimage,
	}

	jsonRequestBody, _ = json.Marshal(swapRequestTwo)

	req = httptest.NewRequest("POST", "/v1/swap", strings.NewReader(string(jsonRequestBody)))

	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)
	var errorResponse cashu.ErrorResponse

	err = json.Unmarshal(w.Body.Bytes(), &errorResponse)

	if err != nil {
		t.Fatalf("Could not parse error response %s", w.Body.String())
	}

	if errorResponse.Code != cashu.PROOF_VERIFICATION_FAILED {
		t.Errorf("Incorrect error code, got %v", errorResponse.Code)

	}
	if errorResponse.Error != "Proof could not be verified" {
		t.Errorf("Incorrect error string, got %s", errorResponse.Error)

	}

	// Try swapping with not enough signatures
	swapProofsTimelockNotExpiredWrongSig, err = GenerateProofsHTLC(postSwapResponse.Signatures, correctPreimage, activeKeys, swapSecretsHTLC, swapSecretKeyHTLC, []*secp256k1.PrivateKey{lockingPrivKeyTwo})

	if err != nil {
		t.Fatalf("Error generating proofs: %v", err)
	}

	swapBlindedMessagesHTLCWrongPreimage, _, _, err = CreateHTLCBlindedMessages(1000, activeKeys, correctPreimage, 2, []*secp256k1.PublicKey{lockingPrivKeyTwo.PubKey()}, []*secp256k1.PublicKey{refundPrivKey.PubKey()}, 100, cashu.SigInputs)

	if err != nil {
		t.Fatalf("could not createBlind message: %v", err)
	}

	swapRequestTwo = cashu.PostSwapRequest{
		Inputs:  swapProofsTimelockNotExpiredWrongSig,
		Outputs: swapBlindedMessagesHTLCWrongPreimage,
	}

	jsonRequestBody, _ = json.Marshal(swapRequestTwo)

	req = httptest.NewRequest("POST", "/v1/swap", strings.NewReader(string(jsonRequestBody)))

	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)
	errorResponse = cashu.ErrorResponse{}

	err = json.Unmarshal(w.Body.Bytes(), &errorResponse)

	if err != nil {
		t.Fatalf("Could not parse error response %s", w.Body.String())
	}

	if errorResponse.Code != cashu.PROOF_VERIFICATION_FAILED {
		t.Errorf("Incorrect error code, got %v", errorResponse.Code)

	}
	if errorResponse.Error != "Proof could not be verified" {
		t.Errorf("Incorrect error string, got %s", errorResponse.Error)

	}

	// Try swapping with correct signatures but wrong preimage
	swapProofsTimelockNotExpiredWrongSig, err = GenerateProofsHTLC(postSwapResponse.Signatures, incorrectPreimage, activeKeys, swapSecretsHTLC, swapSecretKeyHTLC, []*secp256k1.PrivateKey{lockingPrivKeyOne, lockingPrivKeyTwo})

	if err != nil {
		t.Fatalf("Error generating proofs: %v", err)
	}

	swapBlindedMessagesHTLCWrongPreimage, _, _, err = CreateHTLCBlindedMessages(1000, activeKeys, correctPreimage, 2, []*secp256k1.PublicKey{lockingPrivKeyOne.PubKey(), lockingPrivKeyTwo.PubKey()}, []*secp256k1.PublicKey{refundPrivKey.PubKey()}, 100, cashu.SigInputs)

	if err != nil {
		t.Fatalf("could not createBlind message: %v", err)
	}

	swapRequestTwo = cashu.PostSwapRequest{
		Inputs:  swapProofsTimelockNotExpiredWrongSig,
		Outputs: swapBlindedMessagesHTLCWrongPreimage,
	}

	jsonRequestBody, _ = json.Marshal(swapRequestTwo)

	req = httptest.NewRequest("POST", "/v1/swap", strings.NewReader(string(jsonRequestBody)))

	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("Expected status code 400, got %d", w.Code)
	}

	err = json.Unmarshal(w.Body.Bytes(), &errorRes)
	if err != nil {
		t.Fatalf("json.Unmarshal(w.Body.Bytes(), &errorRes): %v", err)
	}

	if errorRes.Code != cashu.PROOF_VERIFICATION_FAILED {
		t.Errorf("Expected Invalid Proof, got %s", w.Body.String())
	}

	if *errorRes.Detail != `invalid preimage` {
		t.Fatalf("Expected response Invalid preimage, got %s", w.Body.String())
	}

	// Try swapping with correct signatures and correct preimage
	swapProofsTimelockNotExpiredWrongSig, err = GenerateProofsHTLC(postSwapResponse.Signatures, correctPreimage, activeKeys, swapSecretsHTLC, swapSecretKeyHTLC, []*secp256k1.PrivateKey{lockingPrivKeyOne, lockingPrivKeyTwo})

	if err != nil {
		t.Fatalf("Error generating proofs: %v", err)
	}

	swapBlindedMessagesHTLCWrongPreimage, _, _, err = CreateHTLCBlindedMessages(1000, activeKeys, correctPreimage, 2, []*secp256k1.PublicKey{lockingPrivKeyTwo.PubKey()}, []*secp256k1.PublicKey{refundPrivKey.PubKey()}, 100, cashu.SigInputs)

	if err != nil {
		t.Fatalf("could not createBlind message: %v", err)
	}

	swapRequestTwo = cashu.PostSwapRequest{
		Inputs:  swapProofsTimelockNotExpiredWrongSig,
		Outputs: swapBlindedMessagesHTLCWrongPreimage,
	}

	jsonRequestBody, _ = json.Marshal(swapRequestTwo)

	req = httptest.NewRequest("POST", "/v1/swap", strings.NewReader(string(jsonRequestBody)))

	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("Expected status code 200, got %d", w.Code)
	}

}
