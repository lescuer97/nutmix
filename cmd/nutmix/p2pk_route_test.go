package main

import (
	"context"
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

func TestRoutesP2PKSwapMelt(t *testing.T) {
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

	if err != nil {
		t.Fatalf("could not parse locking pubkey: %v", err)
	}

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
	p2pkBlindedMessages, p2pkMintingSecrets, P2PKMintingSecretKeys, err := CreateP2PKBlindedMessages(1000, activeKeys, lockingPrivKey.PubKey(), 1, []*secp256k1.PublicKey{lockingPrivKey.PubKey()}, nil, 0, cashu.SigInputs)

	if err != nil {
		t.Fatalf("could not createBlind message: %v", err)
	}

	mintRequest := cashu.PostMintBolt11Request{
		Quote:   postMintQuoteResponse.Quote,
		Outputs: p2pkBlindedMessages,
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

	// SWAP P2PK TOKEN with other P2PK TOKENS
	swapProofs, err := GenerateProofsP2PK(postMintResponse.Signatures, activeKeys, p2pkMintingSecrets, P2PKMintingSecretKeys, []*secp256k1.PrivateKey{lockingPrivKey})

	if err != nil {
		t.Fatalf("Error generating proofs: %v", err)
	}

	swapBlindedMessagesP2PK, swapSecretsP2PK, swapSecretKeyP2PK, err := CreateP2PKBlindedMessages(1000, activeKeys, lockingPrivKey.PubKey(), 1, []*secp256k1.PublicKey{lockingPrivKey.PubKey()}, nil, 0, cashu.SigInputs)

	if err != nil {
		t.Fatalf("could not createBlind message: %v", err)
	}

	swapRequest := cashu.PostSwapRequest{
		Inputs:  swapProofs,
		Outputs: swapBlindedMessagesP2PK,
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
	swapProofsWrongSigs, err := GenerateProofsP2PK(postSwapResponse.Signatures, activeKeys, swapSecretsP2PK, swapSecretKeyP2PK, []*secp256k1.PrivateKey{wrongPrivKey})

	if err != nil {
		t.Fatalf("Error generating proofs: %v", err)
	}

	swapBlindedMessagesP2PKWrongSigs, _, _, err := CreateP2PKBlindedMessages(1000, activeKeys, lockingPrivKey.PubKey(), 1, []*secp256k1.PublicKey{lockingPrivKey.PubKey()}, nil, 0, cashu.SigInputs)

	if err != nil {
		t.Fatalf("could not createBlind message: %v", err)
	}

	swapRequestTwo := cashu.PostSwapRequest{
		Inputs:  swapProofsWrongSigs,
		Outputs: swapBlindedMessagesP2PKWrongSigs,
	}

	jsonRequestBody, _ = json.Marshal(swapRequestTwo)

	req = httptest.NewRequest("POST", "/v1/swap", strings.NewReader(string(jsonRequestBody)))

	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	errorResponse := cashu.ErrorResponse{}

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

}

func CreateP2PKBlindedMessages(amount uint64, keyset signer.GetKeysResponse, pubkey *secp256k1.PublicKey, nSigs uint, pubkeys []*secp256k1.PublicKey, refundPubkey []*secp256k1.PublicKey, locktime uint, sigflag cashu.SigFlag) ([]cashu.BlindedMessage, []string, []*secp256k1.PrivateKey, error) {
	splitAmounts := cashu.AmountSplit(amount)
	splitLen := len(splitAmounts)

	blindedMessages := make([]cashu.BlindedMessage, splitLen)
	secrets := make([]string, splitLen)
	rs := make([]*secp256k1.PrivateKey, splitLen)

	for i, amt := range splitAmounts {
		spendCond, err := makeP2PKSpendCondition(pubkey, nSigs, pubkeys, refundPubkey, locktime, sigflag)

		if err != nil {
			return nil, nil, nil, fmt.Errorf("MakeP2PKSpendCondition: %w", err)
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

func makeP2PKSpendCondition(pubkey *secp256k1.PublicKey, nSigs uint, pubkeys []*secp256k1.PublicKey, refundPubkey []*secp256k1.PublicKey, locktime uint, sigflag cashu.SigFlag) (cashu.SpendCondition, error) {
	var spendCondition cashu.SpendCondition
	spendCondition.Type = cashu.P2PK
	spendCondition.Data.Data = hex.EncodeToString(pubkey.SerializeCompressed())
	spendCondition.Data.Tags.Pubkeys = pubkeys
	spendCondition.Data.Tags.NSigs = nSigs
	spendCondition.Data.Tags.Locktime = locktime
	spendCondition.Data.Tags.Sigflag = sigflag
	spendCondition.Data.Tags.Refund = refundPubkey

	nonce, err := cashu.GenerateNonceHex()
	// generate random Nonce
	if err != nil {
		return spendCondition, err
	}
	spendCondition.Data.Nonce = nonce

	return spendCondition, nil
}

func GenerateProofsP2PK(signatures []cashu.BlindSignature, keyset signer.GetKeysResponse, secrets []string, secretsKey []*secp256k1.PrivateKey, privkeys []*secp256k1.PrivateKey) ([]cashu.Proof, error) {
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
		}

		if err != nil {
			return nil, fmt.Errorf("Error signing proof: %w", err)
		}

		proofs = append(proofs, proof)
	}

	return proofs, nil
}

func TestP2PKMultisigSigning(t *testing.T) {
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

	if err != nil {
		t.Fatalf("could not parse locking pubkey: %v", err)
	}

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
	p2pkBlindedMessages, p2pkMintingSecrets, P2PKMintingSecretKeys, err := CreateP2PKBlindedMessages(1000, activeKeys, lockingPrivKeyOne.PubKey(), 2, []*secp256k1.PublicKey{lockingPrivKeyTwo.PubKey()}, nil, 0, cashu.SigInputs)

	if err != nil {
		t.Fatalf("could not createBlind message: %v", err)
	}

	mintRequest := cashu.PostMintBolt11Request{
		Quote:   postMintQuoteResponse.Quote,
		Outputs: p2pkBlindedMessages,
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

	// SWAP P2PK TOKEN with other P2PK TOKENS
	// sign multisig with correct privkeys
	swapProofs, err := GenerateProofsP2PK(postMintResponse.Signatures, activeKeys, p2pkMintingSecrets, P2PKMintingSecretKeys, []*secp256k1.PrivateKey{lockingPrivKeyOne, lockingPrivKeyTwo})

	if err != nil {
		t.Fatalf("Error generating proofs: %v", err)
	}

	refundPrivKey := secp256k1.PrivKeyFromBytes([]byte{0x01, 0x02, 0x03, 0x06})

	swapBlindedMessagesP2PK, swapSecretsP2PK, swapSecretKeyP2PK, err := CreateP2PKBlindedMessages(1000, activeKeys, lockingPrivKeyOne.PubKey(), 2, []*secp256k1.PublicKey{lockingPrivKeyTwo.PubKey()}, []*secp256k1.PublicKey{refundPrivKey.PubKey()}, 100, cashu.SigInputs)

	if err != nil {
		t.Fatalf("could not createBlind message: %v", err)
	}

	swapRequest := cashu.PostSwapRequest{
		Inputs:  swapProofs,
		Outputs: swapBlindedMessagesP2PK,
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
	swapProofsTimelockNotExpiredWrongSig, err := GenerateProofsP2PK(postSwapResponse.Signatures, activeKeys, swapSecretsP2PK, swapSecretKeyP2PK, []*secp256k1.PrivateKey{lockingPrivKeyTwo, wrongPrivKey})

	if err != nil {
		t.Fatalf("Error generating proofs: %v", err)
	}

	swapBlindedMessagesP2PKWrongSigs, _, _, err := CreateP2PKBlindedMessages(1000, activeKeys, lockingPrivKeyOne.PubKey(), 2, []*secp256k1.PublicKey{lockingPrivKeyTwo.PubKey()}, []*secp256k1.PublicKey{refundPrivKey.PubKey()}, 100, cashu.SigInputs)

	if err != nil {
		t.Fatalf("could not createBlind message: %v", err)
	}

	swapRequestTwo := cashu.PostSwapRequest{
		Inputs:  swapProofsTimelockNotExpiredWrongSig,
		Outputs: swapBlindedMessagesP2PKWrongSigs,
	}

	jsonRequestBody, _ = json.Marshal(swapRequestTwo)

	req = httptest.NewRequest("POST", "/v1/swap", strings.NewReader(string(jsonRequestBody)))

	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("Expected status code 403, got %d", w.Code)
	}

	var errorRes cashu.ErrorResponse
	err = json.Unmarshal(w.Body.Bytes(), &errorRes)
	if err != nil {
		t.Fatalf("Error generating proofs: %v", err)
	}

	// if *errorRes.Detail != `Locktime has passed and no refund key was found` {
	// 	t.Fatalf("Expected response No valid signatures, got %s", *errorRes.Detail)
	// }

	// TRY SWAPPING with refund key
	swapProofsRefund, err := GenerateProofsP2PK(postSwapResponse.Signatures, activeKeys, swapSecretsP2PK, swapSecretKeyP2PK, []*secp256k1.PrivateKey{lockingPrivKeyTwo, refundPrivKey})
	if err != nil {
		t.Fatalf("Error generating refund proofs: %v", err)
	}

	currentPlus15 := time.Now().Add(15 * time.Minute).Unix()

	// generate new blind signatures with timelock over 15 minutes of current time
	swapBlindedMessagesP2PKWrongSigsOverlock, swapSecretsP2PK, swapSecretKeyP2PK, err := CreateP2PKBlindedMessages(1000, activeKeys, lockingPrivKeyOne.PubKey(), 2, []*secp256k1.PublicKey{lockingPrivKeyTwo.PubKey()}, []*secp256k1.PublicKey{refundPrivKey.PubKey()}, uint(currentPlus15), cashu.SigInputs)

	if err != nil {
		t.Fatalf("Error generating proofs: %v", err)
	}
	swapRequestRefund := cashu.PostSwapRequest{
		Inputs:  swapProofsRefund,
		Outputs: swapBlindedMessagesP2PKWrongSigsOverlock,
	}

	jsonRequestBody, _ = json.Marshal(swapRequestRefund)

	req = httptest.NewRequest("POST", "/v1/swap", strings.NewReader(string(jsonRequestBody)))

	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// var postSwapResponse cashu.PostSwapResponse

	if w.Code != 200 {
		t.Fatalf("Expected status code 200, got %d", w.Code)
	}

	err = json.Unmarshal(w.Body.Bytes(), &postSwapResponse)

	if err != nil {
		t.Fatalf("Error unmarshalling response: %v", err)
	}

	// try swapping with wrong refund key and timelock not yet expired

	swapProofsTimelockNotExpiredWrongSig, err = GenerateProofsP2PK(postSwapResponse.Signatures, activeKeys, swapSecretsP2PK, swapSecretKeyP2PK, []*secp256k1.PrivateKey{lockingPrivKeyTwo, wrongPrivKey})

	if err != nil {
		t.Fatalf("Error generating proofs: %v", err)
	}

	swapBlindedMessagesP2PKWrongSigs, _, _, err = CreateP2PKBlindedMessages(1000, activeKeys, lockingPrivKeyOne.PubKey(), 2, []*secp256k1.PublicKey{lockingPrivKeyTwo.PubKey()}, []*secp256k1.PublicKey{refundPrivKey.PubKey()}, 100, cashu.SigInputs)

	if err != nil {
		t.Fatalf("could not createBlind message: %v", err)
	}

	swapRequestTwo = cashu.PostSwapRequest{
		Inputs:  swapProofsTimelockNotExpiredWrongSig,
		Outputs: swapBlindedMessagesP2PKWrongSigs,
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

	if errorResponse.Code != 10001 {
		t.Errorf("Incorrect error code, got %v", errorResponse.Code)
	}
	if errorResponse.Error != "Proof could not be verified" {
		t.Errorf("Incorrect error string, got %s", errorResponse.Error)

	}

}
