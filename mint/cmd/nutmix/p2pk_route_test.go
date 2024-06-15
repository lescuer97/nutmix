package main

import (
	"context"
	"crypto/rand"
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
	"github.com/lescuer97/nutmix/pkg/crypto"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestRoutesSwapMelt(t *testing.T) {
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
		t.Fatal(fmt.Errorf("failed to get connection string: %s", err))
	}

	os.Setenv("DATABASE_URL", connUri)
	os.Setenv("MINT_PRIVATE_KEY", MintPrivateKey)
	os.Setenv("MINT_LIGHTNING_BACKEND", "FakeWallet")

	router, mint := SetupRoutingForTesting()

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
	var postMintQuoteResponse cashu.PostMintQuoteBolt11Response
	err = json.Unmarshal(w.Body.Bytes(), &postMintQuoteResponse)

	if err != nil {
		t.Errorf("Error unmarshalling response: %v", err)
	}

	referenceKeyset := mint.ActiveKeysets[cashu.Sat.String()][1]

	// ask for minting
	p2pkBlindedMessages, p2pkMintingSecrets, P2PKMintingSecretKeys, err := CreateP2PKBlindedMessages(1000, referenceKeyset, lockingPrivKey.PubKey(), 1, []*secp256k1.PublicKey{lockingPrivKey.PubKey()}, nil, 0, cashu.SigInputs)

	if err != nil {
		t.Fatalf("could not createBlind message: %v", err)
	}

	mintRequest := cashu.PostMintBolt11Request{
		Quote:   postMintQuoteResponse.Quote,
		Outputs: p2pkBlindedMessages,
	}

	jsonRequestBody, _ = json.Marshal(mintRequest)

	var aliceBlindSigs []cashu.BlindSignature

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

	aliceBlindSigs = append(aliceBlindSigs, postMintResponse.Signatures...)

	// SWAP P2PK TOKEN with other P2PK TOKENS
	swapProofs, err := GenerateProofsP2PK(postMintResponse.Signatures, mint.ActiveKeysets, p2pkMintingSecrets, P2PKMintingSecretKeys, []*secp256k1.PrivateKey{lockingPrivKey})

	if err != nil {
		t.Fatalf("Error generating proofs: %v", err)
	}

	swapBlindedMessagesP2PK, swapSecretsP2PK, swapSecretKeyP2PK, err := CreateP2PKBlindedMessages(1000, mint.ActiveKeysets[cashu.Sat.String()][1], lockingPrivKey.PubKey(), 1, []*secp256k1.PublicKey{lockingPrivKey.PubKey()}, nil, 0, cashu.SigInputs)

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
	swapProofsWrongSigs, err := GenerateProofsP2PK(postSwapResponse.Signatures, mint.ActiveKeysets, swapSecretsP2PK, swapSecretKeyP2PK, []*secp256k1.PrivateKey{wrongPrivKey})

	if err != nil {
		t.Fatalf("Error generating proofs: %v", err)
	}

	swapBlindedMessagesP2PKWrongSigs, _, _, err := CreateP2PKBlindedMessages(1000, mint.ActiveKeysets[cashu.Sat.String()][1], lockingPrivKey.PubKey(), 1, []*secp256k1.PublicKey{lockingPrivKey.PubKey()}, nil, 0, cashu.SigInputs)

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

	if w.Code != 403 {
		t.Fatalf("Expected status code 403, got %d", w.Code)
	}

	if w.Body.String() != `"No valid signatures"` {
		t.Fatalf("Expected response No valid signatures, got %s", w.Body.String())
	}

}

