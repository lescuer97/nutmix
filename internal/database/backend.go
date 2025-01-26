package database

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/utils"
)

type MintMeltBalance struct {
	Mint []cashu.MintRequestDB
	Melt []cashu.MeltRequestDB
}
type NostrLoginAuth struct {
	Nonce     string
	Activated bool
	Expiry    int
}

var DBError = errors.New("ERROR DATABASE")

var DATABASE_URL_ENV = "DATABASE_URL"

const (
	DOCKERDATABASE = "DOCKERDATABASE"
	CUSTOMDATABASE = "CUSTOMDATABASE"
)

type MintDB interface {
	GetTx(ctx context.Context) (pgx.Tx, error)

	/// Calls for the Functioning of the mint
	GetAllSeeds() ([]cashu.Seed, error)
	GetSeedsByUnit(unit cashu.Unit) ([]cashu.Seed, error)
	SaveNewSeed(seed cashu.Seed) error
	SaveNewSeeds(seeds []cashu.Seed) error
	// This should be used to only update the Active Status of seed on the db
	UpdateSeedsActiveStatus(seeds []cashu.Seed) error

	SaveMintRequest(request cashu.MintRequestDB) error
	ChangeMintRequestState(quote string, paid bool, state cashu.ACTION_STATE, minted bool) error
	GetMintRequestById(quote string) (cashu.MintRequestDB, error)

	GetMeltRequestById(quote string) (cashu.MeltRequestDB, error)
	SaveMeltRequest(request cashu.MeltRequestDB) error
	ChangeMeltRequestState(quote string, paid bool, state cashu.ACTION_STATE, melted bool) error
	AddPreimageMeltRequest(quote string, preimage string) error

	SaveProof(proofs []cashu.Proof) error
	GetProofsFromSecret(SecretList []string) ([]cashu.Proof, error)
	GetProofsFromSecretCurve(Ys []string) ([]cashu.Proof, error)

	GetRestoreSigsFromBlindedMessages(B_ []string) ([]cashu.RecoverSigDB, error)
	SaveRestoreSigs(recover_sigs []cashu.RecoverSigDB) error

	GetConfig() (utils.Config, error)
	SetConfig(config utils.Config) error
	UpdateConfig(config utils.Config) error

	/// Calls for the admin dashboard

	GetMintMeltBalanceByTime(time int64) (MintMeltBalance, error)

	SaveNostrAuth(auth NostrLoginAuth) error
	UpdateNostrAuthActivation(tx pgx.Tx, nonce string, activated bool) error
	GetNostrAuth(tx pgx.Tx, nonce string) (NostrLoginAuth, error)

	// liquidity swaps
	AddLiquiditySwap(tx pgx.Tx, swap utils.LiquiditySwap) error
	GetLiquiditySwapById(tx pgx.Tx, id string) (utils.LiquiditySwap, error)
	ChangeLiquiditySwapState(tx pgx.Tx, id string, state utils.SwapState) error
	GetAllLiquiditySwaps() ([]utils.LiquiditySwap, error)
	GetLiquiditySwapsByStates(states []utils.SwapState) ([]utils.LiquiditySwap, error)

	// liquidity provider state
}
