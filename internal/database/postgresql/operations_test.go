package postgresql

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/utils"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestAddAndRequestMintRequestValidPubkey(t *testing.T) {
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
	defer func() {
		if err := postgresContainer.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	}()

	connUri, err := postgresContainer.ConnectionString(ctx)
	if err != nil {
		t.Fatal(fmt.Errorf("failed to get connection string: %v", err))
	}

	t.Setenv("DATABASE_URL", connUri)

	db, err := DatabaseSetup(ctx, "migrations")
	if err != nil {
		t.Fatalf("could not setup migration. %v", err)

	}

	pubkeyStr := "03d56ce4e446a85bbdaa547b4ec2b073d40ff802831352b8272b7dd7a4de5a7cac"
	pubkeyBytes, err := hex.DecodeString(pubkeyStr)
	if err != nil {
		t.Fatalf("could not decode hex string. %v", err)
	}

	pubkey, err := secp256k1.ParsePubKey(pubkeyBytes)
	if err != nil {
		t.Fatalf("could not parse pubkey bytes correctly. %v", err)
	}

	quoteId, err := utils.RandomHash()
	if err != nil {
		t.Fatalf("could not generate new random hash. %v", err)
	}
	amount := uint64(1000)
	now := time.Now().Unix()

	mintRequestDB := cashu.MintRequestDB{
		Quote:       quoteId,
		RequestPaid: false,
		Expiry:      now,
		Unit:        cashu.Sat.String(),
		State:       cashu.UNPAID,
		SeenAt:      now,
		Amount:      &amount,
		Pubkey:      cashu.WrappedPublicKey{PublicKey: pubkey},
	}

	log.Println("adding mint request to database")
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatalf("could not get transaction. %v", err)
	}
	err = db.SaveMintRequest(tx, mintRequestDB)
	if err != nil {
		t.Fatalf("db.SaveMintRequest(tx, mintRequestDB). %v", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		t.Fatalf("could not commit transaction. %v", err)
	}

	log.Println("adding mint request to database")
	tx, err = db.GetTx(ctx)
	if err != nil {
		t.Fatalf("could not get transaction. %v", err)
	}
	mintRequest, err := db.GetMintRequestById(tx, quoteId)
	if err != nil {
		t.Fatalf("db.GetMintRequestById(tx, mintRequestDB). %v", err)
	}

	// Verify that the pubkey retrieved from DB matches the one we saved
	if mintRequest.Pubkey.PublicKey == nil {
		t.Fatal("pubkey should not be nil after retrieval")
	}
	retrievedPubkeyStr := hex.EncodeToString(mintRequest.Pubkey.SerializeCompressed())
	if retrievedPubkeyStr != pubkeyStr {
		t.Errorf("pubkey mismatch: saved %s, got %s", pubkeyStr, retrievedPubkeyStr)
	}

	err = tx.Commit(ctx)
	if err != nil {
		t.Fatalf("could not commit transaction. %v", err)
	}
}
func TestAddAndRequestMintRequestNilPubkey(t *testing.T) {
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
	defer func() {
		if err := postgresContainer.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	}()

	connUri, err := postgresContainer.ConnectionString(ctx)
	if err != nil {
		t.Fatal(fmt.Errorf("failed to get connection string: %v", err))
	}

	t.Setenv("DATABASE_URL", connUri)

	db, err := DatabaseSetup(ctx, "migrations")
	if err != nil {
		t.Fatalf("could not setup migration. %v", err)

	}

	quoteId, err := utils.RandomHash()
	if err != nil {
		t.Fatalf("could not generate new random hash. %v", err)
	}
	amount := uint64(1000)
	now := time.Now().Unix()

	mintRequestDB := cashu.MintRequestDB{
		Quote:       quoteId,
		RequestPaid: false,
		Expiry:      now,
		Unit:        cashu.Sat.String(),
		State:       cashu.UNPAID,
		SeenAt:      now,
		Amount:      &amount,
		Pubkey:      cashu.WrappedPublicKey{PublicKey: nil},
	}

	log.Println("adding mint request to database")
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatalf("could not get transaction. %v", err)
	}
	err = db.SaveMintRequest(tx, mintRequestDB)
	if err != nil {
		t.Fatalf("db.SaveMintRequest(tx, mintRequestDB). %v", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		t.Fatalf("could not commit transaction. %v", err)
	}

	log.Println("adding mint request to database")
	tx, err = db.GetTx(ctx)
	if err != nil {
		t.Fatalf("could not get transaction. %v", err)
	}
	mintRequest, err := db.GetMintRequestById(tx, quoteId)
	if err != nil {
		t.Fatalf("db.GetMintRequestById(tx, mintRequestDB). %v", err)
	}

	if mintRequest.Pubkey.PublicKey != nil {
		t.Errorf("pubkey should be nil. %v", mintRequest.Pubkey)
	}

	err = tx.Commit(ctx)
	if err != nil {
		t.Fatalf("could not commit transaction. %v", err)
	}
}