func CreateP2PKBlindedMessages(amount uint64, keyset cashu.Keyset, pubkey *secp256k1.PublicKey, nSigs int, pubkeys []*secp256k1.PublicKey, refundPubkey []*secp256k1.PublicKey, locktime int, sigflag cashu.SigFlag) ([]cashu.BlindedMessage, []string, []*secp256k1.PrivateKey, error) {
	splitAmounts := cashu.AmountSplit(amount)
	splitLen := len(splitAmounts)

	blindedMessages := make([]cashu.BlindedMessage, splitLen)
	secrets := make([]string, splitLen)
	rs := make([]*secp256k1.PrivateKey, splitLen)

	for i, amt := range splitAmounts {
		spendCond, err := makeP2PKSpendCondition(pubkey, nSigs, pubkeys, refundPubkey, locktime, sigflag)

		if err != nil {
			return nil, nil, nil, fmt.Errorf("MakeP2PKSpendCondition: %+v", err)
		}

		jsonSpend, err := spendCond.String()

		if err != nil {
			return nil, nil, nil, fmt.Errorf("json.Marshal(spendCond): %+v", err)
		}

		// generate new private key r
		r, err := secp256k1.GeneratePrivateKey()
		if err != nil {
			return nil, nil, nil, err
		}

		var B_ *secp256k1.PublicKey
		var secret string = jsonSpend
		// generate random secret until it finds valid point
		for {
			B_, r, err = crypto.BlindMessage(secret, r)
			if err == nil {
				break
			}
		}

		blindedMessage := newBlindedMessage(keyset.Id, amt, B_)
		blindedMessages[i] = blindedMessage
		secrets[i] = secret
		rs[i] = r
	}

	return blindedMessages, secrets, rs, nil
}

func makeP2PKSpendCondition(pubkey *secp256k1.PublicKey, nSigs int, pubkeys []*secp256k1.PublicKey, refundPubkey []*secp256k1.PublicKey, locktime int, sigflag cashu.SigFlag) (cashu.SpendCondition, error) {
	var spendCondition cashu.SpendCondition
	spendCondition.Type = cashu.P2PK
	spendCondition.Data.Data = pubkey
	spendCondition.Data.Tags.Pubkeys = pubkeys
	spendCondition.Data.Tags.NSigs = nSigs
	spendCondition.Data.Tags.Locktime = locktime
	spendCondition.Data.Tags.Sigflag = sigflag
	spendCondition.Data.Tags.Refund = refundPubkey

	// generate random Nonce
	nonce := make([]byte, 32)  // create a slice with length 16 for the nonce
	_, err := rand.Read(nonce) // read random bytes into the nonce slice
	if err != nil {
		return spendCondition, err
	}
	spendCondition.Data.Nonce = hex.EncodeToString(nonce)

	return spendCondition, nil
}

func GenerateProofsP2PK(signatures []cashu.BlindSignature, keysets map[string]KeysetMap, secrets []string, secretsKey []*secp256k1.PrivateKey, privkeys []*secp256k1.PrivateKey) ([]cashu.Proof, error) {
	// try to swap tokens
	var proofs []cashu.Proof
	// unblid the signatures and make proofs
	for i, output := range signatures {

		parsedBlindFactor, err := hex.DecodeString(output.C_)
		if err != nil {
			return nil, fmt.Errorf("Error decoding hex: %v", err)
		}
		blindedFactor, err := secp256k1.ParsePubKey(parsedBlindFactor)
		if err != nil {
			return nil, fmt.Errorf("Error parsing pubkey: %v", err)
		}

		mintPublicKey, err := secp256k1.ParsePubKey(keysets[cashu.Sat.String()][output.Amount].PrivKey.PubKey().SerializeCompressed())
		if err != nil {
			return nil, fmt.Errorf("Error parsing pubkey: %v", err)
		}

		C := crypto.UnblindSignature(blindedFactor, secretsKey[i], mintPublicKey)

		hexC := hex.EncodeToString(C.SerializeCompressed())

		proof := cashu.Proof{Id: output.Id, Amount: output.Amount, C: hexC, Secret: secrets[i]}

		for _, privkey := range privkeys {
			err = proof.Sign(privkey)
			if err != nil {
				return nil, fmt.Errorf("Error signing proof: %v", err)
			}
		}

		if err != nil {
			return nil, fmt.Errorf("Error signing proof: %v", err)
		}

		proofs = append(proofs, proof)
	}

	return proofs, nil
}

