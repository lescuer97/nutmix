package postgresql

import (
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/utils"
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

	commitConfigTx(t, db, func(tx pgx.Tx) error {
		return db.SetLDKConfig(ctx, tx, want)
	})

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
	commitConfigTx(t, db, func(tx pgx.Tx) error {
		return db.SetLDKConfig(ctx, tx, want)
	})
	got, err = db.GetLDKConfig(ctx)
	if err != nil {
		t.Fatalf("db.GetLDKConfig(ctx): %v", err)
	}
	if got.TorProxyAddress != nil {
		t.Fatalf("expected nil tor proxy address, got %q", *got.TorProxyAddress)
	}
}

func TestSetLDKConfigRollsBackWithTransaction(t *testing.T) {
	db, ctx := setupTestDB(t)
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatalf("db.GetTx(ctx): %v", err)
	}

	config := database.LDKConfig{
		ConfigDirectory: "/tmp/ldk-rollback",
		ChainSourceType: database.LDKChainSourceBitcoind,
		Rpc: database.LDKRPCConfig{
			Address: "127.0.0.1", Username: "user", Password: "pass", Port: 18443,
		},
	}
	if err := db.SetLDKConfig(ctx, tx, config); err != nil {
		t.Fatalf("db.SetLDKConfig(tx, config): %v", err)
	}
	if err := db.Rollback(ctx, tx); err != nil {
		t.Fatalf("db.Rollback(ctx, tx): %v", err)
	}

	if _, err := db.GetLDKConfig(ctx); err == nil {
		t.Fatal("expected rolled-back LDK config not to be persisted")
	}
}

func TestLightningAndLDKConfigsRollBackTogether(t *testing.T) {
	db, ctx := setupTestDB(t)
	var oldConfig utils.Config
	oldConfig.Default()
	oldConfig.NAME = "old"
	oldConfig.NETWORK = "regtest"
	oldLDK := database.LDKConfig{
		ConfigDirectory: "/tmp/ldk-old",
		ChainSourceType: database.LDKChainSourceBitcoind,
		Rpc:             database.LDKRPCConfig{Address: "127.0.0.1", Username: "user", Password: "pass", Port: 18443},
	}
	commitConfigTx(t, db, func(tx pgx.Tx) error {
		if err := db.SetConfig(tx, oldConfig); err != nil {
			return err
		}
		return db.SetLDKConfig(ctx, tx, oldLDK)
	})

	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatalf("db.GetTx(ctx): %v", err)
	}
	newConfig := oldConfig
	newConfig.NAME = "new"
	newLDK := oldLDK
	newLDK.Rpc.Address = "127.0.0.2"
	if err := db.UpdateConfig(tx, newConfig); err != nil {
		t.Fatalf("db.UpdateConfig(tx, newConfig): %v", err)
	}
	if err := db.SetLDKConfig(ctx, tx, newLDK); err != nil {
		t.Fatalf("db.SetLDKConfig(ctx, tx, newLDK): %v", err)
	}
	if err := db.Rollback(ctx, tx); err != nil {
		t.Fatalf("db.Rollback(ctx, tx): %v", err)
	}

	readTx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatalf("db.GetTx(ctx): %v", err)
	}
	defer func() { _ = db.Rollback(ctx, readTx) }()
	gotConfig, err := db.GetConfig(readTx)
	if err != nil {
		t.Fatalf("db.GetConfig(readTx): %v", err)
	}
	gotLDK, err := db.GetLDKConfig(ctx)
	if err != nil {
		t.Fatalf("db.GetLDKConfig(ctx): %v", err)
	}
	if gotConfig.NAME != oldConfig.NAME || gotLDK.Rpc.Address != oldLDK.Rpc.Address {
		t.Fatalf("rollback mismatch: config=%q LDK address=%q", gotConfig.NAME, gotLDK.Rpc.Address)
	}
}