func TestGetMintMeltBalanceByTime_OnlyPubkey(t *testing.T) {
	db, ctx := setupTestDB(t)

	pubkeyStr := "03d56ce4e446a85bbdaa547b4ec2b073d40ff802831352b8272b7dd7a4de5a7cac"
	pubkeyBytes, err := hex.DecodeString(pubkeyStr)
	if err != nil {
		t.Fatalf("could not decode hex string. %v", err)
	}
	pubkey, err := secp256k1.ParsePubKey(pubkeyBytes)
	if err != nil {
		t.Fatalf("could not parse pubkey bytes correctly. %v", err)
	}
	wrappedPubkey := cashu.WrappedPublicKey{PublicKey: pubkey}

	now := time.Now().Unix()
	queryTime := now - 500

	entryOldTime := now - 1000
	entryNewTime := now - 100

	// Mint Request 1: Old, Issued (Excluded by time)
	mint1 := cashu.MintRequestDB{
		Quote:       "mint1",
		State:       cashu.ISSUED,
		SeenAt:      entryOldTime,
		Amount:      ptr(100),
		Pubkey:      wrappedPubkey,
		Request:     "req1",
		Unit:        cashu.Sat.String(),
		Expiry:      now + 10000,
		RequestPaid: true,
	}

	// Mint Request 2: New, Issued (Included)
	mint2 := cashu.MintRequestDB{
		Quote:       "mint2",
		State:       cashu.ISSUED,
		SeenAt:      entryNewTime,
		Amount:      ptr(200),
		Pubkey:      wrappedPubkey,
		Request:     "req2",
		Unit:        cashu.Sat.String(),
		Expiry:      now + 10000,
		RequestPaid: true,
	}

	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatalf("could not get transaction. %v", err)
	}
	if err := db.SaveMintRequest(tx, mint1); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveMintRequest(tx, mint2); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	// Melt Requests
	// Melt Request 1: Old, Issued (Excluded by time)
	melt1 := cashu.MeltRequestDB{
		Quote:       "melt1",
		State:       cashu.ISSUED,
		SeenAt:      entryOldTime,
		Amount:      100,
		Request:     "reqMelt1",
		Unit:        cashu.Sat.String(),
		Expiry:      now + 10000,
		RequestPaid: true,
	}

	// Melt Request 2: New, Paid (Included)
	melt2 := cashu.MeltRequestDB{
		Quote:       "melt2",
		State:       cashu.PAID,
		SeenAt:      entryNewTime,
		Amount:      200,
		Request:     "reqMelt2",
		Unit:        cashu.Sat.String(),
		Expiry:      now + 10000,
		RequestPaid: true,
	}

	tx, err = db.GetTx(ctx)
	if err != nil {
		t.Fatalf("could not get transaction. %v", err)
	}
	if err := db.SaveMeltRequest(tx, melt1); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveMeltRequest(tx, melt2); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	// Query
	res, err := db.GetMintMeltBalanceByTime(queryTime)
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Mint) != 1 {
		t.Errorf("Expected 1 mint request, got %d", len(res.Mint))
	} else {
		if res.Mint[0].Quote != "mint2" {
			t.Errorf("Expected mint2, got %s", res.Mint[0].Quote)
		}
		if res.Mint[0].Pubkey.PublicKey == nil {
			t.Error("Expected pubkey to be present")
		} else if hex.EncodeToString(res.Mint[0].Pubkey.SerializeCompressed()) != pubkeyStr {
			t.Errorf("Pubkey mismatch. Got %x, expected %s", res.Mint[0].Pubkey.SerializeCompressed(), pubkeyStr)
		}
	}

	if len(res.Melt) != 1 {
		t.Errorf("Expected 1 melt request, got %d", len(res.Melt))
	} else {
		if res.Melt[0].Quote != "melt2" {
			t.Errorf("Expected melt2, got %s", res.Melt[0].Quote)
		}
	}
}

