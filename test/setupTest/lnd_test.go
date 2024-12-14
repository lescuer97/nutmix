package setuptest

import (
	"context"
	"os"
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lescuer97/nutmix/internal/lightning"
	"github.com/lescuer97/nutmix/internal/utils"
)

func TestSetupLightingCommsLND(t *testing.T) {
	// setup
	ctx := context.Background()
	_, _, _, _, err := utils.SetUpLightingNetworkTestEnviroment(ctx, "lightingsetup-test")
	t.Setenv("MINT_LIGHTNING_BACKEND", "LndGrpcWallet")

	lnd_host := os.Getenv(utils.LND_HOST)
	tls_cert := os.Getenv(utils.LND_TLS_CERT)
	macaroon := os.Getenv(utils.LND_MACAROON)

	if err != nil {
		t.Fatalf("setUpLightingNetworkEnviroment %+v", err)
	}
	lndWallet := lightning.LndGrpcWallet{
		Network: chaincfg.RegressionNetParams,
	}

	err = lndWallet.SetupGrpc(lnd_host, macaroon, tls_cert)
	if err != nil {
		t.Fatalf("setUpLightingNetworkEnviroment %+v", err)
	}

	invoice, err := lndWallet.RequestInvoice(1000)
	if err != nil {
		t.Fatalf("could not setup lighting comms %+v", err)
	}

	if len(invoice.PaymentRequest) == 0 {
		t.Fatalf("There is no payment request %+v", err)
	}

}

func TestSetupLightingCommsLnBits(t *testing.T) {
	// setup
	ctx := context.Background()
	_, _, _, _, err := utils.SetUpLightingNetworkTestEnviroment(ctx, "lnbits-test")
	t.Setenv("MINT_LIGHTNING_BACKEND", "LNbitsWallet")

	endpoint := os.Getenv(utils.MINT_LNBITS_ENDPOINT)
	key := os.Getenv(utils.MINT_LNBITS_KEY)

	lnbitsWallet := lightning.LnbitsWallet{
		Network:  chaincfg.RegressionNetParams,
		Key:      key,
		Endpoint: endpoint,
	}

	if err != nil {
		t.Fatalf("setUpLightingNetworkEnviroment %+v", err)
	}
	invoice, err := lnbitsWallet.RequestInvoice(1000)
	if err != nil {
		t.Fatalf("could not setup lighting comms %+v", err)
	}

	if len(invoice.PaymentRequest) == 0 {
		t.Fatalf("There is no payment request %+v", err)
	}

}
