package comms

import (
	"context"
	"os"
	"testing"
)

func TestSetupLightingCommsLND(t *testing.T) {
	// setup
	ctx := context.Background()
	_, _, _, _, err := SetUpLightingNetworkTestEnviroment(ctx, "lightingsetup-test")
	os.Setenv("MINT_LIGHTNING_BACKEND", "LndGrpcWallet")

	ctx = context.WithValue(ctx, LND_HOST, os.Getenv(LND_HOST))
	ctx = context.WithValue(ctx, LND_TLS_CERT, os.Getenv(LND_TLS_CERT))
	ctx = context.WithValue(ctx, LND_MACAROON, os.Getenv(LND_MACAROON))
	ctx = context.WithValue(ctx, "MINT_LIGHTNING_BACKEND", os.Getenv("MINT_LIGHTNING_BACKEND"))

	if err != nil {
		t.Fatalf("setUpLightingNetworkEnviroment %+v", err)
	}

	lightingComms, err := SetupLightingComms(ctx)

	if err != nil {
		t.Fatalf("could not setup lighting comms %+v", err)
	}

	invoice, err := lightingComms.RequestInvoice(1000)
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
	_, _, _, _, err := SetUpLightingNetworkTestEnviroment(ctx, "lnbits-test")
	os.Setenv("MINT_LIGHTNING_BACKEND", "LNbitsWallet")

	ctx = context.WithValue(ctx, MINT_LNBITS_ENDPOINT, os.Getenv(MINT_LNBITS_ENDPOINT))
	ctx = context.WithValue(ctx, MINT_LNBITS_KEY, os.Getenv(MINT_LNBITS_KEY))
	ctx = context.WithValue(ctx, "MINT_LIGHTNING_BACKEND", os.Getenv("MINT_LIGHTNING_BACKEND"))

	if err != nil {
		t.Fatalf("setUpLightingNetworkEnviroment %+v", err)
	}

 //    lightingComms, err := SetupLightingComms(ctx)
	//
	// if err != nil {
	// 	t.Fatalf("could not setup lighting comms %+v", err)
	// }
	// //
	// invoice, err := lightingComms.RequestInvoice(1000)
	// if err != nil {
	// 	t.Fatalf("could not setup lighting comms %+v", err)
	// }
	//
	// if len(invoice.PaymentRequest) == 0 {
	// 	t.Fatalf("There is no payment request %+v", err)
	// }

}