func TestGetMintMeltBalanceByTime_MixedPubkeys(t *testing.T) {
	db, ctx := setupTestDB(t)

	pubkeyStr := "03d56ce4e446a85bbdaa547b4ec2b073d40ff802831352b8272b7dd7a4de5a7cac"
	pubkeyBytes, err := hex.DecodeString(pubkeyStr)
	if err != nil {
		t.Fatalf("could not decode hex string. %v", err)
	}
	pubkey, err := secp256k1.ParsePubKey(pubkeyBytes)
	if err != nil {
		t.Fatalf("could not parse pubkey bytes correctly. %v", err)
	}
	wrappedPubkey := cashu.WrappedPublicKey{PublicKey: pubkey}

	now := time.Now().Unix()
	queryTime := now - 500
	entryNewTime := now - 100

	// Mint Request with Pubkey
	mint1 := cashu.MintRequestDB{
		Quote:       "mintWithKey",
		State:       cashu.ISSUED,
		SeenAt:      entryNewTime,
		Amount:      ptr(200),
		Pubkey:      wrappedPubkey,
		Request:     "reqWithKey",
		Unit:        cashu.Sat.String(),
		Expiry:      now + 10000,
		RequestPaid: true,
	}

	// Mint Request without Pubkey
	mint2 := cashu.MintRequestDB{
		Quote:       "mintNoKey",
		State:       cashu.ISSUED,
		SeenAt:      entryNewTime,
		Amount:      ptr(400),
		Pubkey:      cashu.WrappedPublicKey{PublicKey: nil},
		Request:     "reqNokey",
		Unit:        cashu.Sat.String(),
		Expiry:      now + 10000,
		RequestPaid: true,
	}

	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatalf("could not get transaction. %v", err)
	}
	if err := db.SaveMintRequest(tx, mint1); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveMintRequest(tx, mint2); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	// Melt Requests
	melt1 := cashu.MeltRequestDB{
		Quote:       "melt1",
		State:       cashu.ISSUED,
		SeenAt:      entryNewTime,
		Amount:      100,
		Request:     "reqMelt1",
		Unit:        cashu.Sat.String(),
		Expiry:      now + 10000,
		RequestPaid: true,
	}

	tx, err = db.GetTx(ctx)
	if err != nil {
		t.Fatalf("could not get transaction. %v", err)
	}
	if err := db.SaveMeltRequest(tx, melt1); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	// Query
	res, err := db.GetMintMeltBalanceByTime(queryTime)
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Mint) != 2 {
		t.Errorf("Expected 2 mint requests, got %d", len(res.Mint))
	}

	var foundWithKey, foundNoKey bool
	for _, m := range res.Mint {
		switch m.Quote {
		case "mintWithKey":
			foundWithKey = true
			if m.Pubkey.PublicKey == nil {
				t.Error("Expected mintWithKey to have a pubkey")
			} else if hex.EncodeToString(m.Pubkey.SerializeCompressed()) != pubkeyStr {
				t.Errorf("Pubkey mismatch. Got %x, expected %s", m.Pubkey.SerializeCompressed(), pubkeyStr)
			}
		case "mintNoKey":
			foundNoKey = true
			if m.Pubkey.PublicKey != nil {
				t.Errorf("Expected mintNoKey to have NO pubkey, but got %v", m.Pubkey.PublicKey)
			}
		}
	}

	if !foundWithKey {
		t.Error("Did not find mintWithKey")
	}
	if !foundNoKey {
		t.Error("Did not find mintNoKey")
	}

	if len(res.Melt) != 1 {
		t.Errorf("Expected 1 melt request, got %d", len(res.Melt))
	} else {
		if res.Melt[0].Quote != "melt1" {
			t.Errorf("Expected melt1, got %s", res.Melt[0].Quote)
		}
	}
}