func TestMultisigSigning(t *testing.T) {
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
		t.Fatal(fmt.Errorf("failed to get connection string: %s", err))
	}

	os.Setenv("DATABASE_URL", connUri)
	os.Setenv("MINT_PRIVATE_KEY", MintPrivateKey)
	os.Setenv("MINT_LIGHTNING_BACKEND", "FakeWallet")

	router, mint := SetupRoutingForTesting()

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
	var postMintQuoteResponse cashu.PostMintQuoteBolt11Response
	err = json.Unmarshal(w.Body.Bytes(), &postMintQuoteResponse)

	if err != nil {
		t.Errorf("Error unmarshalling response: %v", err)
	}

	referenceKeyset := mint.ActiveKeysets[cashu.Sat.String()][1]

	// ask for minting
	// Create multisig token for 2 pubkeys
	p2pkBlindedMessages, p2pkMintingSecrets, P2PKMintingSecretKeys, err := CreateP2PKBlindedMessages(1000, referenceKeyset, lockingPrivKeyOne.PubKey(), 2, []*secp256k1.PublicKey{lockingPrivKeyTwo.PubKey()}, nil, 0, cashu.SigInputs)

	if err != nil {
		t.Fatalf("could not createBlind message: %v", err)
	}

	mintRequest := cashu.PostMintBolt11Request{
		Quote:   postMintQuoteResponse.Quote,
		Outputs: p2pkBlindedMessages,
	}

	jsonRequestBody, _ = json.Marshal(mintRequest)

	var aliceBlindSigs []cashu.BlindSignature

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

	aliceBlindSigs = append(aliceBlindSigs, postMintResponse.Signatures...)

	// SWAP P2PK TOKEN with other P2PK TOKENS
	// sign multisig with correct privkeys
	swapProofs, err := GenerateProofsP2PK(postMintResponse.Signatures, mint.ActiveKeysets, p2pkMintingSecrets, P2PKMintingSecretKeys, []*secp256k1.PrivateKey{lockingPrivKeyOne, lockingPrivKeyTwo})

	if err != nil {
		t.Fatalf("Error generating proofs: %v", err)
	}

	refundPrivKey := secp256k1.PrivKeyFromBytes([]byte{0x01, 0x02, 0x03, 0x06})

	swapBlindedMessagesP2PK, swapSecretsP2PK, swapSecretKeyP2PK, err := CreateP2PKBlindedMessages(1000, mint.ActiveKeysets[cashu.Sat.String()][1], lockingPrivKeyOne.PubKey(), 2, []*secp256k1.PublicKey{lockingPrivKeyTwo.PubKey()}, []*secp256k1.PublicKey{refundPrivKey.PubKey()}, 100, cashu.SigInputs)

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

	// // TRY SWAPING with WRONG SIGNATURES
	swapProofsWrongSigs, err := GenerateProofsP2PK(postSwapResponse.Signatures, mint.ActiveKeysets, swapSecretsP2PK, swapSecretKeyP2PK, []*secp256k1.PrivateKey{lockingPrivKeyTwo, wrongPrivKey})

	if err != nil {
		t.Fatalf("Error generating proofs: %v", err)
	}

	swapBlindedMessagesP2PKWrongSigs, _, _, err := CreateP2PKBlindedMessages(1000, mint.ActiveKeysets[cashu.Sat.String()][1], lockingPrivKeyOne.PubKey(), 2, []*secp256k1.PublicKey{lockingPrivKeyTwo.PubKey()}, []*secp256k1.PublicKey{refundPrivKey.PubKey()}, 100, cashu.SigInputs)

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

	if w.Code != 403 {
		t.Fatalf("Expected status code 403, got %d", w.Code)
	}

	if w.Body.String() != `"Not enough signatures"` {
		t.Fatalf("Expected response No valid signatures, got %s", w.Body.String())
	}

	// TRY SWAPPING with refund key
	swapProofsRefund, err := GenerateProofsP2PK(postSwapResponse.Signatures, mint.ActiveKeysets, swapSecretsP2PK, swapSecretKeyP2PK, []*secp256k1.PrivateKey{lockingPrivKeyTwo, refundPrivKey})

	if err != nil {
		t.Fatalf("Error generating proofs: %v", err)
	}
	swapRequestRefund := cashu.PostSwapRequest{
		Inputs:  swapProofsRefund,
		Outputs: swapBlindedMessagesP2PKWrongSigs,
	}

	jsonRequestBody, _ = json.Marshal(swapRequestRefund)

	req = httptest.NewRequest("POST", "/v1/swap", strings.NewReader(string(jsonRequestBody)))

	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("Expected status code 200, got %d", w.Code)
	}

}
