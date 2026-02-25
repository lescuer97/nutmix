package ldk

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ldk_node "github.com/lescuer97/ldkgo/bindings/ldk_node_ffi"
	mockdb "github.com/lescuer97/nutmix/internal/database/mock_db"
	"github.com/lescuer97/nutmix/internal/utils"
)

func mustPersistedConfig(t *testing.T, configDirectory string) PersistedConfig {
	t.Helper()

	config, err := NewPersistedConfig(RPCConfig{
		Address:  "127.0.0.1",
		Port:     18443,
		Username: "user",
		Password: "pass",
	}, configDirectory)
	if err != nil {
		t.Fatalf("NewPersistedConfig(...): %v", err)
	}

	return config
}

func mustElectrumPersistedConfig(t *testing.T, configDirectory string) PersistedConfig {
	t.Helper()

	config, err := NewPersistedConfigWithChainSource(
		ChainSourceElectrum,
		RPCConfig{},
		"ssl://electrum.example:50002",
		"",
		configDirectory,
	)
	if err != nil {
		t.Fatalf("NewPersistedConfigWithChainSource(...): %v", err)
	}

	return config
}

func mustEsploraPersistedConfig(t *testing.T, configDirectory string) PersistedConfig {
	t.Helper()

	config, err := NewPersistedConfigWithChainSource(
		ChainSourceEsplora,
		RPCConfig{},
		"",
		"https://blockstream.info/api",
		configDirectory,
	)
	if err != nil {
		t.Fatalf("NewPersistedConfigWithChainSource(...): %v", err)
	}

	return config
}

func TestReadOrCreateSeedCreatesLDKSeedFile(t *testing.T) {
	tempDir := t.TempDir()

	seed, err := ReadOrCreateSeed(tempDir)
	if err != nil {
		t.Fatalf("ReadOrCreateSeed(tempDir): %v", err)
	}

	if seed == "" {
		t.Fatalf("expected non-empty seed")
	}

	seedPath := filepath.Join(tempDir, seedFileName)
	seedFile, err := os.ReadFile(seedPath)
	if err != nil {
		t.Fatalf("os.ReadFile(seedPath): %v", err)
	}

	if got := string(seedFile); got != seed {
		t.Fatalf("seed content mismatch, got %q, want %q", got, seed)
	}

	if gotWords := len(strings.Fields(seed)); gotWords != 24 {
		t.Fatalf("seed words mismatch, got %d, want 24", gotWords)
	}

	seedInfo, err := os.Stat(seedPath)
	if err != nil {
		t.Fatalf("os.Stat(seedPath): %v", err)
	}
	if seedInfo.Mode().Perm() != 0o600 {
		t.Fatalf("seed mode mismatch, got %o, want %o", seedInfo.Mode().Perm(), 0o600)
	}
}

func TestReadOrCreateSeedReturnsExistingSeed(t *testing.T) {
	tempDir := t.TempDir()
	seedPath := filepath.Join(tempDir, seedFileName)
	expectedSeed := "alpha beta gamma delta epsilon zeta eta theta iota kappa lambda mu nu xi omicron pi rho sigma tau upsilon phi chi psi omega"

	err := os.WriteFile(seedPath, []byte(expectedSeed), 0o600)
	if err != nil {
		t.Fatalf("os.WriteFile(seedPath, expectedSeed, 0600): %v", err)
	}

	seed, err := ReadOrCreateSeed(tempDir)
	if err != nil {
		t.Fatalf("ReadOrCreateSeed(tempDir): %v", err)
	}

	if seed != expectedSeed {
		t.Fatalf("seed mismatch, got %q, want %q", seed, expectedSeed)
	}
}

func TestReadOrCreateSeedFailsOnInvalidExistingSeed(t *testing.T) {
	tempDir := t.TempDir()
	seedPath := filepath.Join(tempDir, seedFileName)
	invalidSeed := "abandon amount liar amount expire adjust cage candy arch gather drum buyer"

	err := os.WriteFile(seedPath, []byte(invalidSeed), 0o600)
	if err != nil {
		t.Fatalf("os.WriteFile(seedPath, invalidSeed, 0600): %v", err)
	}

	_, err = ReadOrCreateSeed(tempDir)
	if err == nil {
		t.Fatalf("expected error for invalid seed")
	}
}