func TestSaveProofAndGetBySecret_ValidPubkey(t *testing.T) {
	db, ctx := setupTestDB(t)

	// Create a valid public key for the C field
	cPubkeyStr := "03d56ce4e446a85bbdaa547b4ec2b073d40ff802831352b8272b7dd7a4de5a7cac"
	cPubkeyBytes, err := hex.DecodeString(cPubkeyStr)
	if err != nil {
		t.Fatalf("could not decode C hex string. %v", err)
	}
	cPubkey, err := secp256k1.ParsePubKey(cPubkeyBytes)
	if err != nil {
		t.Fatalf("could not parse C pubkey bytes correctly. %v", err)
	}
	wrappedC := cashu.WrappedPublicKey{PublicKey: cPubkey}

	// Create a valid public key for the Y field (using a different key)
	yPubkeyStr := "02a9acc1e48c25eeeb9289b5031cc57da9fe72f3fe2861d264bdc074209b107ba2"
	yPubkeyBytes, err := hex.DecodeString(yPubkeyStr)
	if err != nil {
		t.Fatalf("could not decode Y hex string. %v", err)
	}
	yPubkey, err := secp256k1.ParsePubKey(yPubkeyBytes)
	if err != nil {
		t.Fatalf("could not parse Y pubkey bytes correctly. %v", err)
	}
	wrappedY := cashu.WrappedPublicKey{PublicKey: yPubkey}

	now := time.Now().Unix()
	secret := "test_secret_1"

	proof := cashu.Proof{
		Amount:  100,
		Id:      "test_keyset_id",
		Secret:  secret,
		C:       wrappedC,
		Y:       wrappedY,
		Witness: "",
		SeenAt:  now,
		State:   cashu.PROOF_UNSPENT,
		Quote:   nil,
	}

	// Save the proof
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatalf("could not get transaction. %v", err)
	}
	err = db.SaveProof(tx, []cashu.Proof{proof})
	if err != nil {
		t.Fatalf("db.SaveProof failed: %v", err)
	}
	err = tx.Commit(ctx)
	if err != nil {
		t.Fatalf("could not commit transaction. %v", err)
	}

	// Retrieve the proof by secret
	tx, err = db.GetTx(ctx)
	if err != nil {
		t.Fatalf("could not get transaction. %v", err)
	}
	proofs, err := db.GetProofsFromSecret(tx, []string{secret})
	if err != nil {
		t.Fatalf("db.GetProofsFromSecret failed: %v", err)
	}
	err = tx.Commit(ctx)
	if err != nil {
		t.Fatalf("could not commit transaction. %v", err)
	}

	// Verify we got exactly one proof
	if len(proofs) != 1 {
		t.Fatalf("expected 1 proof, got %d", len(proofs))
	}

	retrievedProof := proofs[0]

	// Verify the C (WrappedPublicKey) field matches
	if retrievedProof.C.PublicKey == nil {
		t.Fatal("C field (pubkey) should not be nil after retrieval")
	}
	retrievedCStr := hex.EncodeToString(retrievedProof.C.SerializeCompressed())
	if retrievedCStr != cPubkeyStr {
		t.Errorf("C field mismatch: saved %s, got %s", cPubkeyStr, retrievedCStr)
	}

	// Verify the Y (WrappedPublicKey) field matches
	if retrievedProof.Y.PublicKey == nil {
		t.Fatal("Y field (pubkey) should not be nil after retrieval")
	}
	retrievedYStr := hex.EncodeToString(retrievedProof.Y.SerializeCompressed())
	if retrievedYStr != yPubkeyStr {
		t.Errorf("Y field mismatch: saved %s, got %s", yPubkeyStr, retrievedYStr)
	}

	// Also verify other fields
	if retrievedProof.Amount != proof.Amount {
		t.Errorf("Amount mismatch: saved %d, got %d", proof.Amount, retrievedProof.Amount)
	}
	if retrievedProof.Secret != proof.Secret {
		t.Errorf("Secret mismatch: saved %s, got %s", proof.Secret, retrievedProof.Secret)
	}
}

