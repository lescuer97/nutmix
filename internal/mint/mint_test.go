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
	"github.com/lescuer97/nutmix/internal/lightning"
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

const quoteId = "quoteid"

func SetupDataOnDB(mint *Mint) error {
	now := time.Now().Unix()

	melt_quote := cashu.MeltRequestDB{
		Quote:       quoteId,
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
			Amount: 2,
			Id:     "00bfa73302d12ffd",
			Secret: "secret1",
			C:      "c1",
			Y:      "y1",
			SeenAt: now,
			State:  cashu.PROOF_PENDING,
			Quote:  &melt_quote.Quote,
		},
		cashu.Proof{
			Amount: 2,
			Id:     "00bfa73302d12ffd",
			Secret: "secret2",
			C:      "c2",
			Y:      "y2",
			SeenAt: now,
			State:  cashu.PROOF_PENDING,
			Quote:  &melt_quote.Quote,
		},
	}
	// Sets proofs and quotes to pending
	ctx := context.Background()

	tx, err := mint.MintDB.GetTx(ctx)
	if err != nil {
		return fmt.Errorf("mint.MintDB.GetTx(ctx): %+v ", err)
	}
	defer mint.MintDB.Rollback(ctx, tx)

	err = mint.MintDB.SaveMeltRequest(tx, melt_quote)
	if err != nil {
		return fmt.Errorf("mint.MintDB.SaveMeltRequest(tx, melt_quote): %+v ", err)
	}

	err = mint.MintDB.SaveProof(tx, proofs)
	if err != nil {
		return fmt.Errorf("mint.MintDB.SaveProof(tx, proofs): %+v ", err)
	}
	//
	err = mint.MintDB.Commit(ctx, tx)
	if err != nil {
		return fmt.Errorf("mint.MintDB.Commit(ctx, tx): %+v ", err)
	}

	return nil
}

// should succeed and quote should be success and proofs as spent
func TestPendingQuotesAndProofsWithPostgresAndMockLNSuccess(t *testing.T) {
	mint := SetupMintWithLightningMockPostgres(t)
	err := SetupDataOnDB(mint)
	if err != nil {
		t.Fatalf("SetupDataOnDB(mint): %+v ", err)
	}

	meltRequest, err := mint.CheckMeltQuoteState(quoteId)
	if err != nil {
		t.Fatalf("mint.CheckMeltQuoteState(quoteId): %+v ", err)
	}

	if meltRequest.Quote != quoteId {
		t.Errorf("melt quote id: %+v ", meltRequest.Quote)
	}
	if meltRequest.State != cashu.PAID {
		t.Errorf("State should be paid: %+v ", meltRequest.State)
	}

	ctx := context.Background()
	tx, err := mint.MintDB.GetTx(ctx)
	if err != nil {
		t.Fatalf("mint.MintDB.GetTx(ctx): %+v ", err)
	}
	defer mint.MintDB.Rollback(ctx, tx)

	savedQuote, err := mint.MintDB.GetMeltRequestById(tx, meltRequest.Quote)
	if err != nil {
		t.Fatalf("mint.MintDB.GetMeltChangeByQuote(tx, meltRequest.Quote): %+v ", err)
	}
	if savedQuote.Quote != quoteId {
		t.Errorf("melt quote id: %+v ", meltRequest.Quote)
	}
	if savedQuote.State != cashu.PAID {
		t.Errorf("melt quote id: %+v ", meltRequest.Quote)
	}

	meltChange, err := mint.MintDB.GetMeltChangeByQuote(tx, meltRequest.Quote)
	if err != nil {
		t.Fatalf("mint.MintDB.GetMeltChangeByQuote(tx, meltRequest.Quote): %+v ", err)
	}

	if len(meltChange) > 0 {
		t.Errorf("\n There should be 0 change.  %+v \n ", len(meltChange))
	}

	savedProofs, err := mint.MintDB.GetProofsFromQuote(tx, meltRequest.Quote)
	if err != nil {
		t.Fatalf("mint.MintDB.GetMeltChangeByQuote(tx, meltRequest.Quote): %+v ", err)
	}

	totalProof := 0
	for _, proof := range savedProofs {
		totalProof += int(proof.Amount)
		if proof.State != cashu.PROOF_SPENT {
			t.Errorf("\n Proof should be spent. %+v\n", proof)
		}
	}
	if totalProof != 4 {
		t.Errorf("\n Proof amount are not correc. %+v\n", totalProof)
	}

	err = mint.MintDB.Commit(ctx, tx)
	if err != nil {
		t.Fatalf("mint.MintDB.Commit(ctx, tx): %+v ", err)
	}

}
func TestPendingQuotesAndProofsWithPostgresAndMockLNFail(t *testing.T) {
	mint := SetupMintWithLightningMockPostgres(t)

	lightning := lightning.FakeWallet{
		Network: *mint.LightningBackend.GetNetwork(),
		UnpurposeErrors: []lightning.FakeWalletError{
			lightning.FailQueryFailed,
		},
	}
	mint.LightningBackend = lightning

	err := SetupDataOnDB(mint)
	if err != nil {
		t.Fatalf("SetupDataOnDB(mint): %+v ", err)
	}

	meltRequest, err := mint.CheckMeltQuoteState(quoteId)
	if err != nil {
		t.Fatalf("mint.CheckMeltQuoteState(quoteId): %+v ", err)
	}

	if meltRequest.Quote != quoteId {
		t.Errorf("melt quote id: %+v ", meltRequest.Quote)
	}
	if meltRequest.State != cashu.UNPAID {
		t.Errorf("State should be paid: %+v ", meltRequest.State)
	}

	ctx := context.Background()
	tx, err := mint.MintDB.GetTx(ctx)
	if err != nil {
		t.Fatalf("mint.MintDB.GetTx(ctx): %+v ", err)
	}
	defer tx.Rollback(ctx)

	savedQuote, err := mint.MintDB.GetMeltRequestById(tx, meltRequest.Quote)
	if err != nil {
		t.Fatalf("mint.MintDB.GetMeltChangeByQuote(tx, meltRequest.Quote): %+v ", err)
	}
	if savedQuote.Quote != quoteId {
		t.Errorf("melt quote id: %+v ", meltRequest.Quote)
	}
	if savedQuote.State != cashu.UNPAID {
		t.Errorf("melt quote id: %+v ", meltRequest.Quote)
	}

	meltChange, err := mint.MintDB.GetMeltChangeByQuote(tx, meltRequest.Quote)
	if err != nil {
		t.Fatalf("mint.MintDB.GetMeltChangeByQuote(tx, meltRequest.Quote): %+v ", err)
	}

	if len(meltChange) != 0 {
		t.Errorf("\n There should be 0 change.  %+v \n ", len(meltChange))
	}

	savedProofs, err := mint.MintDB.GetProofsFromQuote(tx, meltRequest.Quote)
	if err != nil {
		t.Fatalf("mint.MintDB.GetMeltChangeByQuote(tx, meltRequest.Quote): %+v ", err)
	}

	if len(savedProofs) > 0 {
		t.Errorf("\n There should not be any proofs. %+v\n", savedProofs)
	}
	err = mint.MintDB.Commit(ctx, tx)
	if err != nil {
		t.Fatalf("mint.MintDB.Commit(ctx, tx): %+v ", err)
	}
}

