package admin

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/api/cashu"
	mockdb "github.com/lescuer97/nutmix/internal/database/mock_db"
	"github.com/lescuer97/nutmix/internal/lightning/ldk"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/utils"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

func newPostContext(values url.Values) *gin.Context {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/admin/bolt11", nil)
	req.PostForm = values
	req.Form = values
	c.Request = req
	return c
}

func mustBitcoindPersistedConfigForAdminTest(t *testing.T, configDirectory string) ldk.PersistedConfig {
	t.Helper()

	config, err := ldk.NewPersistedConfig(ldk.RPCConfig{
		Address:  "127.0.0.1",
		Port:     18443,
		Username: "user",
		Password: "pass",
	}, configDirectory)
	if err != nil {
		t.Fatalf("ldk.NewPersistedConfig(...): %v", err)
	}

	return config
}

func TestCheckIntegerFromStringSuccess(t *testing.T) {
	text := "2"
	int, err := checkLimitSat(text)

	if err != nil {
		t.Error("Check limit should have work")
	}

	success := 2
	if *int != success {
		t.Error("Conversion should have occurred")
	}
}

func TestCheckIntegerFromStringFailureBool(t *testing.T) {
	text := "2.2"
	_, err := checkLimitSat(text)

	if err == nil {
		t.Error("Check limit should have failed. Because it should not allow float")
	}

}

func mustNpub(t *testing.T) string {
	t.Helper()

	privateKey := nostr.GeneratePrivateKey()
	publicKey, err := nostr.GetPublicKey(privateKey)
	if err != nil {
		t.Fatalf("nostr.GetPublicKey(privateKey): %v", err)
	}

	npub, err := nip19.EncodePublicKey(publicKey)
	if err != nil {
		t.Fatalf("nip19.EncodePublicKey(publicKey): %v", err)
	}

	return npub
}

func setTempConfigDir(t *testing.T) string {
	t.Helper()
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	return filepath.Join(configHome, utils.ConfigDirName)
}

func TestParseNpubToWrappedPublicKey(t *testing.T) {
	npub := mustNpub(t)

	wrapped, err := parseNpubToWrappedPublicKey(npub)
	if err != nil {
		t.Fatalf("parseNpubToWrappedPublicKey(npub): %v", err)
	}

	if wrapped.PublicKey == nil {
		t.Fatal("expected wrapped public key to be non nil")
	}
}

func TestParseNpubArrayToWrappedPublicKeysFailsOnInvalidNpub(t *testing.T) {
	npub := mustNpub(t)

	_, err := parseNpubArrayToWrappedPublicKeys([]string{npub, "invalid_npub"})
	if err == nil {
		t.Fatal("expected parse to fail for invalid npub")
	}
}

