package database

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
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

type AuthUser struct {
	Sub string `db:"sub"`
	Aud *string `db:"aud"`
	LastLoggedIn uint64 `db:"last_logged_in"`
}


var DBError = errors.New("ERROR DATABASE")

var DATABASE_URL_ENV = "DATABASE_URL"

const (
	DOCKERDATABASE = "DOCKERDATABASE"
	CUSTOMDATABASE = "CUSTOMDATABASE"
)

type MintDB interface {
	GetTx(ctx context.Context) (pgx.Tx, error)
	Commit(ctx context.Context, tx pgx.Tx) error
	Rollback(ctx context.Context, tx pgx.Tx) error
	SubTx(ctx context.Context, tx pgx.Tx) (pgx.Tx, error)

	/// Calls for the Functioning of the mint
	GetAllSeeds() ([]cashu.Seed, error)
	GetSeedsByUnit(unit cashu.Unit) ([]cashu.Seed, error)
	SaveNewSeed(seed cashu.Seed) error
	SaveNewSeeds(seeds []cashu.Seed) error
	// This should be used to only update the Active Status of seed on the db
	UpdateSeedsActiveStatus(seeds []cashu.Seed) error

	SaveMintRequest(tx pgx.Tx, request cashu.MintRequestDB) error
	ChangeMintRequestState(tx pgx.Tx, quote string, paid bool, state cashu.ACTION_STATE, minted bool) error
	GetMintRequestById(tx pgx.Tx, quote string) (cashu.MintRequestDB, error)
	GetMintRequestByRequest(tx pgx.Tx, request string) (cashu.MintRequestDB, error)

	GetMeltRequestById(tx pgx.Tx, quote string) (cashu.MeltRequestDB, error)
	SaveMeltRequest(tx pgx.Tx, request cashu.MeltRequestDB) error
	ChangeMeltRequestState(tx pgx.Tx, quote string, paid bool, state cashu.ACTION_STATE, melted bool, fee_paid uint64) error
	AddPreimageMeltRequest(tx pgx.Tx, quote string, preimage string) error

	GetMeltQuotesByState(state cashu.ACTION_STATE) ([]cashu.MeltRequestDB, error)

	SaveProof(tx pgx.Tx, proofs []cashu.Proof) error
	GetProofsFromSecret(tx pgx.Tx, SecretList []string) (cashu.Proofs, error)
	GetProofsFromSecretCurve(tx pgx.Tx, Ys []string) (cashu.Proofs, error)
	GetProofsFromQuote(tx pgx.Tx, quote string) (cashu.Proofs, error)
	SetProofsState(tx pgx.Tx, proofs cashu.Proofs, state cashu.ProofState) error
	DeleteProofs(tx pgx.Tx, proofs cashu.Proofs) error

	GetRestoreSigsFromBlindedMessages(tx pgx.Tx, B_ []string) ([]cashu.RecoverSigDB, error)
	SaveRestoreSigs(tx pgx.Tx, recover_sigs []cashu.RecoverSigDB) error

	GetProofsMintReserve() (templates.MintReserve, error)
	GetBlindSigsMintReserve() (templates.MintReserve, error)

	GetConfig() (utils.Config, error)
	SetConfig(config utils.Config) error
	UpdateConfig(config utils.Config) error

	SaveMeltChange(tx pgx.Tx, change []cashu.BlindedMessage, quote string) error
	GetMeltChangeByQuote(tx pgx.Tx, quote string) ([]cashu.MeltChange, error)
	DeleteChangeByQuote(tx pgx.Tx, quote string) error

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


	// Mint Auth 
	GetAuthUser(tx pgx.Tx,sub string) (AuthUser, error)
	MakeAuthUser(tx pgx.Tx,auth AuthUser)  error
	UpdateLastLoggedIn(tx pgx.Tx, sub string, lastLoggedIn uint64)  error

	// liquidity provider state
}
