package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	mockdb "github.com/lescuer97/nutmix/internal/database/mock_db"
	"github.com/lescuer97/nutmix/internal/lightning/ldk"
	m "github.com/lescuer97/nutmix/internal/mint"
)

func TestGetLDKFormValuesUsesPersistedConfigWithoutActiveBackend(t *testing.T) {
	gin.SetMode(gin.TestMode)
	configDirectory := t.TempDir()
	db := &mockdb.MockDB{}
	persistedConfig, err := ldk.NewPersistedConfigWithChainSource(
		ldk.ChainSourceElectrum,
		ldk.RPCConfig{Address: "127.0.0.1", Port: 18443, Username: "user", Password: "pass"},
		"ssl://electrum.example:50002",
		"",
		configDirectory,
	)
	if err != nil {
		t.Fatalf("ldk.NewPersistedConfigWithChainSource(...): %v", err)
	}
	if err := ldk.SaveConfig(context.Background(), db, persistedConfig); err != nil {
		t.Fatalf("ldk.SaveConfig(...): %v", err)
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/lightningdata", nil)

	formValues := getLDKFormValues(c, &m.Mint{MintDB: db})
	if formValues.ChainSourceType != string(ldk.ChainSourceElectrum) {
		t.Fatalf("unexpected chain source type: %q", formValues.ChainSourceType)
	}
	if formValues.ElectrumServerURL != "ssl://electrum.example:50002" {
		t.Fatalf("unexpected electrum server url: %q", formValues.ElectrumServerURL)
	}
	if formValues.Password != "" {
		t.Fatalf("expected persisted password to stay hidden")
	}
}

func TestGetLDKFormValuesPrefersRequestValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	configDirectory := t.TempDir()
	db := &mockdb.MockDB{}
	persistedConfig, err := ldk.NewPersistedConfig(ldk.RPCConfig{
		Address:  "127.0.0.1",
		Port:     18443,
		Username: "user",
		Password: "pass",
	}, configDirectory)
	if err != nil {
		t.Fatalf("ldk.NewPersistedConfig(...): %v", err)
	}
	if err := ldk.SaveConfig(context.Background(), db, persistedConfig); err != nil {
		t.Fatalf("ldk.SaveConfig(...): %v", err)
	}

	values := url.Values{}
	values.Set("LDK_CHAIN_SOURCE_TYPE", string(ldk.ChainSourceElectrum))
	values.Set("BITCOIN_NODE_RPC_ADDRESS", "10.0.0.2")
	values.Set("BITCOIN_NODE_RPC_PORT", "8332")
	values.Set("BITCOIN_NODE_RPC_USERNAME", "override-user")
	values.Set("BITCOIN_NODE_RPC_PASSWORD", "override-pass")
	values.Set("ELECTRUM_SERVER_URL", "ssl://override.example:50002")

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/admin/lightningdata", nil)
	req.PostForm = values
	req.Form = values
	c.Request = req

	formValues := getLDKFormValues(c, &m.Mint{MintDB: db})
	if formValues.ChainSourceType != string(ldk.ChainSourceElectrum) {
		t.Fatalf("unexpected chain source type: %q", formValues.ChainSourceType)
	}
	if formValues.Address != "10.0.0.2" || formValues.Port != "8332" || formValues.Username != "override-user" || formValues.Password != "override-pass" {
		t.Fatalf("unexpected overridden bitcoind form values: %+v", formValues)
	}
	if formValues.ElectrumServerURL != "ssl://override.example:50002" {
		t.Fatalf("unexpected overridden electrum url: %q", formValues.ElectrumServerURL)
	}
}

func TestGetLDKFormValuesSupportsEsploraValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	configDirectory := t.TempDir()
	db := &mockdb.MockDB{}
	persistedConfig, err := ldk.NewPersistedConfig(ldk.RPCConfig{
		Address:  "127.0.0.1",
		Port:     18443,
		Username: "user",
		Password: "pass",
	}, configDirectory)
	if err != nil {
		t.Fatalf("ldk.NewPersistedConfig(...): %v", err)
	}
	if err := ldk.SaveConfig(context.Background(), db, persistedConfig); err != nil {
		t.Fatalf("ldk.SaveConfig(...): %v", err)
	}

	values := url.Values{}
	values.Set("LDK_CHAIN_SOURCE_TYPE", string(ldk.ChainSourceEsplora))
	values.Set("ESPLORA_SERVER_URL", "https://mempool.space/api")

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/admin/lightningdata", nil)
	req.PostForm = values
	req.Form = values
	c.Request = req

	formValues := getLDKFormValues(c, &m.Mint{MintDB: db})
	if formValues.ChainSourceType != string(ldk.ChainSourceEsplora) {
		t.Fatalf("unexpected chain source type: %q", formValues.ChainSourceType)
	}
	if formValues.EsploraServerURL != "https://mempool.space/api" {
		t.Fatalf("unexpected overridden esplora url: %q", formValues.EsploraServerURL)
	}
}
