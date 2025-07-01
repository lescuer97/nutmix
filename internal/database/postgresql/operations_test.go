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
		Pubkey:      pubkey,
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

	if hex.EncodeToString(mintRequest.Pubkey.SerializeCompressed()) != "03d56ce4e446a85bbdaa547b4ec2b073d40ff802831352b8272b7dd7a4de5a7cac" {
		t.Errorf("pubkey from mint request is not correct. %x", mintRequest.Pubkey.SerializeCompressed())
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
		Pubkey:      nil,
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

	if mintRequest.Pubkey != nil {
		t.Errorf("pubkey should be nil. %v", mintRequest.Pubkey)
	}

	err = tx.Commit(ctx)
	if err != nil {
		t.Fatalf("could not commit transaction. %v", err)
	}
}
