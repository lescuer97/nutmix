package main

import (
	"os"

	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/utils"
)

func applyTestingConfigEnv(config *utils.Config) {
	config.MINT_LIGHTNING_BACKEND = utils.StringToLightningBackend(os.Getenv(mint.MINT_LIGHTNING_BACKEND_ENV))
	config.NETWORK = os.Getenv(mint.NETWORK_ENV)
	config.LND_GRPC_HOST = os.Getenv(utils.LND_HOST)
	config.LND_TLS_CERT = os.Getenv(utils.LND_TLS_CERT)
	config.LND_MACAROON = os.Getenv(utils.LND_MACAROON)
	config.MINT_LNBITS_KEY = os.Getenv(utils.MINT_LNBITS_KEY)
	config.MINT_LNBITS_ENDPOINT = os.Getenv(utils.MINT_LNBITS_ENDPOINT)
}

