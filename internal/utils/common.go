package utils

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/lescuer97/nutmix/internal/lightning"
)

const ConfigFileName string = "config.toml"
const ConfigDirName string = "nutmix"
const LogFileName string = "nutmix.log"

type LightningBackend string

const FAKE_WALLET LightningBackend = "FakeWallet"
const LNDGRPC LightningBackend = "LndGrpcWallet"
const LNBITS LightningBackend = "LNbitsWallet"
const CLNGRPC LightningBackend = "ClnGrpcWallet"

func StringToLightningBackend(text string) LightningBackend {

	switch text {
	case string(FAKE_WALLET):
		return FAKE_WALLET
	case string(LNDGRPC):
		return LNDGRPC
	case string(LNBITS):
		return LNBITS
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

	PEG_OUT_ONLY       bool `db:"peg_out_only"`
	PEG_OUT_LIMIT_SATS *int `db:"peg_out_limit_sats,omitempty"`
	PEG_IN_LIMIT_SATS  *int `db:"peg_in_limit_sats,omitempty"`
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

func (c *Config) SetTOMLFile() error {
	dir, err := os.UserConfigDir()

	if err != nil {
		return fmt.Errorf("os.UserHomeDir(), %w", err)
	}

	var pathToProjectDir string = dir + "/" + ConfigDirName
	var pathToProjectConfigFile string = pathToProjectDir + "/" + ConfigFileName

	bytes, err := toml.Marshal(c)
	if err != nil {
		return fmt.Errorf("toml.Marshal(c), %w", err)
	}

	err = os.WriteFile(pathToProjectConfigFile, bytes, 0764)

	if err != nil {
		return fmt.Errorf("os.WriteFile(pathToProjectConfigFile, bytes,0764), %w", err)
	}

	return nil
}

func getConfigFile() ([]byte, error) {
	dir, err := os.UserConfigDir()

	if err != nil {
		return []byte{}, fmt.Errorf("os.UserHomeDir(), %w", err)
	}

	var pathToProjectDir string = dir + "/" + ConfigDirName
	var pathToProjectConfigFile string = pathToProjectDir + "/" + ConfigFileName
	err = CreateDirectoryAndPath(pathToProjectDir, ConfigFileName)

	if err != nil {
		return []byte{}, fmt.Errorf("utils.CreateDirectoryAndPath(pathToProjectDir, ConfigFileName), %w", err)
	}

	// Manipulate Config file and parse
	return os.ReadFile(pathToProjectConfigFile)
}