func TestMintSettingsNotificationsFailsWithNotificationOnInvalidNpub(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	form := url.Values{}
	form.Set("NOSTR_NOTIFICATIONS", "on")
	form.Add("NOSTR_NOTIFICATION_NPUBS", "not_an_npub")

	req, err := http.NewRequest(http.MethodPost, "/admin/mintsettings/notifications", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("http.NewRequest(...): %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx.Request = req

	var config utils.Config
	config.Default()
	var mintInstance mint.Mint
	mintInstance.Config = config

	var mockDatabase mockdb.MockDB
	mockDatabase.Config = config
	mintInstance.MintDB = &mockDatabase

	handler := MintSettingsNotifications(&mintInstance)
	handler(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, "Nostr notification npub is not valid") {
		t.Fatalf("expected error notification body, got %q", body)
	}
}

func TestMintSettingsNotificationsRendersComponentOnSuccess(t *testing.T) {
	configDir := setTempConfigDir(t)
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	npub := mustNpub(t)
	form := url.Values{}
	form.Set("NOSTR_NOTIFICATIONS", "on")
	form.Set("NOSTR_NOTIFICATION_NIP04_DM", "on")
	form.Add("NOSTR_NOTIFICATION_NPUBS", npub)

	req, err := http.NewRequest(http.MethodPost, "/admin/mintsettings/notifications", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("http.NewRequest(...): %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx.Request = req

	var config utils.Config
	config.Default()
	var mintInstance mint.Mint
	mintInstance.Config = config

	var mockDatabase mockdb.MockDB
	mockDatabase.Config = config
	mintInstance.MintDB = &mockDatabase

	handler := MintSettingsNotifications(&mintInstance)
	handler(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, "id=\"nostr-notifications-form\"") {
		t.Fatalf("expected notifications component in response, got %q", body)
	}
	if !strings.Contains(body, "Nostr notification settings successfully set") {
		t.Fatalf("expected success notification in response, got %q", body)
	}
	if !strings.Contains(body, "js-copy-npub-btn") {
		t.Fatalf("expected copy button class in response, got %q", body)
	}
	if !strings.Contains(body, "nostr-copy-feedback") {
		t.Fatalf("expected copy feedback element in response, got %q", body)
	}
	if !strings.Contains(body, "/admin/mintsettings/notifications/test") {
		t.Fatalf("expected test notification endpoint in response, got %q", body)
	}
	if !strings.Contains(body, "Test notification") {
		t.Fatalf("expected test notification button in response, got %q", body)
	}

	if mintInstance.NostrNotificationConfig == nil {
		t.Fatal("expected nostr notification config to be set")
	}

	if !mintInstance.NostrNotificationConfig.NOSTR_NOTIFICATION_NIP04_DM {
		t.Fatal("expected NOSTR_NOTIFICATION_NIP04_DM to be enabled")
	}

	if _, err := os.Stat(filepath.Join(configDir, utils.NostrNotificationNsecFileName)); err != nil {
		t.Fatalf("expected nostr notification nsec file to be created: %v", err)
	}
}

func TestMintSettingsNotificationsTestRequiresEnabledNotifications(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	req, err := http.NewRequest(http.MethodPost, "/admin/mintsettings/notifications/test", nil)
	if err != nil {
		t.Fatalf("http.NewRequest(...): %v", err)
	}
	ctx.Request = req

	var config utils.Config
	config.Default()

	var mintInstance mint.Mint
	mintInstance.Config = config

	handler := MintSettingsNotificationsTest(&mintInstance)
	handler(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, "Enable nostr notifications first") {
		t.Fatalf("expected disabled-notifications error in response, got %q", body)
	}
}

func TestMintSettingsNotificationsTestWritesSuccessNotification(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	req, err := http.NewRequest(http.MethodPost, "/admin/mintsettings/notifications/test", nil)
	if err != nil {
		t.Fatalf("http.NewRequest(...): %v", err)
	}
	ctx.Request = req

	var config utils.Config
	config.Default()

	var mintInstance mint.Mint
	mintInstance.Config = config
	var nostrNotificationConfig utils.NostrNotificationConfig
	nostrNotificationConfig.NOSTR_NOTIFICATIONS = true
	mintInstance.NostrNotificationConfig = &nostrNotificationConfig

	handler := MintSettingsNotificationsTest(&mintInstance)
	handler(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, "Test error log has been written") {
		t.Fatalf("expected success notification in response, got %q", body)
	}
}

func TestMintSettingsNotificationDeleteNpub(t *testing.T) {
	configDir := setTempConfigDir(t)
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	npub := mustNpub(t)
	wrapped, err := parseNpubToWrappedPublicKey(npub)
	if err != nil {
		t.Fatalf("parseNpubToWrappedPublicKey(npub): %v", err)
	}

	var config utils.Config
	config.Default()
	var nostrConfig utils.NostrNotificationConfig
	if err := nostrConfig.SetNostrNotificationConfig(true, nil, []cashu.WrappedPublicKey{wrapped}); err != nil {
		t.Fatalf("nostrConfig.SetNostrNotificationConfig(...): %v", err)
	}

	nsec, err := utils.ReadOrCreateNostrNotificationNsec(nil)
	if err != nil {
		t.Fatalf("utils.ReadOrCreateNostrNotificationNsec(nil): %v", err)
	}
	nostrConfig.NOSTR_NOTIFICATION_NSEC = nsec

	var mintInstance mint.Mint
	mintInstance.Config = config
	mintInstance.NostrNotificationConfig = &nostrConfig

	var mockDatabase mockdb.MockDB
	mockDatabase.Config = config
	mockDatabase.NostrNotificationConfig = &nostrConfig
	mintInstance.MintDB = &mockDatabase

	req, err := http.NewRequest(http.MethodDelete, "/admin/mintsettings/notifications/npubs/"+npub, nil)
	if err != nil {
		t.Fatalf("http.NewRequest(...): %v", err)
	}
	ctx.Request = req
	ctx.Params = gin.Params{{Key: "npub", Value: npub}}

	handler := MintSettingsNotificationDeleteNpub(&mintInstance)
	handler(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	if mintInstance.NostrNotificationConfig == nil {
		t.Fatal("expected nostr notification config to remain available after delete")
	}

	if len(mintInstance.NostrNotificationConfig.NOSTR_NOTIFICATION_NPUBS) != 0 {
		t.Fatalf("expected npub list to be empty after delete, got %d", len(mintInstance.NostrNotificationConfig.NOSTR_NOTIFICATION_NPUBS))
	}

	body := recorder.Body.String()
	if !strings.Contains(body, "Nostr recipient deleted") {
		t.Fatalf("expected delete notification in response, got %q", body)
	}

	if _, err := os.Stat(filepath.Join(configDir, utils.NostrNotificationNsecFileName)); err != nil {
		t.Fatalf("expected nostr notification nsec file to exist after delete: %v", err)
	}
}

func TestMintSettingsNotificationsKeepsNpubsWhenDisabled(t *testing.T) {
	configDir := setTempConfigDir(t)
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	npub := mustNpub(t)
	wrapped, err := parseNpubToWrappedPublicKey(npub)
	if err != nil {
		t.Fatalf("parseNpubToWrappedPublicKey(npub): %v", err)
	}

	var config utils.Config
	config.Default()
	var nostrConfig utils.NostrNotificationConfig
	if err := nostrConfig.SetNostrNotificationConfig(true, nil, []cashu.WrappedPublicKey{wrapped}); err != nil {
		t.Fatalf("nostrConfig.SetNostrNotificationConfig(...): %v", err)
	}

	nsec, err := utils.ReadOrCreateNostrNotificationNsec(nil)
	if err != nil {
		t.Fatalf("utils.ReadOrCreateNostrNotificationNsec(nil): %v", err)
	}
	nostrConfig.NOSTR_NOTIFICATION_NSEC = nsec

	var mintInstance mint.Mint
	mintInstance.Config = config
	mintInstance.NostrNotificationConfig = &nostrConfig

	var mockDatabase mockdb.MockDB
	mockDatabase.Config = config
	mockDatabase.NostrNotificationConfig = &nostrConfig
	mintInstance.MintDB = &mockDatabase

	req, err := http.NewRequest(http.MethodPost, "/admin/mintsettings/notifications", strings.NewReader(url.Values{}.Encode()))
	if err != nil {
		t.Fatalf("http.NewRequest(...): %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx.Request = req

	handler := MintSettingsNotifications(&mintInstance)
	handler(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	if mintInstance.NostrNotificationConfig == nil {
		t.Fatal("expected nostr notification config to remain available")
	}

	if mintInstance.NostrNotificationConfig.NOSTR_NOTIFICATIONS {
		t.Fatal("expected notifications to be disabled")
	}

	if len(mintInstance.NostrNotificationConfig.NOSTR_NOTIFICATION_NPUBS) != 1 {
		t.Fatalf("expected npub list to be preserved, got %d", len(mintInstance.NostrNotificationConfig.NOSTR_NOTIFICATION_NPUBS))
	}

	if mintInstance.NostrNotificationConfig.NOSTR_NOTIFICATION_NIP04_DM {
		t.Fatal("expected NOSTR_NOTIFICATION_NIP04_DM to be disabled")
	}

	if _, err := os.Stat(filepath.Join(configDir, utils.NostrNotificationNsecFileName)); err != nil {
		t.Fatalf("expected nostr notification nsec file to be preserved: %v", err)
	}
}

func TestMintSettingsNotificationsDedupeNpubs(t *testing.T) {
	setTempConfigDir(t)
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	npub := mustNpub(t)
	form := url.Values{}
	form.Set("NOSTR_NOTIFICATIONS", "on")
	form.Add("NOSTR_NOTIFICATION_NPUBS", npub)
	form.Add("NOSTR_NOTIFICATION_NPUBS", npub)

	req, err := http.NewRequest(http.MethodPost, "/admin/mintsettings/notifications", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("http.NewRequest(...): %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx.Request = req

	var config utils.Config
	config.Default()
	var mintInstance mint.Mint
	mintInstance.Config = config

	var mockDatabase mockdb.MockDB
	mockDatabase.Config = config
	mintInstance.MintDB = &mockDatabase

	handler := MintSettingsNotifications(&mintInstance)
	handler(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	if mintInstance.NostrNotificationConfig == nil {
		t.Fatal("expected nostr notification config to be set")
	}

	if len(mintInstance.NostrNotificationConfig.NOSTR_NOTIFICATION_NPUBS) != 1 {
		t.Fatalf("expected npub list to be deduplicated to 1, got %d", len(mintInstance.NostrNotificationConfig.NOSTR_NOTIFICATION_NPUBS))
	}
}

func TestMintSettingsNotificationDeleteNpubNotFoundReturnsError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	npubStored := mustNpub(t)
	npubDelete := mustNpub(t)
	storedKey, err := parseNpubToWrappedPublicKey(npubStored)
	if err != nil {
		t.Fatalf("parseNpubToWrappedPublicKey(npubStored): %v", err)
	}

	var config utils.Config
	config.Default()
	var nostrConfig utils.NostrNotificationConfig
	if err := nostrConfig.SetNostrNotificationConfig(true, nil, []cashu.WrappedPublicKey{storedKey}); err != nil {
		t.Fatalf("nostrConfig.SetNostrNotificationConfig(...): %v", err)
	}

	var mintInstance mint.Mint
	mintInstance.Config = config
	mintInstance.NostrNotificationConfig = &nostrConfig

	var mockDatabase mockdb.MockDB
	mockDatabase.Config = config
	mockDatabase.NostrNotificationConfig = &nostrConfig
	mintInstance.MintDB = &mockDatabase

	req, err := http.NewRequest(http.MethodDelete, "/admin/mintsettings/notifications/npubs/"+npubDelete, nil)
	if err != nil {
		t.Fatalf("http.NewRequest(...): %v", err)
	}
	ctx.Request = req
	ctx.Params = gin.Params{{Key: "npub", Value: npubDelete}}

	handler := MintSettingsNotificationDeleteNpub(&mintInstance)
	handler(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, "Nostr recipient was not found") {
		t.Fatalf("expected not-found error notification, got %q", body)
	}

	if mintInstance.NostrNotificationConfig == nil {
		t.Fatal("expected nostr notification config to remain set")
	}

	if len(mintInstance.NostrNotificationConfig.NOSTR_NOTIFICATION_NPUBS) != 1 {
		t.Fatalf("expected npub list unchanged, got %d", len(mintInstance.NostrNotificationConfig.NOSTR_NOTIFICATION_NPUBS))
	}
}

func TestMintSettingsNotificationsDoesNotMutateConfigOnDBFailure(t *testing.T) {
	configDir := setTempConfigDir(t)
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	npub := mustNpub(t)
	form := url.Values{}
	form.Set("NOSTR_NOTIFICATIONS", "on")
	form.Add("NOSTR_NOTIFICATION_NPUBS", npub)

	req, err := http.NewRequest(http.MethodPost, "/admin/mintsettings/notifications", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("http.NewRequest(...): %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx.Request = req

	var config utils.Config
	config.Default()

	var mintInstance mint.Mint
	mintInstance.Config = config

	var mockDatabase mockdb.MockDB
	mockDatabase.Config = config
	mockDatabase.UpdateNostrNotificationConfigErr = errors.New("db down")
	mintInstance.MintDB = &mockDatabase

	handler := MintSettingsNotifications(&mintInstance)
	handler(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, "Could not persist nostr notification settings") {
		t.Fatalf("expected persist error notification, got %q", body)
	}

	if mintInstance.NostrNotificationConfig != nil && mintInstance.NostrNotificationConfig.NOSTR_NOTIFICATIONS {
		t.Fatal("expected in-memory config to remain unchanged after DB failure")
	}

	if mintInstance.NostrNotificationConfig != nil && len(mintInstance.NostrNotificationConfig.NOSTR_NOTIFICATION_NPUBS) != 0 {
		t.Fatalf("expected npub list to remain unchanged after DB failure, got %d", len(mintInstance.NostrNotificationConfig.NOSTR_NOTIFICATION_NPUBS))
	}

	if _, err := os.Stat(filepath.Join(configDir, utils.NostrNotificationNsecFileName)); err != nil {
		t.Fatalf("expected nostr notification nsec file to still be created before DB failure: %v", err)
	}
}

func TestParseLDKPersistedConfig(t *testing.T) {
	configDirectory := t.TempDir()
	values := url.Values{}
	values.Set("LDK_CHAIN_SOURCE_TYPE", string(ldk.ChainSourceBitcoind))
	values.Set("BITCOIN_NODE_RPC_ADDRESS", "127.0.0.1")
	values.Set("BITCOIN_NODE_RPC_PORT", "18443")
	values.Set("BITCOIN_NODE_RPC_USERNAME", "user")
	values.Set("BITCOIN_NODE_RPC_PASSWORD", "pass")

	c := newPostContext(values)

	config, err := parseLDKPersistedConfig(c, ldk.PersistedConfig{ConfigDirectory: configDirectory, ChainSourceType: ldk.ChainSourceBitcoind}, configDirectory)
	if err != nil {
		t.Fatalf("parseLDKPersistedConfig(c): %v", err)
	}
	if config.ChainSourceType != ldk.ChainSourceBitcoind {
		t.Fatalf("unexpected chain source type: %q", config.ChainSourceType)
	}
	if config.Rpc.Address != "127.0.0.1" || config.Rpc.Port != 18443 || config.Rpc.Username != "user" || config.Rpc.Password != "pass" {
		t.Fatalf("unexpected parsed rpc config: %+v", config.Rpc)
	}
	if config.ConfigDirectory != configDirectory {
		t.Fatalf("unexpected config directory: %q", config.ConfigDirectory)
	}
}

func TestParseLDKPersistedConfigPreservesExistingBitcoindPassword(t *testing.T) {
	configDirectory := t.TempDir()
	values := url.Values{}
	values.Set("LDK_CHAIN_SOURCE_TYPE", string(ldk.ChainSourceBitcoind))
	values.Set("BITCOIN_NODE_RPC_ADDRESS", "127.0.0.1")
	values.Set("BITCOIN_NODE_RPC_PORT", "18443")
	values.Set("BITCOIN_NODE_RPC_USERNAME", "user")

	c := newPostContext(values)
	existingConfig := mustBitcoindPersistedConfigForAdminTest(t, configDirectory)

	config, err := parseLDKPersistedConfig(c, existingConfig, configDirectory)
	if err != nil {
		t.Fatalf("parseLDKPersistedConfig(c): %v", err)
	}
	if config.Rpc.Password != existingConfig.Rpc.Password {
		t.Fatalf("expected existing password to be preserved")
	}
}

func TestParseLDKPersistedConfigElectrum(t *testing.T) {
	configDirectory := t.TempDir()
	values := url.Values{}
	values.Set("LDK_CHAIN_SOURCE_TYPE", string(ldk.ChainSourceElectrum))
	values.Set("ELECTRUM_SERVER_URL", "ssl://electrum.example:50002")

	c := newPostContext(values)
	existingConfig := mustBitcoindPersistedConfigForAdminTest(t, configDirectory)

	config, err := parseLDKPersistedConfig(c, existingConfig, configDirectory)
	if err != nil {
		t.Fatalf("parseLDKPersistedConfig(c): %v", err)
	}
	if config.ChainSourceType != ldk.ChainSourceElectrum {
		t.Fatalf("unexpected chain source type: %q", config.ChainSourceType)
	}
	if config.ElectrumServerURL != "ssl://electrum.example:50002" {
		t.Fatalf("unexpected electrum server url: %q", config.ElectrumServerURL)
	}
	if config.Rpc.Password != existingConfig.Rpc.Password {
		t.Fatalf("expected inactive bitcoind config to be preserved")
	}
}

func TestParseLDKPersistedConfigEsplora(t *testing.T) {
	configDirectory := t.TempDir()
	values := url.Values{}
	values.Set("LDK_CHAIN_SOURCE_TYPE", string(ldk.ChainSourceEsplora))
	values.Set("ESPLORA_SERVER_URL", "https://blockstream.info/api")

	c := newPostContext(values)
	existingConfig := mustBitcoindPersistedConfigForAdminTest(t, configDirectory)

	config, err := parseLDKPersistedConfig(c, existingConfig, configDirectory)
	if err != nil {
		t.Fatalf("parseLDKPersistedConfig(c): %v", err)
	}
	if config.ChainSourceType != ldk.ChainSourceEsplora {
		t.Fatalf("unexpected chain source type: %q", config.ChainSourceType)
	}
	if config.EsploraServerURL != "https://blockstream.info/api" {
		t.Fatalf("unexpected esplora server url: %q", config.EsploraServerURL)
	}
	if config.Rpc.Password != existingConfig.Rpc.Password {
		t.Fatalf("expected inactive bitcoind config to be preserved")
	}
}

func TestParseLDKPersistedConfigRejectsInvalidPort(t *testing.T) {
	configDirectory := t.TempDir()
	values := url.Values{}
	values.Set("LDK_CHAIN_SOURCE_TYPE", string(ldk.ChainSourceBitcoind))
	values.Set("BITCOIN_NODE_RPC_ADDRESS", "127.0.0.1")
	values.Set("BITCOIN_NODE_RPC_PORT", "70000")
	values.Set("BITCOIN_NODE_RPC_USERNAME", "user")
	values.Set("BITCOIN_NODE_RPC_PASSWORD", "pass")

	c := newPostContext(values)

	_, err := parseLDKPersistedConfig(c, ldk.PersistedConfig{ConfigDirectory: configDirectory, ChainSourceType: ldk.ChainSourceBitcoind}, configDirectory)
	if err == nil {
		t.Fatalf("expected invalid port error")
	}
}

func TestParseLDKPersistedConfigRejectsInvalidElectrumURL(t *testing.T) {
	configDirectory := t.TempDir()
	values := url.Values{}
	values.Set("LDK_CHAIN_SOURCE_TYPE", string(ldk.ChainSourceElectrum))
	values.Set("ELECTRUM_SERVER_URL", "electrum.example:50002")

	c := newPostContext(values)

	_, err := parseLDKPersistedConfig(c, ldk.PersistedConfig{ConfigDirectory: configDirectory, ChainSourceType: ldk.ChainSourceBitcoind}, configDirectory)
	if err == nil {
		t.Fatalf("expected invalid electrum url error")
	}
}

func TestParseLDKPersistedConfigRejectsInvalidEsploraURL(t *testing.T) {
	configDirectory := t.TempDir()
	values := url.Values{}
	values.Set("LDK_CHAIN_SOURCE_TYPE", string(ldk.ChainSourceEsplora))
	values.Set("ESPLORA_SERVER_URL", "blockstream.info/api")

	c := newPostContext(values)

	_, err := parseLDKPersistedConfig(c, ldk.PersistedConfig{ConfigDirectory: configDirectory, ChainSourceType: ldk.ChainSourceBitcoind}, configDirectory)
	if err == nil {
		t.Fatalf("expected invalid esplora url error")
	}
}

func TestParseLDKPersistedConfigRejectsInvalidConfigDirectory(t *testing.T) {
	values := url.Values{}
	values.Set("LDK_CHAIN_SOURCE_TYPE", string(ldk.ChainSourceBitcoind))
	values.Set("BITCOIN_NODE_RPC_ADDRESS", "127.0.0.1")
	values.Set("BITCOIN_NODE_RPC_PORT", "18443")
	values.Set("BITCOIN_NODE_RPC_USERNAME", "user")
	values.Set("BITCOIN_NODE_RPC_PASSWORD", "pass")

	c := newPostContext(values)

	_, err := parseLDKPersistedConfig(c, ldk.PersistedConfig{ConfigDirectory: "relative/ldk", ChainSourceType: ldk.ChainSourceBitcoind}, "relative/ldk")
	if err == nil {
		t.Fatalf("expected invalid config directory error")
	}
}

func TestLDKConfigsEqual(t *testing.T) {
	a := ldk.PersistedConfig{
		ChainSourceType: ldk.ChainSourceBitcoind,
		Rpc: ldk.RPCConfig{
			Address:  "127.0.0.1",
			Port:     18443,
			Username: "user",
			Password: "pass",
		},
		ConfigDirectory: "/tmp/ldk-a",
	}
	b := a

	if !ldkConfigsEqual(a, b) {
		t.Fatalf("expected configs to be equal")
	}

	b.Rpc.Port = 8332
	if ldkConfigsEqual(a, b) {
		t.Fatalf("expected configs to differ")
	}

	b = a
	b.ChainSourceType = ldk.ChainSourceElectrum
	b.ElectrumServerURL = "ssl://electrum.example:50002"
	if ldkConfigsEqual(a, b) {
		t.Fatalf("expected chain source types to differ")
	}

	b = a
	b.ChainSourceType = ldk.ChainSourceEsplora
	b.EsploraServerURL = "https://blockstream.info/api"
	if ldkConfigsEqual(a, b) {
		t.Fatalf("expected chain source types to differ")
	}

	b = a
	b.ConfigDirectory = "/tmp/ldk-b"
	if ldkConfigsEqual(a, b) {
		t.Fatalf("expected config directories to differ")
	}
}
