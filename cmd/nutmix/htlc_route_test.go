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
	"github.com/lescuer97/nutmix/pkg/crypto"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var correctPreimage = hex.EncodeToString([]byte("12345"))

func TestRoutesHTLCSwapMelt(t *testing.T) {
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

	os.Setenv("DATABASE_URL", connUri)
	os.Setenv("MINT_PRIVATE_KEY", MintPrivateKey)
	os.Setenv("MINT_LIGHTNING_BACKEND", "FakeWallet")
	os.Setenv(mint.NETWORK_ENV, "regtest")

	ctx = context.WithValue(ctx, mint.NETWORK_ENV, os.Getenv(mint.NETWORK_ENV))
	ctx = context.WithValue(ctx, mint.MINT_LIGHTNING_BACKEND_ENV, os.Getenv(mint.MINT_LIGHTNING_BACKEND_ENV))
	ctx = context.WithValue(ctx, database.DATABASE_URL_ENV, os.Getenv(database.DATABASE_URL_ENV))
	ctx = context.WithValue(ctx, mint.NETWORK_ENV, os.Getenv(mint.NETWORK_ENV))

	router, mint := SetupRoutingForTesting(ctx)

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
	p2pkBlindedMessages, p2pkMintingSecrets, P2PKMintingSecretKeys, err := CreateHTLCBlindedMessages(1000, referenceKeyset, correctPreimage, 1, []*secp256k1.PublicKey{lockingPrivKey.PubKey()}, nil, 0, cashu.SigInputs)

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

	// SWAP HTLC TOKEN with other HTLC TOKENS
	swapProofs, err := GenerateProofsHTLC(postMintResponse.Signatures, correctPreimage, mint.ActiveKeysets, p2pkMintingSecrets, P2PKMintingSecretKeys, []*secp256k1.PrivateKey{lockingPrivKey})

	if err != nil {
		t.Fatalf("Error generating proofs: %v", err)
	}


	swapBlindedMessagesP2PK, swapSecretsP2PK, swapSecretKeyP2PK, err := CreateHTLCBlindedMessages(1000, mint.ActiveKeysets[cashu.Sat.String()][1], correctPreimage, 1, []*secp256k1.PublicKey{lockingPrivKey.PubKey()}, nil, 0, cashu.SigInputs)

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
	swapProofsWrongSigs, err := GenerateProofsHTLC(postSwapResponse.Signatures, correctPreimage,  mint.ActiveKeysets, swapSecretsP2PK, swapSecretKeyP2PK, []*secp256k1.PrivateKey{wrongPrivKey})

	if err != nil {
		t.Fatalf("Error generating proofs: %v", err)
	}

	swapBlindedMessagesP2PKWrongSigs, _, _, err := CreateHTLCBlindedMessages(1000, mint.ActiveKeysets[cashu.Sat.String()][1], correctPreimage, 1, []*secp256k1.PublicKey{lockingPrivKey.PubKey()}, nil, 0, cashu.SigInputs)

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

func CreateHTLCBlindedMessages(amount uint64, keyset cashu.Keyset, preimage string, nSigs int, pubkeys []*secp256k1.PublicKey, refundPubkey []*secp256k1.PublicKey, locktime int, sigflag cashu.SigFlag) ([]cashu.BlindedMessage, []string, []*secp256k1.PrivateKey, error) {
	splitAmounts := cashu.AmountSplit(amount)
	splitLen := len(splitAmounts)

	blindedMessages := make([]cashu.BlindedMessage, splitLen)
	secrets := make([]string, splitLen)
	rs := make([]*secp256k1.PrivateKey, splitLen)

	for i, amt := range splitAmounts {
		spendCond, err := makeHTLCSpendCondition(preimage, nSigs, pubkeys, refundPubkey, locktime, sigflag)

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

func makeHTLCSpendCondition(preimage string, nSigs int, pubkeys []*secp256k1.PublicKey, refundPubkey []*secp256k1.PublicKey, locktime int, sigflag cashu.SigFlag) (cashu.SpendCondition, error) {

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
	nonce := make([]byte, 32)  // create a slice with length 16 for the nonce
    _, err = rand.Read(nonce) // read random bytes into the nonce slice
	if err != nil {
		return spendCondition, err
	}
	spendCondition.Data.Nonce = hex.EncodeToString(nonce)

	return spendCondition, nil
}

func GenerateProofsHTLC(signatures []cashu.BlindSignature, preimage string, keysets map[string]mint.KeysetMap, secrets []string, secretsKey []*secp256k1.PrivateKey, privkeys []*secp256k1.PrivateKey) ([]cashu.Proof, error) {
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

		mintPublicKey, err := secp256k1.ParsePubKey(keysets[cashu.Sat.String()][output.Amount].PrivKey.PubKey().SerializeCompressed())
		if err != nil {
			return nil, fmt.Errorf("Error parsing pubkey: %w", err)
		}

		C := crypto.UnblindSignature(blindedFactor, secretsKey[i], mintPublicKey)

		hexC := hex.EncodeToString(C.SerializeCompressed())

		proof := cashu.Proof{Id: output.Id, Amount: output.Amount, C: hexC, Secret: secrets[i]}

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

	os.Setenv(database.DATABASE_URL_ENV, connUri)
	os.Setenv(MINT_PRIVATE_KEY_ENV, MintPrivateKey)
	os.Setenv(mint.MINT_LIGHTNING_BACKEND_ENV, "FakeWallet")
	os.Setenv(mint.NETWORK_ENV, "regtest")

	ctx = context.WithValue(ctx, mint.NETWORK_ENV, os.Getenv(mint.NETWORK_ENV))
	ctx = context.WithValue(ctx, mint.MINT_LIGHTNING_BACKEND_ENV, os.Getenv(mint.MINT_LIGHTNING_BACKEND_ENV))
	ctx = context.WithValue(ctx, database.DATABASE_URL_ENV, os.Getenv(database.DATABASE_URL_ENV))
	ctx = context.WithValue(ctx, mint.NETWORK_ENV, os.Getenv(mint.NETWORK_ENV))

	router, mint := SetupRoutingForTesting(ctx)

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
	p2pkBlindedMessages, p2pkMintingSecrets, P2PKMintingSecretKeys, err := CreateHTLCBlindedMessages(1000, referenceKeyset, correctPreimage, 2, []*secp256k1.PublicKey{lockingPrivKeyTwo.PubKey()}, nil, 0, cashu.SigInputs)

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
	swapProofs, err := GenerateProofsHTLC(postMintResponse.Signatures,correctPreimage, mint.ActiveKeysets, p2pkMintingSecrets, P2PKMintingSecretKeys, []*secp256k1.PrivateKey{lockingPrivKeyOne, lockingPrivKeyTwo})

	if err != nil {
		t.Fatalf("Error generating proofs: %v", err)
	}

	refundPrivKey := secp256k1.PrivKeyFromBytes([]byte{0x01, 0x02, 0x03, 0x06})

	swapBlindedMessagesP2PK, swapSecretsP2PK, swapSecretKeyP2PK, err := CreateHTLCBlindedMessages(1000, mint.ActiveKeysets[cashu.Sat.String()][1], correctPreimage, 2, []*secp256k1.PublicKey{lockingPrivKeyTwo.PubKey()}, []*secp256k1.PublicKey{refundPrivKey.PubKey()}, 100, cashu.SigInputs)

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
	swapProofsTimelockNotExpiredWrongSig, err := GenerateProofsHTLC(postSwapResponse.Signatures, correctPreimage, mint.ActiveKeysets, swapSecretsP2PK, swapSecretKeyP2PK, []*secp256k1.PrivateKey{lockingPrivKeyTwo, wrongPrivKey})

	if err != nil {
		t.Fatalf("Error generating proofs: %v", err)
	}

	swapBlindedMessagesP2PKWrongSigs, _, _, err := CreateHTLCBlindedMessages(1000, mint.ActiveKeysets[cashu.Sat.String()][1], correctPreimage, 2, []*secp256k1.PublicKey{lockingPrivKeyTwo.PubKey()}, []*secp256k1.PublicKey{refundPrivKey.PubKey()}, 100, cashu.SigInputs)

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

	if w.Code != 403 {
		t.Fatalf("Expected status code 403, got %d", w.Code)
	}

	if w.Body.String() != `"Locktime has passed and no refund key was found"` {
		t.Fatalf("Expected response No valid signatures, got %s", w.Body.String())
	}

	// TRY SWAPPING with refund key
	swapProofsRefund, err := GenerateProofsHTLC(postSwapResponse.Signatures, correctPreimage, mint.ActiveKeysets, swapSecretsP2PK, swapSecretKeyP2PK, []*secp256k1.PrivateKey{lockingPrivKeyTwo, refundPrivKey})

	currentPlus15 := time.Now().Add(15 * time.Minute).Unix()

	// generate new blind signatures with timelock over 15 minutes of current time
	swapBlindedMessagesP2PKWrongSigsOverlock, swapSecretsP2PK, swapSecretKeyP2PK, err := CreateHTLCBlindedMessages(1000, mint.ActiveKeysets[cashu.Sat.String()][1], correctPreimage, 2, []*secp256k1.PublicKey{lockingPrivKeyTwo.PubKey()}, []*secp256k1.PublicKey{refundPrivKey.PubKey()}, int(currentPlus15), cashu.SigInputs)

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

	swapProofsTimelockNotExpiredWrongSig, err = GenerateProofsHTLC(postSwapResponse.Signatures, correctPreimage, mint.ActiveKeysets, swapSecretsP2PK, swapSecretKeyP2PK, []*secp256k1.PrivateKey{lockingPrivKeyTwo, wrongPrivKey})

	if err != nil {
		t.Fatalf("Error generating proofs: %v", err)
	}

	swapBlindedMessagesP2PKWrongSigs, _, _, err = CreateHTLCBlindedMessages(1000, mint.ActiveKeysets[cashu.Sat.String()][1], correctPreimage, 2, []*secp256k1.PublicKey{lockingPrivKeyTwo.PubKey()}, []*secp256k1.PublicKey{refundPrivKey.PubKey()}, 100, cashu.SigInputs)

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

	if w.Code != 403 {
		t.Fatalf("Expected status code 403, got %d", w.Code)
	}

	if w.Body.String() != `"Not enough signatures"` {
		t.Fatalf("Expected response No valid signatures, got %s", w.Body.String())
	}

}