func TestSaveProofAndGetBySecretCurve_ValidPubkey(t *testing.T) {
	db, ctx := setupTestDB(t)

	// Create a valid public key for the C field
	cPubkeyStr := "03d56ce4e446a85bbdaa547b4ec2b073d40ff802831352b8272b7dd7a4de5a7cac"
	cPubkeyBytes, err := hex.DecodeString(cPubkeyStr)
	if err != nil {
		t.Fatalf("could not decode C hex string. %v", err)
	}
	cPubkey, err := secp256k1.ParsePubKey(cPubkeyBytes)
	if err != nil {
		t.Fatalf("could not parse C pubkey bytes correctly. %v", err)
	}
	wrappedC := cashu.WrappedPublicKey{PublicKey: cPubkey}

	// Create a valid public key for the Y field (using a different key)
	yPubkeyStr := "02a9acc1e48c25eeeb9289b5031cc57da9fe72f3fe2861d264bdc074209b107ba2"
	yPubkeyBytes, err := hex.DecodeString(yPubkeyStr)
	if err != nil {
		t.Fatalf("could not decode Y hex string. %v", err)
	}
	yPubkey, err := secp256k1.ParsePubKey(yPubkeyBytes)
	if err != nil {
		t.Fatalf("could not parse Y pubkey bytes correctly. %v", err)
	}
	wrappedY := cashu.WrappedPublicKey{PublicKey: yPubkey}

	now := time.Now().Unix()
	secret := "test_secret_2"

	proof := cashu.Proof{
		Amount:  200,
		Id:      "test_keyset_id",
		Secret:  secret,
		C:       wrappedC,
		Y:       wrappedY,
		Witness: "",
		SeenAt:  now,
		State:   cashu.PROOF_UNSPENT,
		Quote:   nil,
	}

	// Save the proof
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatalf("could not get transaction. %v", err)
	}
	err = db.SaveProof(tx, []cashu.Proof{proof})
	if err != nil {
		t.Fatalf("db.SaveProof failed: %v", err)
	}
	err = tx.Commit(ctx)
	if err != nil {
		t.Fatalf("could not commit transaction. %v", err)
	}

	// Retrieve the proof by Y (secret curve) - pass the WrappedPublicKey
	tx, err = db.GetTx(ctx)
	if err != nil {
		t.Fatalf("could not get transaction. %v", err)
	}
	proofs, err := db.GetProofsFromSecretCurve(tx, []cashu.WrappedPublicKey{wrappedY})
	if err != nil {
		t.Fatalf("db.GetProofsFromSecretCurve failed: %v", err)
	}
	err = tx.Commit(ctx)
	if err != nil {
		t.Fatalf("could not commit transaction. %v", err)
	}

	// Verify we got exactly one proof
	if len(proofs) != 1 {
		t.Fatalf("expected 1 proof, got %d", len(proofs))
	}

	retrievedProof := proofs[0]

	// Verify the C (WrappedPublicKey) field matches
	if retrievedProof.C.PublicKey == nil {
		t.Fatal("C field (pubkey) should not be nil after retrieval")
	}
	retrievedCStr := hex.EncodeToString(retrievedProof.C.SerializeCompressed())
	if retrievedCStr != cPubkeyStr {
		t.Errorf("C field mismatch: saved %s, got %s", cPubkeyStr, retrievedCStr)
	}

	// Verify the Y (WrappedPublicKey) field matches
	if retrievedProof.Y.PublicKey == nil {
		t.Fatal("Y field (pubkey) should not be nil after retrieval")
	}
	retrievedYStr := hex.EncodeToString(retrievedProof.Y.SerializeCompressed())
	if retrievedYStr != yPubkeyStr {
		t.Errorf("Y field mismatch: saved %s, got %s", yPubkeyStr, retrievedYStr)
	}

	// Also verify other fields
	if retrievedProof.Amount != proof.Amount {
		t.Errorf("Amount mismatch: saved %d, got %d", proof.Amount, retrievedProof.Amount)
	}
}

