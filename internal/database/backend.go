package database

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/utils"
)

type NostrLoginAuth struct {
	Nonce     string
	Activated bool
	Expiry    int
}

type AuthUser struct {
	Aud          *string `db:"aud"`
	Sub          string  `db:"sub"`
	LastLoggedIn uint64  `db:"last_logged_in"`
}

var ErrDB = errors.New("ERROR DATABASE")

var DATABASE_URL_ENV = "DATABASE_URL"

const (
	DOCKERDATABASE = "DOCKERDATABASE"
	CUSTOMDATABASE = "CUSTOMDATABASE"
)

// ProofTimeSeriesPoint represents a single data point for charting proofs over time
type ProofTimeSeriesPoint struct {
	Timestamp   int64  `json:"timestamp"`   // Unix timestamp (seconds) for the bucket start
	TotalAmount uint64 `json:"totalAmount"` // Sum of proof amounts in this bucket
	Count       uint64 `json:"count"`       // Number of proofs in this bucket
}

type StatsSummaryItem struct {
	Unit     string `json:"unit"`
	Quantity uint64 `json:"quantity"`
	Amount   uint64 `json:"amount"`
}

type StatsSnapshot struct {
	MintSummary      []StatsSummaryItem `db:"mint_summary"`
	MeltSummary      []StatsSummaryItem `db:"melt_summary"`
	BlindSigsSummary []StatsSummaryItem `db:"blind_sigs_summary"`
	ProofsSummary    []StatsSummaryItem `db:"proofs_summary"`
	ID               int64              `db:"id"`
	StartDate        int64              `db:"start_date"`
	EndDate          int64              `db:"end_date"`
	Fees             uint64             `db:"fees"`
}

type MintStatsRow struct {
	Quote   string
	Unit    string
	Amount  *uint64
	Request string
}

type MeltStatsRow struct {
	Quote  string
	Unit   string
	Amount uint64
}

type KeysetStatsRow struct {
	KeysetID string
	Unit     string
	Amount   uint64
}

type KeysetFeeRow struct {
	KeysetID    string
	Unit        string
	Quantity    uint64
	InputFeePpk uint64
}

type LightningActivityRow struct {
	ID      string
	Type    string
	Request string
	State   string
	Unit    string
	SeenAt  int64
}

