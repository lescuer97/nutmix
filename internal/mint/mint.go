package mint

import (
	"context"
	"errors"
	"fmt"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/lightning"
	"github.com/lescuer97/nutmix/internal/signer"
	"github.com/lescuer97/nutmix/internal/utils"
	"log"
	"sync"
)

type Mint struct {
	LightningBackend lightning.LightningBackend
	PendingProofs    []cashu.Proof
	ActiveProofs     *ActiveProofs
	ActiveQuotes     *ActiveQuote
	Config           utils.Config
	MintPubkey       string
	MintDB           database.MintDB
	Signer           signer.Signer
}

type ActiveProofs struct {
	Proofs map[cashu.Proof]bool
	sync.Mutex
}

func (a *ActiveProofs) AddProofs(proofs []cashu.Proof) error {
	a.Lock()
	defer a.Unlock()
	// check if proof already exists
	for _, p := range proofs {

		if a.Proofs[p] {
			return cashu.AlreadyActiveProof
		}

		a.Proofs[p] = true
	}
	return nil
}

func (a *ActiveProofs) RemoveProofs(proofs []cashu.Proof) error {
	a.Lock()
	defer a.Unlock()
	// check if proof already exists
	for _, p := range proofs {

		delete(a.Proofs, p)

	}
	return nil
}

type ActiveQuote struct {
	Quote map[string]bool
	sync.Mutex
}

func (q *ActiveQuote) AddQuote(quote string) error {
	q.Lock()

	defer q.Unlock()

	if q.Quote[quote] {
		return cashu.AlreadyActiveQuote
	}

	q.Quote[quote] = true

	return nil
}
func (q *ActiveQuote) RemoveQuote(quote string) error {
	q.Lock()
	defer q.Unlock()

	delete(q.Quote, quote)

	return nil
}
func (m *Mint) AddQuotesAndProofs(quote string, proofs []cashu.Proof) error {

	if quote != "" {
		err := m.ActiveQuotes.AddQuote(quote)
		if err != nil {
			return fmt.Errorf("m.AddActiveMintQuote(quote): %w", err)
		}
	}

	if len(proofs) == 0 {
		err := m.ActiveProofs.AddProofs(proofs)
		if err != nil {
			return fmt.Errorf("AddProofs: %w", err)
		}

	}
	return nil
}

func (m *Mint) RemoveQuotesAndProofs(quote string, proofs []cashu.Proof) {
	if quote != "" {
		m.ActiveQuotes.RemoveQuote(quote)
	}

	if len(proofs) == 0 {
		m.ActiveProofs.RemoveProofs(proofs)

	}
}

// errors types for validation
var (
	ErrInvalidProof        = errors.New("Invalid proof")
	ErrQuoteNotPaid        = errors.New("Quote not paid")
	ErrMessageAmountToBig  = errors.New("Message amount is to big")
	ErrInvalidBlindMessage = errors.New("Invalid blind message")
)

var (
	NETWORK_ENV                = "NETWORK"
	MINT_LIGHTNING_BACKEND_ENV = "MINT_LIGHTNING_BACKEND"
)

func (m *Mint) CheckProofsAreSameUnit(proofs []cashu.Proof, keys []cashu.BasicKeysetResponse) (cashu.Unit, error) {

	units := make(map[string]bool)

	seenKeys := make(map[string]cashu.BasicKeysetResponse)

	for _, v := range keys {
		seenKeys[v.Id] = v
	}
	for _, proof := range proofs {

		val, exists := seenKeys[proof.Id]

		if exists {
			units[val.Unit] = true
		}
		if len(units) > 1 {
			return cashu.Sat, fmt.Errorf("Proofs are not the same unit")
		}
	}

	if len(units) == 0 {
		return cashu.Sat, fmt.Errorf("No units found")
	}

	var returnedUnit cashu.Unit
	for unit := range units {
		finalUnit, err := cashu.UnitFromString(unit)
		if err != nil {
			return cashu.Sat, fmt.Errorf("UnitFromString: %w", err)
		}

		returnedUnit = finalUnit
	}

	return returnedUnit, nil

}