func TestPrepareInitConfigUsesSeedAndConfig(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	db := &mockdb.MockDB{}
	err := SaveConfig(ctx, db, mustPersistedConfig(t, tempDir))
	if err != nil {
		t.Fatalf("SaveConfig(...): %v", err)
	}

	backend := &LDK{node: nil, db: db, network: "testnet3"}
	seedMnemonic, storageDir, network, config, err := backend.prepareInitConfig(ctx)
	if err != nil {
		t.Fatalf("backend.prepareInitConfig(ctx): %v", err)
	}

	if seedMnemonic == "" {
		t.Fatalf("expected non-empty prepared seed")
	}
	if storageDir != tempDir {
		t.Fatalf("storageDir = %q, want %q", storageDir, tempDir)
	}
	if network != ldk_node.NetworkTestnet {
		t.Fatalf("expected testnet network, got %v", network)
	}
	if config.Rpc.Address != "127.0.0.1" {
		t.Fatalf("rpc address mismatch, got %q", config.Rpc.Address)
	}
}

func TestValidatePersistedConfigRequiresRPCCredentials(t *testing.T) {
	configDirectory := t.TempDir()
	err := validatePersistedConfig(PersistedConfig{
		ChainSourceType: ChainSourceBitcoind,
		Rpc: RPCConfig{
			Address:  "127.0.0.1",
			Port:     18443,
			Username: "",
			Password: "",
		},
		ConfigDirectory: configDirectory,
	})
	if err == nil {
		t.Fatalf("expected error when username/password are empty")
	}
}

func TestValidatePersistedConfigRequiresElectrumServerURL(t *testing.T) {
	configDirectory := t.TempDir()
	err := validatePersistedConfig(PersistedConfig{
		ChainSourceType: ChainSourceElectrum,
		ConfigDirectory: configDirectory,
	})
	if err == nil {
		t.Fatalf("expected error when electrum server url is empty")
	}
}

func TestValidatePersistedConfigRequiresEsploraServerURL(t *testing.T) {
	configDirectory := t.TempDir()
	err := validatePersistedConfig(PersistedConfig{
		ChainSourceType: ChainSourceEsplora,
		ConfigDirectory: configDirectory,
	})
	if err == nil {
		t.Fatalf("expected error when esplora server url is empty")
	}
}

func TestPrepareInitConfigReturnsElectrumConfig(t *testing.T) {
	ctx := context.Background()
	configDirectory := t.TempDir()
	db := &mockdb.MockDB{}
	err := SaveConfig(ctx, db, mustElectrumPersistedConfig(t, configDirectory))
	if err != nil {
		t.Fatalf("SaveConfig(...): %v", err)
	}

	backend := &LDK{node: nil, db: db, network: "testnet3"}
	_, storageDir, network, config, err := backend.prepareInitConfig(ctx)
	if err != nil {
		t.Fatalf("backend.prepareInitConfig(ctx): %v", err)
	}
	if storageDir != configDirectory {
		t.Fatalf("storageDir = %q, want %q", storageDir, configDirectory)
	}
	if network != ldk_node.NetworkTestnet {
		t.Fatalf("expected testnet network, got %v", network)
	}
	if config.ChainSourceType != ChainSourceElectrum {
		t.Fatalf("expected electrum chain source type, got %q", config.ChainSourceType)
	}
	if config.ElectrumServerURL != "ssl://electrum.example:50002" {
		t.Fatalf("unexpected electrum server url: %q", config.ElectrumServerURL)
	}
}

func TestPrepareInitConfigReturnsEsploraConfig(t *testing.T) {
	ctx := context.Background()
	configDirectory := t.TempDir()
	db := &mockdb.MockDB{}
	err := SaveConfig(ctx, db, mustEsploraPersistedConfig(t, configDirectory))
	if err != nil {
		t.Fatalf("SaveConfig(...): %v", err)
	}

	backend := &LDK{node: nil, db: db, network: "testnet3"}
	_, storageDir, network, config, err := backend.prepareInitConfig(ctx)
	if err != nil {
		t.Fatalf("backend.prepareInitConfig(ctx): %v", err)
	}
	if storageDir != configDirectory {
		t.Fatalf("storageDir = %q, want %q", storageDir, configDirectory)
	}
	if network != ldk_node.NetworkTestnet {
		t.Fatalf("expected testnet network, got %v", network)
	}
	if config.ChainSourceType != ChainSourceEsplora {
		t.Fatalf("expected esplora chain source type, got %q", config.ChainSourceType)
	}
	if config.EsploraServerURL != "https://blockstream.info/api" {
		t.Fatalf("unexpected esplora server url: %q", config.EsploraServerURL)
	}
}

func TestNewPersistedConfigWithChainSourceRejectsInvalidElectrumURL(t *testing.T) {
	_, err := NewPersistedConfigWithChainSource(ChainSourceElectrum, RPCConfig{}, "electrum.example:50002", "", t.TempDir())
	if err == nil {
		t.Fatal("expected invalid electrum url error")
	}
}

func TestNewPersistedConfigWithChainSourceRejectsInvalidEsploraURL(t *testing.T) {
	_, err := NewPersistedConfigWithChainSource(ChainSourceEsplora, RPCConfig{}, "", "esplora.example/api", t.TempDir())
	if err == nil {
		t.Fatal("expected invalid esplora url error")
	}
}