func TestSaveRestoreSigsAndGet_ValidPubkeys(t *testing.T) {
	db, ctx := setupTestDB(t)

	// Create valid public keys for B_ and C_ fields (using different keys)
	b_PubkeyStr := "03d56ce4e446a85bbdaa547b4ec2b073d40ff802831352b8272b7dd7a4de5a7cac"
	b_PubkeyBytes, err := hex.DecodeString(b_PubkeyStr)
	if err != nil {
		t.Fatalf("could not decode B_ hex string. %v", err)
	}
	b_Pubkey, err := secp256k1.ParsePubKey(b_PubkeyBytes)
	if err != nil {
		t.Fatalf("could not parse B_ pubkey bytes correctly. %v", err)
	}
	wrappedB := cashu.WrappedPublicKey{PublicKey: b_Pubkey}

	// Use a different pubkey for C_
	c_PubkeyStr := "02a9acc1e48c25eeeb9289b5031cc57da9fe72f3fe2861d264bdc074209b107ba2"
	c_PubkeyBytes, err := hex.DecodeString(c_PubkeyStr)
	if err != nil {
		t.Fatalf("could not decode C_ hex string. %v", err)
	}
	c_Pubkey, err := secp256k1.ParsePubKey(c_PubkeyBytes)
	if err != nil {
		t.Fatalf("could not parse C_ pubkey bytes correctly. %v", err)
	}
	wrappedC := cashu.WrappedPublicKey{PublicKey: c_Pubkey}

	// Create DLEQ with valid private keys (using dummy 32-byte values)
	eBytes, _ := hex.DecodeString("0000000000000000000000000000000000000000000000000000000000000001")
	sBytes, _ := hex.DecodeString("0000000000000000000000000000000000000000000000000000000000000002")
	dleq := &cashu.BlindSignatureDLEQ{
		E: secp256k1.PrivKeyFromBytes(eBytes),
		S: secp256k1.PrivKeyFromBytes(sBytes),
	}

	now := time.Now().Unix()

	recoverSig := cashu.RecoverSigDB{
		Amount:    100,
		Id:        "test_keyset_id",
		B_:        wrappedB,
		C_:        wrappedC,
		CreatedAt: now,
		Dleq:      dleq,
	}

	// Save the recovery signature
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatalf("could not get transaction. %v", err)
	}
	err = db.SaveRestoreSigs(tx, []cashu.RecoverSigDB{recoverSig})
	if err != nil {
		t.Fatalf("db.SaveRestoreSigs failed: %v", err)
	}
	err = tx.Commit(ctx)
	if err != nil {
		t.Fatalf("could not commit transaction. %v", err)
	}

	// Retrieve the recovery signature by B_
	tx, err = db.GetTx(ctx)
	if err != nil {
		t.Fatalf("could not get transaction. %v", err)
	}
	sigs, err := db.GetRestoreSigsFromBlindedMessages(tx, []string{b_PubkeyStr})
	if err != nil {
		t.Fatalf("db.GetRestoreSigsFromBlindedMessages failed: %v", err)
	}
	err = tx.Commit(ctx)
	if err != nil {
		t.Fatalf("could not commit transaction. %v", err)
	}

	// Verify we got exactly one signature
	if len(sigs) != 1 {
		t.Fatalf("expected 1 recovery signature, got %d", len(sigs))
	}

	retrievedSig := sigs[0]

	// Verify the B_ (WrappedPublicKey) field matches
	if retrievedSig.B_.PublicKey == nil {
		t.Fatal("B_ field should not be nil after retrieval")
	}
	retrievedB_Str := hex.EncodeToString(retrievedSig.B_.SerializeCompressed())
	if retrievedB_Str != b_PubkeyStr {
		t.Errorf("B_ field mismatch: saved %s, got %s", b_PubkeyStr, retrievedB_Str)
	}

	// Verify the C_ (WrappedPublicKey) field matches
	if retrievedSig.C_.PublicKey == nil {
		t.Fatal("C_ field should not be nil after retrieval")
	}
	retrievedC_Str := hex.EncodeToString(retrievedSig.C_.SerializeCompressed())
	if retrievedC_Str != c_PubkeyStr {
		t.Errorf("C_ field mismatch: saved %s, got %s", c_PubkeyStr, retrievedC_Str)
	}

	// Verify other fields
	if retrievedSig.Amount != recoverSig.Amount {
		t.Errorf("Amount mismatch: saved %d, got %d", recoverSig.Amount, retrievedSig.Amount)
	}
	if retrievedSig.Id != recoverSig.Id {
		t.Errorf("Id mismatch: saved %s, got %s", recoverSig.Id, retrievedSig.Id)
	}
}

