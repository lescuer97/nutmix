package mint

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lescuer97/nutmix/api/cashu"
	pq "github.com/lescuer97/nutmix/internal/database/postgresql"
	"github.com/lescuer97/nutmix/internal/utils"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const MintPrivateKey string = "0000000000000000000000000000000000000000000000000000000000000001"

func TestCreateNewSeed(t *testing.T) {
	decodedPrivKey, err := hex.DecodeString(MintPrivateKey)
	if err != nil {
		t.Fatal("hex.DecodeString(mint_privkey)")
	}

	parsedPrivateKey := secp256k1.PrivKeyFromBytes(decodedPrivKey)
	masterKey, err := MintPrivateKeyToBip32(parsedPrivateKey)
	if err != nil {
		t.Fatal("mint.MintPrivateKeyToBip32(parsedPrivateKey)")
	}

	seed1, err := CreateNewSeed(masterKey, 1, 0)
	if err != nil {
		t.Fatal("CreateNewSeed(masterKey, 1, 0)")
	}

	if seed1.Id != "00bfa73302d12ffd" {
		t.Errorf("seed id incorrect. %v", seed1.Id)

	}

}
func TestGeneratedKeysetsMakeTheCorrectIds(t *testing.T) {
	decodedPrivKey, err := hex.DecodeString(MintPrivateKey)
	if err != nil {
		t.Fatal("hex.DecodeString(mint_privkey)")
	}

	parsedPrivateKey := secp256k1.PrivKeyFromBytes(decodedPrivKey)
	masterKey, err := MintPrivateKeyToBip32(parsedPrivateKey)
	if err != nil {
		t.Fatal("mint.MintPrivateKeyToBip32(parsedPrivateKey)")
	}
	seed1, err := CreateNewSeed(masterKey, 1, 0)
	if err != nil {
		t.Fatal("CreateNewSeed(masterKey, 1, 0)")
	}

	keyset, err := DeriveKeyset(masterKey, seed1)
	if err != nil {
		t.Fatal("DeriveKeyset(masterKey,seed1 )")
	}
	newSeedId, err := cashu.DeriveKeysetId(keyset)
	if err != nil {
		t.Fatal("cashu.DeriveKeysetId(keyset)")
	}

	if newSeedId != "00bfa73302d12ffd" {
		t.Errorf("seed id incorrect. %v", seed1.Id)

	}

}

func SetupMintWithLightningMockPostgres(t *testing.T) *Mint {
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

	db, err := pq.DatabaseSetup(ctx, "../../migrations/")
	if err != nil {
		t.Fatal("Error conecting to db", err)
	}

	seeds, err := db.GetAllSeeds()

	if err != nil {
		t.Fatalf("Could not keysets: %v", err)
	}

	decodedPrivKey, err := hex.DecodeString(MintPrivateKey)
	if err != nil {
		t.Fatalf("hex.DecodeString(mint_privkey): %+v ", err)
	}

	parsedPrivateKey := secp256k1.PrivKeyFromBytes(decodedPrivKey)
	masterKey, err := MintPrivateKeyToBip32(parsedPrivateKey)
	if err != nil {
		t.Fatalf("mint.MintPrivateKeyToBip32(parsedPrivateKey): %+v ", err)
	}
	// incase there are no seeds in the db we create a new one
	if len(seeds) == 0 {

		seed, err := CreateNewSeed(masterKey, 1, 0)
		if err != nil {
			t.Fatalf("mint.CreateNewSeed(masterKey, 1, 0) %+v ", err)
		}

		err = db.SaveNewSeeds([]cashu.Seed{seed})

		seeds = append(seeds, seed)

		if err != nil {
			t.Fatalf("SaveNewSeed: %+v ", err)
		}

	}

	config, err := SetUpConfigDB(db)

	config.MINT_LIGHTNING_BACKEND = utils.FAKE_WALLET

	config.NETWORK = "regtest"
	config.LND_GRPC_HOST = os.Getenv(utils.LND_HOST)
	config.LND_TLS_CERT = os.Getenv(utils.LND_TLS_CERT)
	config.LND_MACAROON = os.Getenv(utils.LND_MACAROON)
	config.MINT_LNBITS_KEY = os.Getenv(utils.MINT_LNBITS_KEY)
	config.MINT_LNBITS_ENDPOINT = os.Getenv(utils.MINT_LNBITS_ENDPOINT)

	if err != nil {
		t.Fatalf("could not setup config file: %+v ", err)
	}

	mint, err := SetUpMint(ctx, parsedPrivateKey, seeds, config, db)

	if err != nil {
		t.Fatalf("SetUpMint: %+v ", err)
	}

	return mint

}

// should succeed and quote should be success and proofs as spent
func TestPendingQuotesAndProofsWithPostgresAndMockLNSuccess(t *testing.T) {
	mint := SetupMintWithLightningMockPostgres(t)

    now := time.Now().Unix()

	melt_quote := cashu.MeltRequestDB{
		Quote:       "quoteid",
		Unit:        cashu.Sat.String(),
		Expiry:      now,
		Amount:      2,
		FeeReserve:  2,
		RequestPaid: false,
		Request:     "rest",
		State:       cashu.PENDING,
		Melted:      false,
		SeenAt:      time.Now().Unix(),
		Mpp:         false,
	}

	proofs := cashu.Proofs{
		cashu.Proof{
			Amount:  2,
			Id:      "test",
			Secret:  "secret1",
			C:       "c1",
			Y:       "y1",
			Witness: "",
			SeenAt:  now,
			State:   cashu.PROOF_PENDING,
			Quote:   &melt_quote.Quote,
		},
		cashu.Proof{
			Amount:  2,
			Id:      "test",
			Secret:  "secret2",
			Witness: "",
			C:       "c2",
			Y:       "y2",
			SeenAt:  now,
			State:   cashu.PROOF_PENDING,
			Quote:   &melt_quote.Quote,
		},
	}
	// Sets proofs and quotes to pending
	ctx := context.Background()

	tx, err := mint.MintDB.GetTx(ctx)
	if err != nil {
		t.Fatalf("mint.MintDB.GetTx(ctx): %+v ", err)
	}
	defer tx.Rollback(ctx)

	err = mint.MintDB.SaveMeltRequest(tx, melt_quote)
	if err != nil {
		t.Fatalf("mint.MintDB.SaveMeltRequest(tx, melt_quote): %+v ", err)
	}

	err = mint.MintDB.SaveProof(tx, proofs)
	if err != nil {
		t.Fatalf("mint.MintDB.SaveProof(tx, proofs): %+v ", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		t.Fatalf("tx.Commit(ctx): %+v ", err)
	}

	meltRequest, err := mint.CheckMeltQuoteState(melt_quote.Quote)
	if err != nil {
		t.Fatalf("mint.CheckMeltQuoteState(melt_quote.Quote): %+v ", err)
	}

	if meltRequest.State != cashu.PAID {
		t.Errorf("State should be paid: %+v ", meltRequest.State)

	}

	/// store pending proofs and quotes
	// mint.MintDB.

	//

}