func TestPendingQuotesAndProofsWithPostgresAndMockLNPending(t *testing.T) {
	mint := SetupMintWithLightningMockPostgres(t)

	lightning := lightning.FakeWallet{
		Network: *mint.LightningBackend.GetNetwork(),
		UnpurposeErrors: []lightning.FakeWalletError{
			lightning.FailQueryPending,
		},
	}
	mint.LightningBackend = lightning

	err := SetupDataOnDB(mint)
	if err != nil {
		t.Fatalf("SetupDataOnDB(mint): %+v ", err)
	}

	meltRequest, err := mint.CheckMeltQuoteState(quoteId)
	if err != nil {
		t.Fatalf("mint.CheckMeltQuoteState(quoteId): %+v ", err)
	}

	if meltRequest.Quote != quoteId {
		t.Errorf("melt quote id: %+v ", meltRequest.Quote)
	}
	if meltRequest.State != cashu.PENDING {
		t.Errorf("State should be paid: %+v ", meltRequest.State)
	}

	ctx := context.Background()
	tx, err := mint.MintDB.GetTx(ctx)
	if err != nil {
		t.Fatalf("mint.MintDB.GetTx(ctx): %+v ", err)
	}
	defer mint.MintDB.Rollback(ctx, tx)

	savedQuote, err := mint.MintDB.GetMeltRequestById(tx, meltRequest.Quote)
	if err != nil {
		t.Fatalf("mint.MintDB.GetMeltChangeByQuote(tx, meltRequest.Quote): %+v ", err)
	}
	if savedQuote.Quote != quoteId {
		t.Errorf("melt quote id: %+v ", meltRequest.Quote)
	}
	if savedQuote.State != cashu.PENDING {
		t.Errorf("melt quote id: %+v ", meltRequest.Quote)
	}

	meltChange, err := mint.MintDB.GetMeltChangeByQuote(tx, meltRequest.Quote)
	if err != nil {
		t.Fatalf("mint.MintDB.GetMeltChangeByQuote(tx, meltRequest.Quote): %+v ", err)
	}

	if len(meltChange) > 0 {
		t.Errorf("\n There should be 0 change.  %+v \n ", len(meltChange))
	}

	savedProofs, err := mint.MintDB.GetProofsFromQuote(tx, meltRequest.Quote)
	if err != nil {
		t.Fatalf("mint.MintDB.GetMeltChangeByQuote(tx, meltRequest.Quote): %+v ", err)
	}

	totalProof := 0
	for _, proof := range savedProofs {
		totalProof += int(proof.Amount)
		if proof.State != cashu.PROOF_PENDING {
			t.Errorf("\n Proof should be spent. %+v\n", proof)
		}
	}
	if totalProof != 4 {
		t.Errorf("\n Proof amount are not correc. %+v\n", totalProof)
	}

	err = mint.MintDB.Commit(ctx, tx)
	if err != nil {
		t.Fatalf("mint.MintDB.Commit(ctx, tx): %+v ", err)
	}
}
