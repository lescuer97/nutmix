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
	tx.Commit(ctx)

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
	tx.Commit(ctx)

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
	tx.Commit(ctx)

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
	tx.Commit(ctx)

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
		if m.Quote == "mintWithKey" {
			foundWithKey = true
			if m.Pubkey.PublicKey == nil {
				t.Error("Expected mintWithKey to have a pubkey")
			} else if hex.EncodeToString(m.Pubkey.SerializeCompressed()) != pubkeyStr {
				t.Errorf("Pubkey mismatch. Got %x, expected %s", m.Pubkey.SerializeCompressed(), pubkeyStr)
			}
		} else if m.Quote == "mintNoKey" {
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
	secret := "test_secret_1"
	yValue := "test_y_value_1"

	proof := cashu.Proof{
		Amount:  100,
		Id:      "test_keyset_id",
		Secret:  secret,
		C:       wrappedPubkey,
		Y:       yValue,
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

	// Verify the C (WrappedPublicKey) field matches
	retrievedProof := proofs[0]
	if retrievedProof.C.PublicKey == nil {
		t.Fatal("C field (pubkey) should not be nil after retrieval")
	}
	retrievedPubkeyStr := hex.EncodeToString(retrievedProof.C.SerializeCompressed())
	if retrievedPubkeyStr != pubkeyStr {
		t.Errorf("C field mismatch: saved %s, got %s", pubkeyStr, retrievedPubkeyStr)
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
	secret := "test_secret_2"
	yValue := "test_y_value_2"

	proof := cashu.Proof{
		Amount:  200,
		Id:      "test_keyset_id",
		Secret:  secret,
		C:       wrappedPubkey,
		Y:       yValue,
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

	// Retrieve the proof by Y (secret curve)
	tx, err = db.GetTx(ctx)
	if err != nil {
		t.Fatalf("could not get transaction. %v", err)
	}
	proofs, err := db.GetProofsFromSecretCurve(tx, []string{yValue})
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

	// Verify the C (WrappedPublicKey) field matches
	retrievedProof := proofs[0]
	if retrievedProof.C.PublicKey == nil {
		t.Fatal("C field (pubkey) should not be nil after retrieval")
	}
	retrievedPubkeyStr := hex.EncodeToString(retrievedProof.C.SerializeCompressed())
	if retrievedPubkeyStr != pubkeyStr {
		t.Errorf("C field mismatch: saved %s, got %s", pubkeyStr, retrievedPubkeyStr)
	}

	// Also verify other fields
	if retrievedProof.Amount != proof.Amount {
		t.Errorf("Amount mismatch: saved %d, got %d", proof.Amount, retrievedProof.Amount)
	}
	if retrievedProof.Y != proof.Y {
		t.Errorf("Y mismatch: saved %s, got %s", proof.Y, retrievedProof.Y)
	}
}

func setupTestDB(t *testing.T) (Postgresql, context.Context) {
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