func TestForcedEsploraSyncConfigUsesDocumentedDefaults(t *testing.T) {
	config := forcedEsploraSyncConfig()
	if config == nil {
		t.Fatal("expected forced Esplora sync config")
	}
	if config.BackgroundSyncConfig == nil {
		t.Fatal("expected background sync config")
	}

	if config.BackgroundSyncConfig.OnchainWalletSyncIntervalSecs != 80 {
		t.Fatalf("unexpected onchain wallet sync interval: %d", config.BackgroundSyncConfig.OnchainWalletSyncIntervalSecs)
	}
	if config.BackgroundSyncConfig.LightningWalletSyncIntervalSecs != 30 {
		t.Fatalf("unexpected lightning wallet sync interval: %d", config.BackgroundSyncConfig.LightningWalletSyncIntervalSecs)
	}
	if config.BackgroundSyncConfig.FeeRateCacheUpdateIntervalSecs != 600 {
		t.Fatalf("unexpected fee rate cache update interval: %d", config.BackgroundSyncConfig.FeeRateCacheUpdateIntervalSecs)
	}
	if config.TimeoutsConfig.OnchainWalletSyncTimeoutSecs != 60 {
		t.Fatalf("unexpected onchain wallet sync timeout: %d", config.TimeoutsConfig.OnchainWalletSyncTimeoutSecs)
	}
	if config.TimeoutsConfig.LightningWalletSyncTimeoutSecs != 30 {
		t.Fatalf("unexpected lightning wallet sync timeout: %d", config.TimeoutsConfig.LightningWalletSyncTimeoutSecs)
	}
	if config.TimeoutsConfig.FeeRateCacheUpdateTimeoutSecs != 10 {
		t.Fatalf("unexpected fee rate cache update timeout: %d", config.TimeoutsConfig.FeeRateCacheUpdateTimeoutSecs)
	}
	if config.TimeoutsConfig.TxBroadcastTimeoutSecs != 10 {
		t.Fatalf("unexpected tx broadcast timeout: %d", config.TimeoutsConfig.TxBroadcastTimeoutSecs)
	}
	if config.TimeoutsConfig.PerRequestTimeoutSecs != 10 {
		t.Fatalf("unexpected per request timeout: %d", config.TimeoutsConfig.PerRequestTimeoutSecs)
	}
}

func TestPrepareInitConfigUsesExplicitStorageDir(t *testing.T) {
	ctx := context.Background()
	persistedDir := t.TempDir()
	tempDir := t.TempDir()
	db := &mockdb.MockDB{}
	err := SaveConfig(ctx, db, mustPersistedConfig(t, persistedDir))
	if err != nil {
		t.Fatalf("SaveConfig(...): %v", err)
	}

	backend := NewConfigBackendWithOptions(db, "regtest", Options{StorageDir: tempDir})
	seedMnemonic, storageDir, _, _, err := backend.prepareInitConfig(ctx)
	if err != nil {
		t.Fatalf("prepareInitConfig(...): %v", err)
	}
	if seedMnemonic == "" {
		t.Fatal("expected seed mnemonic")
	}
	if storageDir != tempDir {
		t.Fatalf("storageDir = %q, want %q", storageDir, tempDir)
	}
	if _, err := os.Stat(filepath.Join(tempDir, seedFileName)); err != nil {
		t.Fatalf("seed file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(persistedDir, seedFileName)); !os.IsNotExist(err) {
		t.Fatalf("expected persisted directory to remain unused, err=%v", err)
	}
}

func TestReadOrCreateSeedFallsBackToConfigDirectory(t *testing.T) {
	xdgConfigHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgConfigHome)

	seed, err := ReadOrCreateSeed("")
	if err != nil {
		t.Fatalf("ReadOrCreateSeed(\"\"): %v", err)
	}
	if seed == "" {
		t.Fatal("expected seed")
	}
	if _, err := os.Stat(filepath.Join(xdgConfigHome, utils.ConfigDirName, seedFileName)); err != nil {
		t.Fatalf("expected fallback seed file: %v", err)
	}
}

func TestNewConfigBackendWithOptionsKeepsStorageDir(t *testing.T) {
	backend := NewConfigBackendWithOptions(&mockdb.MockDB{}, "regtest", Options{StorageDir: "/tmp/ldk"})
	if backend.storageDir() != "/tmp/ldk" {
		t.Fatalf("storageDir = %q", backend.storageDir())
	}
}

func TestNodeStoragePathUsesExplicitStorageDir(t *testing.T) {
	backend := NewConfigBackendWithOptions(&mockdb.MockDB{}, "regtest", Options{StorageDir: "/tmp/ldk"})
	if got := backend.storageDir(); got != "/tmp/ldk" {
		t.Fatalf("storageDir() = %q", got)
	}
}

