package postgresql

import (
	"testing"

	"github.com/lescuer97/nutmix/internal/database"
)

func TestSetAndGetLDKConfigRoundTripEsplora(t *testing.T) {
	db, ctx := setupTestDB(t)
	torProxyAddress := "127.0.0.1:9050"

	want := database.LDKConfig{
		ConfigDirectory:   "/tmp/ldk-test",
		ChainSourceType:   database.LDKChainSourceEsplora,
		ElectrumServerURL: "ssl://electrum.example:50002",
		EsploraServerURL:  "https://blockstream.info/api",
		TorOnly:           true,
		TorProxyAddress:   &torProxyAddress,
		Rpc: database.LDKRPCConfig{
			Address:  "127.0.0.1",
			Username: "rpc-user",
			Password: "rpc-pass",
			Port:     18443,
		},
	}

	if err := db.SetLDKConfig(ctx, want); err != nil {
		t.Fatalf("db.SetLDKConfig(ctx, want): %v", err)
	}

	got, err := db.GetLDKConfig(ctx)
	if err != nil {
		t.Fatalf("db.GetLDKConfig(ctx): %v", err)
	}

	if got.ConfigDirectory != want.ConfigDirectory {
		t.Fatalf("config directory mismatch: got %q want %q", got.ConfigDirectory, want.ConfigDirectory)
	}
	if got.ChainSourceType != want.ChainSourceType {
		t.Fatalf("chain source type mismatch: got %q want %q", got.ChainSourceType, want.ChainSourceType)
	}
	if got.ElectrumServerURL != want.ElectrumServerURL {
		t.Fatalf("electrum server url mismatch: got %q want %q", got.ElectrumServerURL, want.ElectrumServerURL)
	}
	if got.EsploraServerURL != want.EsploraServerURL {
		t.Fatalf("esplora server url mismatch: got %q want %q", got.EsploraServerURL, want.EsploraServerURL)
	}
	if got.TorOnly != want.TorOnly {
		t.Fatalf("tor only mismatch: got %v want %v", got.TorOnly, want.TorOnly)
	}
	if got.TorProxyAddress == nil || *got.TorProxyAddress != *want.TorProxyAddress {
		t.Fatalf("tor proxy address mismatch: got %v want %v", got.TorProxyAddress, want.TorProxyAddress)
	}
	if got.Rpc.Address != want.Rpc.Address {
		t.Fatalf("rpc address mismatch: got %q want %q", got.Rpc.Address, want.Rpc.Address)
	}
	if got.Rpc.Username != want.Rpc.Username {
		t.Fatalf("rpc username mismatch: got %q want %q", got.Rpc.Username, want.Rpc.Username)
	}
	if got.Rpc.Password != want.Rpc.Password {
		t.Fatalf("rpc password mismatch: got %q want %q", got.Rpc.Password, want.Rpc.Password)
	}
	if got.Rpc.Port != want.Rpc.Port {
		t.Fatalf("rpc port mismatch: got %d want %d", got.Rpc.Port, want.Rpc.Port)
	}

	want.TorProxyAddress = nil
	if err := db.SetLDKConfig(ctx, want); err != nil {
		t.Fatalf("db.SetLDKConfig(ctx, want without proxy): %v", err)
	}
	got, err = db.GetLDKConfig(ctx)
	if err != nil {
		t.Fatalf("db.GetLDKConfig(ctx): %v", err)
	}
	if got.TorProxyAddress != nil {
		t.Fatalf("expected nil tor proxy address, got %q", *got.TorProxyAddress)
	}
}
