package templates

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/lescuer97/nutmix/internal/lightning/ldk"
	"github.com/lescuer97/nutmix/internal/utils"
)

func TestLightningConnectionSettingsIncludesLDKOption(t *testing.T) {
	var config utils.Config
	config.Default()

	var b bytes.Buffer
	if err := LightningConnectionSettings(config).Render(context.Background(), &b); err != nil {
		t.Fatalf("LightningConnectionSettings(config).Render: %v", err)
	}

	out := b.String()
	if !strings.Contains(out, "value=\"LDK\"") {
		t.Fatalf("expected LDK option in backend selector")
	}
}

func TestSetupFormsLDKIncludesChainSourceToggle(t *testing.T) {
	var config utils.Config
	config.Default()
	resources := DefaultLDKResourceSnapshot()

	var b bytes.Buffer
	ldkForm := LDKFormValues{
		ChainSourceType: string(ldk.ChainSourceBitcoind),
		Address:         "127.0.0.1",
		Port:            "18443",
		Username:        "bitcoinrpc",
	}

	if err := SetupForms(string(utils.LDK), config, resources, ldkForm).Render(context.Background(), &b); err != nil {
		t.Fatalf("SetupForms(LDK, config).Render: %v", err)
	}

	out := b.String()
	for _, field := range []string{"LDK_CHAIN_SOURCE_TYPE", "Bitcoin Core", "Electrum", "hx-post=\"/admin/lightningdata\""} {
		if !strings.Contains(out, field) {
			t.Fatalf("expected field %s in LDK setup form", field)
		}
	}
}

func TestSetupFormsLDKIncludesBitcoindFields(t *testing.T) {
	var config utils.Config
	config.Default()
	resources := DefaultLDKResourceSnapshot()

	var b bytes.Buffer
	ldkForm := LDKFormValues{
		ChainSourceType: string(ldk.ChainSourceBitcoind),
		Address:         "127.0.0.1",
		Port:            "18443",
		Username:        "bitcoinrpc",
	}

	if err := SetupForms(string(utils.LDK), config, resources, ldkForm).Render(context.Background(), &b); err != nil {
		t.Fatalf("SetupForms(LDK, config).Render: %v", err)
	}

	out := b.String()
	for _, field := range []string{"Bitcoin Core RPC Address", "BITCOIN_NODE_RPC_ADDRESS", "BITCOIN_NODE_RPC_PORT", "BITCOIN_NODE_RPC_USERNAME", "BITCOIN_NODE_RPC_PASSWORD"} {
		if !strings.Contains(out, field) {
			t.Fatalf("expected field %s in LDK setup form", field)
		}
	}

	for _, value := range []string{"127.0.0.1", "18443", "bitcoinrpc"} {
		if !strings.Contains(out, value) {
			t.Fatalf("expected value %s in LDK setup form", value)
		}
	}

	if strings.Contains(out, "value=\"rpcpassword\"") {
		t.Fatalf("expected LDK password input to stay empty")
	}
}

func TestSetupFormsLDKIncludesElectrumFields(t *testing.T) {
	var config utils.Config
	config.Default()
	resources := DefaultLDKResourceSnapshot()

	var b bytes.Buffer
	ldkForm := LDKFormValues{
		ChainSourceType:   string(ldk.ChainSourceElectrum),
		Address:           "127.0.0.1",
		Port:              "18443",
		Username:          "bitcoinrpc",
		Password:          "hidden-pass",
		ElectrumServerURL: "ssl://electrum.example:50002",
	}

	if err := SetupForms(string(utils.LDK), config, resources, ldkForm).Render(context.Background(), &b); err != nil {
		t.Fatalf("SetupForms(LDK, config).Render: %v", err)
	}

	out := b.String()
	for _, field := range []string{"Electrum Server URL", "ELECTRUM_SERVER_URL", "ssl://electrum.example:50002", "type=\"hidden\" name=\"BITCOIN_NODE_RPC_PASSWORD\" value=\"hidden-pass\""} {
		if !strings.Contains(out, field) {
			t.Fatalf("expected field %s in LDK electrum setup form", field)
		}
	}
}