func TestNodeStoragePathFallsBackToConfigDirectory(t *testing.T) {
	xdgConfigHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgConfigHome)

	backend := NewConfigBackendWithOptions(&mockdb.MockDB{}, "regtest", Options{})
	if got := backend.storageDir(); got != "" {
		t.Fatalf("storageDir() = %q, want empty explicit storage dir", got)
	}
}

func TestPrepareInitConfigFailsForExplicitStorageFilePath(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	blockingFile := filepath.Join(tempDir, "not-a-dir")
	if err := os.WriteFile(blockingFile, []byte("x"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(...): %v", err)
	}
	db := &mockdb.MockDB{}
	err := SaveConfig(ctx, db, mustPersistedConfig(t, t.TempDir()))
	if err != nil {
		t.Fatalf("SaveConfig(...): %v", err)
	}

	backend := NewConfigBackendWithOptions(db, "regtest", Options{StorageDir: blockingFile})
	if _, _, _, _, err := backend.prepareInitConfig(ctx); err == nil {
		t.Fatal("expected explicit invalid storage dir to fail")
	}
}

func TestBackendStopAndReopenReusesStorageDirWithoutReseeding(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	db := &mockdb.MockDB{}
	err := SaveConfig(ctx, db, mustPersistedConfig(t, tempDir))
	if err != nil {
		t.Fatalf("SaveConfig(...): %v", err)
	}

	first := NewConfigBackendWithOptions(db, "regtest", Options{})
	if err := first.InitNode(ctx); err != nil {
		t.Fatalf("first.InitNode(ctx): %v", err)
	}
	seedBefore, err := os.ReadFile(filepath.Join(tempDir, seedFileName))
	if err != nil {
		t.Fatalf("os.ReadFile(...): %v", err)
	}

	second := NewConfigBackendWithOptions(db, "regtest", Options{})
	if err := second.InitNode(ctx); err != nil {
		t.Fatalf("second.InitNode(ctx): %v", err)
	}
	seedAfter, err := os.ReadFile(filepath.Join(tempDir, seedFileName))
	if err != nil {
		t.Fatalf("os.ReadFile(...): %v", err)
	}
	if string(seedBefore) != string(seedAfter) {
		t.Fatal("expected seed reuse across restart")
	}
	if first.storageDir() != second.storageDir() {
		t.Fatal("expected node storage dir reuse across restart")
	}
}

func TestNewLdkStartsAndStops(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	t.Cleanup(cancel)

	tempDir := t.TempDir()
	db := &mockdb.MockDB{}

	env, err := utils.SetupLDKLightningNetwork(t, ctx, "ldk-start-stop")
	if err != nil {
		t.Fatalf("utils.SetupLDKLightningNetwork(...): %v", err)
	}

	config, err := NewPersistedConfig(RPCConfig{
		Address:  env.BitcoindRPC.Address,
		Port:     env.BitcoindRPC.Port,
		Username: env.BitcoindRPC.Username,
		Password: env.BitcoindRPC.Password,
	}, tempDir)
	if err != nil {
		t.Fatalf("NewPersistedConfig(...): %v", err)
	}
	if err := SaveConfig(ctx, db, config); err != nil {
		t.Fatalf("SaveConfig(...): %v", err)
	}

	backend, err := NewLdk(ctx, db, "regtest")
	if err != nil {
		t.Fatalf("NewLdk(...): %v", err)
	}

	if err := backend.Stop(); err != nil {
		t.Fatalf("backend.Stop(): %v", err)
	}
}

func TestNewPersistedConfigRejectsEmptyConfigDirectory(t *testing.T) {
	_, err := NewPersistedConfig(RPCConfig{Address: "127.0.0.1", Port: 18443, Username: "user", Password: "pass"}, "")
	if err == nil {
		t.Fatal("expected empty config directory error")
	}
}

func TestNewPersistedConfigRejectsRelativeConfigDirectory(t *testing.T) {
	_, err := NewPersistedConfig(RPCConfig{Address: "127.0.0.1", Port: 18443, Username: "user", Password: "pass"}, "relative/ldk")
	if err == nil {
		t.Fatal("expected relative config directory error")
	}
}

func TestDefaultConfigDirectoryReturnsAbsolutePath(t *testing.T) {
	xdgConfigHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgConfigHome)

	configDirectory, err := DefaultConfigDirectory()
	if err != nil {
		t.Fatalf("DefaultConfigDirectory(): %v", err)
	}
	if !filepath.IsAbs(configDirectory) {
		t.Fatalf("expected absolute config directory, got %q", configDirectory)
	}
	if configDirectory != filepath.Join(xdgConfigHome, utils.ConfigDirName, "ldk") {
		t.Fatalf("configDirectory = %q", configDirectory)
	}
}
