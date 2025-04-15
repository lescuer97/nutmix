package utils

import (
	"crypto/rand"
	"encoding/base64"
	"github.com/lescuer97/nutmix/internal/lightning"
	"os"
)

const ConfigFileName string = "config.toml"
const ConfigDirName string = "nutmix"
const LogFileName string = "nutmix.log"

type LightningBackend string

const FAKE_WALLET LightningBackend = "FakeWallet"
const LNDGRPC LightningBackend = "LndGrpcWallet"
const LNBITS LightningBackend = "LNbitsWallet"
const CLNGRPC LightningBackend = "ClnGrpcWallet"
const Strike LightningBackend = "Strike"

func StringToLightningBackend(text string) LightningBackend {

	switch text {
	case string(FAKE_WALLET):
		return FAKE_WALLET
	case string(LNDGRPC):
		return LNDGRPC
	case string(LNBITS):
		return LNBITS
	case string(Strike):
		return Strike
	default:
		return FAKE_WALLET

	}
}

type Config struct {
	NAME             string `db:"name"`
	DESCRIPTION      string `db:"description"`
	DESCRIPTION_LONG string `db:"description_long"`
	MOTD             string `db:"motd"`
	EMAIL            string `db:"email"`
	NOSTR            string `db:"nostr"`

	NETWORK string `db:"network"`

	MINT_LIGHTNING_BACKEND LightningBackend `db:"mint_lightning_backend"`
	LND_GRPC_HOST          string           `db:"lnd_grpc_host"`
	LND_TLS_CERT           string           `db:"lnd_tls_cert"`
	LND_MACAROON           string           `db:"lnd_macaroon"`

	MINT_LNBITS_ENDPOINT string `db:"mint_lnbits_endpoint"`
	MINT_LNBITS_KEY      string `db:"mint_lnbits_key"`

	CLN_GRPC_HOST   string `db:"cln_grpc_host"`
	CLN_CA_CERT     string `db:"cln_ca_cert"`
	CLN_CLIENT_CERT string `db:"cln_client_cert"`
	CLN_CLIENT_KEY  string `db:"cln_client_key"`
	CLN_MACAROON    string `db:"cln_macaroon"`

	STRIKE_KEY      string `db:"strike_key"`
	STRIKE_ENDPOINT string `db:"strike_endpoint"`

	PEG_OUT_ONLY       bool `db:"peg_out_only"`
	PEG_OUT_LIMIT_SATS *int `db:"peg_out_limit_sats,omitempty"`
	PEG_IN_LIMIT_SATS  *int `db:"peg_in_limit_sats,omitempty"`

	MINT_REQUIRE_AUTH               bool   `db:"mint_require_auth,omitempty"`
	MINT_AUTH_OICD_DISCOVERY_URL    string `db:"mint_auth_discovery_url,omitempty"`
	MINT_AUTH_OICD_CLIENT_ID        string `db:"mint_auth_oicd_client_id,omitempty"`
	MINT_AUTH_RATE_LIMIT_PER_MINUTE int    `db:"mint_auth_rate_limit_per_minute,omitempty"`
	MINT_AUTH_MAX_BLIND_TOKENS      uint64 `db:"mint_auth_max_blind_tokens,omitempty"`

	MINT_AUTH_CLEAR_AUTH_URLS []string `db:"mint_auth_clear_auth_urls,omitempty"`
	MINT_AUTH_BLIND_AUTH_URLS []string `db:"mint_auth_blind_auth_urls,omitempty"`
}

func (c *Config) Default() {
	c.NAME = ""
	c.DESCRIPTION = ""
	c.DESCRIPTION_LONG = ""
	c.MOTD = ""
	c.EMAIL = ""
	c.NOSTR = ""

	c.NETWORK = lightning.MAINNET

	c.MINT_LIGHTNING_BACKEND = FAKE_WALLET

	c.LND_GRPC_HOST = ""
	c.LND_TLS_CERT = ""
	c.LND_MACAROON = ""

	c.MINT_LNBITS_ENDPOINT = ""
	c.MINT_LNBITS_KEY = ""

	c.PEG_OUT_ONLY = false
	c.PEG_OUT_LIMIT_SATS = nil
	c.PEG_IN_LIMIT_SATS = nil

	c.MINT_REQUIRE_AUTH = false
	c.MINT_AUTH_OICD_CLIENT_ID = ""
	c.MINT_AUTH_MAX_BLIND_TOKENS = 100
	c.MINT_AUTH_OICD_DISCOVERY_URL = ""
	c.MINT_AUTH_RATE_LIMIT_PER_MINUTE = 5
	c.MINT_AUTH_CLEAR_AUTH_URLS = []string{}
	c.MINT_AUTH_BLIND_AUTH_URLS = []string{}
	c.STRIKE_KEY = ""
}

func (c *Config) UseEnviromentVars() {
	c.NAME = os.Getenv("NAME")
	c.DESCRIPTION = os.Getenv("DESCRIPTION")
	c.DESCRIPTION_LONG = os.Getenv("DESCRIPTION_LONG")
	c.MOTD = os.Getenv("MOTD")
	c.EMAIL = os.Getenv("EMAIL")
	c.NOSTR = os.Getenv("NOSTR")

	c.NETWORK = os.Getenv("NETWORK")

	c.MINT_LIGHTNING_BACKEND = StringToLightningBackend(os.Getenv("MINT_LIGHTNING_BACKEND"))

	c.LND_GRPC_HOST = os.Getenv("LND_GRPC_HOST")
	c.LND_TLS_CERT = os.Getenv("LND_TLS_CERT")
	c.LND_MACAROON = os.Getenv("LND_MACAROON")

	c.MINT_LNBITS_ENDPOINT = os.Getenv("MINT_LNBITS_ENDPOINT")
	c.MINT_LNBITS_KEY = os.Getenv("MINT_LNBITS_KEY")

}
func RandomHash() (string, error) {
	// Create a byte slice of 30 random bytes
	randomBytes := make([]byte, 30)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", err
	}

	// Encode the random bytes as base64-urlsafe string
	return base64.URLEncoding.EncodeToString(randomBytes), nil
}
