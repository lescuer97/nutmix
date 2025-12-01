package mint

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lescuer97/nutmix/api/cashu"
	pq "github.com/lescuer97/nutmix/internal/database/postgresql"
	"github.com/lescuer97/nutmix/internal/lightning"
	localsigner "github.com/lescuer97/nutmix/internal/signer/local_signer"
	"github.com/lescuer97/nutmix/internal/utils"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const MintPrivateKey string = "0000000000000000000000000000000000000000000000000000000000000001"

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

	t.Setenv("MINT_PRIVATE_KEY", MintPrivateKey)
	connUri, err := postgresContainer.ConnectionString(ctx)

	if err != nil {
		t.Fatal(fmt.Errorf("failed to get connection string: %w", err))
	}
	t.Setenv("DATABASE_URL", connUri)

	db, err := pq.DatabaseSetup(ctx, "../../migrations/")
	if err != nil {
		t.Fatal("Error conecting to db", err)
	}

	signer, err := localsigner.SetupLocalSigner(db)
	if err != nil {
		t.Fatalf("localsigner.SetupLocalSigner(db): %v", err)
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

	mint, err := SetUpMint(ctx, config, db, &signer)

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
		Request:     "lnbcrt2u1pna2hrlpp5gv4edjsvjzflaxex5y4jcm87yhhm7s6clt6hjar50yhswan83fesdqqcqzzsxqzuysp5u3kq8etcat22w2hraktrgppltaegt3prrf5qz9z4cplreje2kzrq9qxpqysgq2ujupalzlwz9nhn55pl6nuwtv4qqkdlkn02rf835l3janjwy7w3n0tl0whh6v3frpvfcsyzud3g6dsx6gqgmw04xj2c0alz4px5hjecq0pnclr",
		State:       cashu.PENDING,
		Melted:      false,
		SeenAt:      time.Now().Unix(),
		Mpp:         false,
	}

	c1Priv, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		return fmt.Errorf("secp256k1.GeneratePrivateKey: %+v ", err)
	}
	c1 := c1Priv.PubKey()
	c2Priv, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		return fmt.Errorf("secp256k1.GeneratePrivateKey: %+v ", err)
	}

	c2 := c2Priv.PubKey()
	proofs := cashu.Proofs{
		cashu.Proof{
			Amount: 2,
			Id:     "00bfa73302d12ffd",
			Secret: "secret1",
			C:      cashu.WrappedPublicKey{PublicKey: c1},
			SeenAt: now,
			State:  cashu.PROOF_PENDING,
			Quote:  &melt_quote.Quote,
		},
		cashu.Proof{
			Amount: 2,
			Id:     "00bfa73302d12ffd",
			Secret: "secret2",
			C:      cashu.WrappedPublicKey{PublicKey: c2},
			SeenAt: now,
			State:  cashu.PROOF_PENDING,
			Quote:  &melt_quote.Quote,
		},
	}

	for i := range proofs {
		p, err := proofs[i].HashSecretToCurve()
		if err != nil {
			return fmt.Errorf("proofs[p].HashSecretToCurve()")
		}
		proofs[i] = p
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
	defer mint.MintDB.Rollback(ctx, tx)

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
