package mint

import (
	"context"
	"fmt"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/lightning"
	"github.com/lescuer97/nutmix/internal/signer"
	"github.com/lescuer97/nutmix/internal/utils"
	"log"
)

type Mint struct {
	LightningBackend lightning.LightningBackend
	Config           utils.Config
	MintPubkey       string
	MintDB           database.MintDB
	Signer           signer.Signer
	Observer         *Observer
}

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
			return cashu.Sat, cashu.ErrNotSameUnits
		}
	}

	if len(units) == 0 {
		return cashu.Sat, cashu.ErrUnitNotSupported
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
	case "testnet3":
		return chaincfg.TestNet3Params, nil
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
	mint := Mint{
		Config: config,
		MintDB: db,
		Signer: sig,
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
	case utils.Strike:
		strikeWallet := lightning.Strike{
			Network: chainparam,
		}

		err := strikeWallet.Setup(config.STRIKE_KEY, config.STRIKE_ENDPOINT)
		if err != nil {
			return &mint, fmt.Errorf("lndWallet.SetupGrpc %w", err)
		}
		mint.LightningBackend = strikeWallet

	default:
		log.Fatalf("Unknown lightning backend: %s", config.MINT_LIGHTNING_BACKEND)
	}

	// parse mint private key and get hex value pubkey
	pubkey, err := sig.GetSignerPubkey()
	if err != nil {
		return &mint, fmt.Errorf("sig.GetSignerPubkey() %w", err)
	}

	mint.MintPubkey = pubkey
	observer := Observer{}
	observer.Proofs = make(map[string][]ProofWatchChannel)
	observer.MeltQuote = make(map[string][]MeltQuoteChannel)
	observer.MintQuote = make(map[string][]MintQuoteChannel)

	mint.Observer = &observer
	return &mint, nil
}