func TestSaveRestoreSigsAndGet_MultipleSigs(t *testing.T) {
	db, ctx := setupTestDB(t)

	// Create first signature with its own B_ and C_
	b1_PubkeyStr := "03d56ce4e446a85bbdaa547b4ec2b073d40ff802831352b8272b7dd7a4de5a7cac"
	b1_PubkeyBytes, _ := hex.DecodeString(b1_PubkeyStr)
	b1_Pubkey, _ := secp256k1.ParsePubKey(b1_PubkeyBytes)
	wrappedB1 := cashu.WrappedPublicKey{PublicKey: b1_Pubkey}

	c1_PubkeyStr := "02a9acc1e48c25eeeb9289b5031cc57da9fe72f3fe2861d264bdc074209b107ba2"
	c1_PubkeyBytes, _ := hex.DecodeString(c1_PubkeyStr)
	c1_Pubkey, _ := secp256k1.ParsePubKey(c1_PubkeyBytes)
	wrappedC1 := cashu.WrappedPublicKey{PublicKey: c1_Pubkey}

	// Create second signature with different B_ and C_
	b2_PubkeyStr := "0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"
	b2_PubkeyBytes, _ := hex.DecodeString(b2_PubkeyStr)
	b2_Pubkey, _ := secp256k1.ParsePubKey(b2_PubkeyBytes)
	wrappedB2 := cashu.WrappedPublicKey{PublicKey: b2_Pubkey}

	c2_PubkeyStr := "02c6047f9441ed7d6d3045406e95c07cd85c778e4b8cef3ca7abac09b95c709ee5"
	c2_PubkeyBytes, _ := hex.DecodeString(c2_PubkeyStr)
	c2_Pubkey, _ := secp256k1.ParsePubKey(c2_PubkeyBytes)
	wrappedC2 := cashu.WrappedPublicKey{PublicKey: c2_Pubkey}

	// Create DLEQ values
	eBytes, _ := hex.DecodeString("0000000000000000000000000000000000000000000000000000000000000001")
	sBytes, _ := hex.DecodeString("0000000000000000000000000000000000000000000000000000000000000002")
	dleq := &cashu.BlindSignatureDLEQ{
		E: secp256k1.PrivKeyFromBytes(eBytes),
		S: secp256k1.PrivKeyFromBytes(sBytes),
	}

	now := time.Now().Unix()

	sig1 := cashu.RecoverSigDB{
		Amount:    100,
		Id:        "keyset1",
		B_:        wrappedB1,
		C_:        wrappedC1,
		CreatedAt: now,
		Dleq:      dleq,
	}

	sig2 := cashu.RecoverSigDB{
		Amount:    200,
		Id:        "keyset2",
		B_:        wrappedB2,
		C_:        wrappedC2,
		CreatedAt: now,
		Dleq:      dleq,
	}

	// Save both recovery signatures
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatalf("could not get transaction. %v", err)
	}
	err = db.SaveRestoreSigs(tx, []cashu.RecoverSigDB{sig1, sig2})
	if err != nil {
		t.Fatalf("db.SaveRestoreSigs failed: %v", err)
	}
	err = tx.Commit(ctx)
	if err != nil {
		t.Fatalf("could not commit transaction. %v", err)
	}

	// Retrieve both signatures by their B_ values
	tx, err = db.GetTx(ctx)
	if err != nil {
		t.Fatalf("could not get transaction. %v", err)
	}
	sigs, err := db.GetRestoreSigsFromBlindedMessages(tx, []string{b1_PubkeyStr, b2_PubkeyStr})
	if err != nil {
		t.Fatalf("db.GetRestoreSigsFromBlindedMessages failed: %v", err)
	}
	err = tx.Commit(ctx)
	if err != nil {
		t.Fatalf("could not commit transaction. %v", err)
	}

	// Verify we got exactly two signatures
	if len(sigs) != 2 {
		t.Fatalf("expected 2 recovery signatures, got %d", len(sigs))
	}

	// Check each signature
	var foundSig1, foundSig2 bool
	for _, sig := range sigs {
		b_Str := hex.EncodeToString(sig.B_.SerializeCompressed())
		c_Str := hex.EncodeToString(sig.C_.SerializeCompressed())

		switch b_Str {
		case b1_PubkeyStr:
			foundSig1 = true
			if c_Str != c1_PubkeyStr {
				t.Errorf("Sig1 C_ mismatch: expected %s, got %s", c1_PubkeyStr, c_Str)
			}
			if sig.Amount != 100 {
				t.Errorf("Sig1 Amount mismatch: expected 100, got %d", sig.Amount)
			}
		case b2_PubkeyStr:
			foundSig2 = true
			if c_Str != c2_PubkeyStr {
				t.Errorf("Sig2 C_ mismatch: expected %s, got %s", c2_PubkeyStr, c_Str)
			}
			if sig.Amount != 200 {
				t.Errorf("Sig2 Amount mismatch: expected 200, got %d", sig.Amount)
			}
		}
	}

	if !foundSig1 {
		t.Error("Did not find signature 1")
	}
	if !foundSig2 {
		t.Error("Did not find signature 2")
	}
}

func setupTestDB(t *testing.T) (Postgresql, context.Context) {
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
	t.Cleanup(func() {
		if err := postgresContainer.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	})

	connUri, err := postgresContainer.ConnectionString(ctx)
	if err != nil {
		t.Fatal(fmt.Errorf("failed to get connection string: %v", err))
	}

	t.Setenv("DATABASE_URL", connUri)

	db, err := DatabaseSetup(ctx, "migrations")
	if err != nil {
		t.Fatalf("could not setup migration. %v", err)
	}

	return db, ctx
}

func ptr(v uint64) *uint64 { return &v }
