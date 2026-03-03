package mint

import (
	"bytes"
	"database/sql"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	mockdb "github.com/lescuer97/nutmix/internal/database/mock_db"
	"github.com/lescuer97/nutmix/internal/utils"
	"github.com/nbd-wtf/go-nostr"
)

func TestSetUpConfigDBLoadsNostrNotificationNsecFromFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	privateKeyHex := nostr.GeneratePrivateKey()
	privateKeyBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		t.Fatalf("hex.DecodeString(privateKeyHex): %v", err)
	}

	if err := utils.WriteNostrNotificationNsec(privateKeyBytes); err != nil {
		t.Fatalf("utils.WriteNostrNotificationNsec(privateKeyBytes): %v", err)
	}

	var config utils.Config
	config.Default()
	config.NOSTR_NOTIFICATIONS = true

	db := &mockdb.MockDB{Config: config} //nolint:exhaustruct
	loadedConfig, err := SetUpConfigDB(db)
	if err != nil {
		t.Fatalf("SetUpConfigDB(db): %v", err)
	}

	if !bytes.Equal(loadedConfig.NOSTR_NOTIFICATION_NSEC, privateKeyBytes) {
		t.Fatal("expected SetUpConfigDB to load nostr notification nsec from file")
	}
}

func TestSetUpConfigDBCreatesNostrNotificationNsecOnInitialBootstrap(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	configDir, err := utils.GetConfigDirectory()
	if err != nil {
		t.Fatalf("utils.GetConfigDirectory(): %v", err)
	}

	if err := os.MkdirAll(configDir, 0o750); err != nil {
		t.Fatalf("os.MkdirAll(configDir, 0750): %v", err)
	}

	configFilePath := filepath.Join(configDir, ConfigFileName)
	configFile := []byte("NETWORK = \"mainnet\"\nMINT_LIGHTNING_BACKEND = \"FakeWallet\"\nNOSTR_NOTIFICATIONS = true\n")
	if err := os.WriteFile(configFilePath, configFile, 0o600); err != nil {
		t.Fatalf("os.WriteFile(configFilePath, configFile, 0600): %v", err)
	}

	db := &mockdb.MockDB{GetConfigErr: sql.ErrNoRows} //nolint:exhaustruct
	loadedConfig, err := SetUpConfigDB(db)
	if err != nil {
		t.Fatalf("SetUpConfigDB(db): %v", err)
	}

	if !loadedConfig.NOSTR_NOTIFICATIONS {
		t.Fatal("expected nostr notifications to remain enabled during bootstrap")
	}

	if len(loadedConfig.NOSTR_NOTIFICATION_NSEC) == 0 {
		t.Fatal("expected SetUpConfigDB to create a nostr notification nsec during bootstrap")
	}

	if _, err := os.Stat(filepath.Join(configDir, utils.NostrNotificationNsecFileName)); err != nil {
		t.Fatalf("expected nostr notification nsec file to be created: %v", err)
	}
}

func TestSetUpConfigDBFailsWhenExistingEnabledNostrNotificationNsecFileIsMissing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	var config utils.Config
	config.Default()
	config.NOSTR_NOTIFICATIONS = true

	db := &mockdb.MockDB{Config: config} //nolint:exhaustruct
	if _, err := SetUpConfigDB(db); err == nil {
		t.Fatal("expected SetUpConfigDB to fail when nostr notifications are enabled without an nsec file")
	}
}
