package comms

import (
	"context"
	"testing"
)

func TestSetupLightingComms(t *testing.T) {
	// setup
	ctx := context.Background()
	_, _, _, err := SetUpLightingNetworkTestEnviroment(ctx, "lightingsetup-test")

	if err != nil {
		t.Fatalf("setUpLightingNetworkEnviroment %+v", err)
	}

	lightingComms, err := SetupLightingComms()

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