type MintDB interface {
	GetTx(ctx context.Context) (pgx.Tx, error)
	Commit(ctx context.Context, tx pgx.Tx) error
	Rollback(ctx context.Context, tx pgx.Tx) error

	/// Calls for the Functioning of the mint
	GetAllSeeds() ([]cashu.Seed, error)
	GetSeedsByUnit(tx pgx.Tx, unit cashu.Unit) ([]cashu.Seed, error)
	SaveNewSeed(tx pgx.Tx, seed cashu.Seed) error
	SaveNewSeeds(seeds []cashu.Seed) error
	// This should be used to only update the Active Status of seed on the db
	UpdateSeedsActiveStatus(tx pgx.Tx, seeds []cashu.Seed) error

	SaveMintRequest(tx pgx.Tx, request cashu.MintRequestDB) error
	ChangeMintRequestState(tx pgx.Tx, quote string, state cashu.ACTION_STATE, minted bool) error
	GetMintRequestById(tx pgx.Tx, quote string) (cashu.MintRequestDB, error)
	GetMintRequestByRequest(tx pgx.Tx, request string) (cashu.MintRequestDB, error)

	GetMeltRequestById(tx pgx.Tx, quote string) (cashu.MeltRequestDB, error)
	SaveMeltRequest(tx pgx.Tx, request cashu.MeltRequestDB) error
	ChangeMeltRequestState(tx pgx.Tx, quote string, state cashu.ACTION_STATE, melted bool, fee_paid uint64) error
	AddPreimageMeltRequest(tx pgx.Tx, quote string, preimage string) error
	ChangeCheckingId(tx pgx.Tx, quote string, checking_id string) error

	GetMeltQuotesByState(state cashu.ACTION_STATE) ([]cashu.MeltRequestDB, error)

	SaveProof(tx pgx.Tx, proofs []cashu.Proof) error
	GetProofsFromSecret(tx pgx.Tx, SecretList []string) (cashu.Proofs, error)
	GetProofsFromSecretCurve(tx pgx.Tx, Ys []cashu.WrappedPublicKey) (cashu.Proofs, error)
	GetProofsFromQuote(tx pgx.Tx, quote string) (cashu.Proofs, error)
	SetProofsState(tx pgx.Tx, proofs cashu.Proofs, state cashu.ProofState) error
	DeleteProofs(tx pgx.Tx, proofs cashu.Proofs) error

	GetRestoreSigsFromBlindedMessages(tx pgx.Tx, B_ []cashu.WrappedPublicKey) ([]cashu.RecoverSigDB, error)
	SaveRestoreSigs(tx pgx.Tx, recover_sigs []cashu.RecoverSigDB) error

	// GetProofsMintReserve(since time.Time, until *time.Time) (EcashInventory, error)
	// GetBlindSigsMintReserve(since time.Time, until *time.Time) (EcashInventory, error)
	GetConfig(tx pgx.Tx) (utils.Config, error)
	SetConfig(tx pgx.Tx, config utils.Config) error
	UpdateConfig(tx pgx.Tx, config utils.Config) error
	GetNostrNotificationConfig(tx pgx.Tx) (*utils.NostrNotificationConfig, error)
	UpdateNostrNotificationConfig(tx pgx.Tx, config utils.NostrNotificationConfig) error

	SaveMeltChange(tx pgx.Tx, change []cashu.BlindedMessage, quote string) error
	GetMeltChangeByQuote(tx pgx.Tx, quote string) ([]cashu.MeltChange, error)
	DeleteChangeByQuote(tx pgx.Tx, quote string) error

	SaveNostrAuth(auth NostrLoginAuth) error
	UpdateNostrAuthActivation(tx pgx.Tx, nonce string, activated bool) error
	GetNostrAuth(tx pgx.Tx, nonce string) (NostrLoginAuth, error)

	// liquidity swaps
	AddLiquiditySwap(tx pgx.Tx, swap utils.LiquiditySwap) error
	GetLiquiditySwapById(tx pgx.Tx, id string) (utils.LiquiditySwap, error)
	ChangeLiquiditySwapState(tx pgx.Tx, id string, state utils.SwapState) error
	GetAllLiquiditySwaps() ([]utils.LiquiditySwap, error)
	GetLiquiditySwapsByStates(tx pgx.Tx, states []utils.SwapState) ([]string, error)

	// Mint Auth
	GetAuthUser(tx pgx.Tx, sub string) (AuthUser, error)
	MakeAuthUser(tx pgx.Tx, auth AuthUser) error
	UpdateLastLoggedIn(tx pgx.Tx, sub string, lastLoggedIn uint64) error
	GetMintRequestsByTime(ctx context.Context, since time.Time) ([]cashu.MintRequestDB, error)
	GetMeltRequestsByTime(ctx context.Context, since time.Time) ([]cashu.MeltRequestDB, error)
	SearchLightningRequests(ctx context.Context, query string, since time.Time, limit int) ([]LightningActivityRow, error)

	GetLatestStatsSnapshot(ctx context.Context) (*StatsSnapshot, error)
	GetMintStatsRows(ctx context.Context, tx pgx.Tx, startDate, endDate int64) ([]MintStatsRow, error)
	GetMeltStatsRows(ctx context.Context, tx pgx.Tx, startDate, endDate int64) ([]MeltStatsRow, error)
	GetProofStatsRows(ctx context.Context, tx pgx.Tx, startDate, endDate int64) ([]KeysetStatsRow, error)
	GetBlindSigStatsRows(ctx context.Context, tx pgx.Tx, startDate, endDate int64) ([]KeysetStatsRow, error)
	GetStatsFeeRows(ctx context.Context, tx pgx.Tx, startDate, endDate int64) ([]KeysetFeeRow, error)
	GetStatsSnapshotsBySince(ctx context.Context, since int64) ([]StatsSnapshot, error)
	InsertStatsSnapshot(ctx context.Context, snapshot StatsSnapshot) error
}