func CheckChainParams(network string) (chaincfg.Params, error) {
	switch network {
	case "testnet":
		return chaincfg.TestNet3Params, nil
	case "mainnet":
		return chaincfg.MainNetParams, nil
	case "regtest":
		return chaincfg.RegressionNetParams, nil
	case "signet":
		return chaincfg.SigNetParams, nil
	default:
		return chaincfg.MainNetParams, fmt.Errorf("Invalid network: %s", network)
	}

}

func SetUpMint(ctx context.Context, config utils.Config, db database.MintDB, sig signer.Signer) (*Mint, error) {
	activeProofs := ActiveProofs{
		Proofs: make(map[cashu.Proof]bool),
	}
	activeQuotes := ActiveQuote{
		Quote: make(map[string]bool),
	}
	mint := Mint{
		Config:       config,
		ActiveProofs: &activeProofs,
		ActiveQuotes: &activeQuotes,
		MintDB:       db,
		Signer:       sig,
	}

	chainparam, err := CheckChainParams(config.NETWORK)
	if err != nil {
		return &mint, fmt.Errorf("CheckChainParams(config.NETWORK) %w", err)
	}

	switch config.MINT_LIGHTNING_BACKEND {

	case utils.FAKE_WALLET:
		fake_wallet := lightning.FakeWallet{
			Network: chainparam,
		}

		mint.LightningBackend = fake_wallet

	case utils.LNDGRPC:
		lndWallet := lightning.LndGrpcWallet{
			Network: chainparam,
		}

		err := lndWallet.SetupGrpc(config.LND_GRPC_HOST, config.LND_MACAROON, config.LND_TLS_CERT)
		if err != nil {
			return &mint, fmt.Errorf("lndWallet.SetupGrpc %w", err)
		}
		mint.LightningBackend = lndWallet
	case utils.LNBITS:
		lnbitsWallet := lightning.LnbitsWallet{
			Network:  chainparam,
			Endpoint: config.MINT_LNBITS_ENDPOINT,
			Key:      config.MINT_LNBITS_KEY,
		}
		mint.LightningBackend = lnbitsWallet
	case utils.CLNGRPC:
		clnWallet := lightning.CLNGRPCWallet{
			Network: chainparam,
		}

		err := clnWallet.SetupGrpc(config.CLN_GRPC_HOST, config.CLN_CA_CERT, config.CLN_CLIENT_CERT, config.CLN_CLIENT_KEY, config.CLN_MACAROON)
		if err != nil {
			return &mint, fmt.Errorf("lndWallet.SetupGrpc %w", err)
		}
		mint.LightningBackend = clnWallet

	default:
		log.Fatalf("Unknown lightning backend: %s", config.MINT_LIGHTNING_BACKEND)
	}

	mint.PendingProofs = make([]cashu.Proof, 0)

	// parse mint private key and get hex value pubkey
	pubkey, err := sig.GetSignerPubkey()
	if err != nil {
		return &mint, fmt.Errorf("sig.GetSignerPubkey() %w", err)
	}

	mint.MintPubkey = pubkey

	return &mint, nil
}

type AddToDBFunc func(string, bool, cashu.ACTION_STATE, bool) error

func (m *Mint) VerifyLightingPaymentHappened(paid bool, quote string, dbCall AddToDBFunc) (cashu.ACTION_STATE, string, error) {
	state, preimage, err := m.LightningBackend.CheckPayed(quote)
	if err != nil {
		return cashu.UNPAID, "", fmt.Errorf("mint.LightningComs.CheckIfInvoicePayed: %w", err)
	}

	switch {
	case state == lightning.SETTLED:
		err := dbCall(quote, true, cashu.PAID, false)
		if err != nil {
			return cashu.PAID, preimage, fmt.Errorf("dbCall: %w", err)
		}
		return cashu.PAID, preimage, nil

	case state == lightning.PENDING:
		err := dbCall(quote, false, cashu.UNPAID, false)
		if err != nil {
			return cashu.UNPAID, preimage, fmt.Errorf("dbCall: %w", err)
		}
		return cashu.UNPAID, preimage, nil

	}

	return cashu.UNPAID, "", nil
}
