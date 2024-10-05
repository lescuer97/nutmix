package mint

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/lightning"
	"github.com/lescuer97/nutmix/internal/utils"
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
	NAME             string
	DESCRIPTION      string
	DESCRIPTION_LONG string
	MOTD             string
	EMAIL            string
	NOSTR            string

	NETWORK string

	MINT_LIGHTNING_BACKEND LightningBackend
	LND_GRPC_HOST          string
	LND_TLS_CERT           string
	LND_MACAROON           string

	MINT_LNBITS_ENDPOINT string
	MINT_LNBITS_KEY      string

	CLN_GRPC_HOST   string
	CLN_CA_CERT     string
	CLN_CLIENT_CERT string
	CLN_CLIENT_KEY  string
	CLN_MACAROON    string

	DATABASE_TYPE string
	DATABASE_URL  string

	PEG_OUT_ONLY       bool
	PEG_OUT_LIMIT_SATS *int
	PEG_IN_LIMIT_SATS  *int
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

	c.DATABASE_TYPE = database.DOCKERDATABASE

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

	c.DATABASE_TYPE = database.CUSTOMDATABASE
	c.DATABASE_URL = os.Getenv("DATABASE_URL")

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

func SetUpConfigFile() (Config, error) {
	dir, err := os.UserConfigDir()

	var config Config

	if err != nil {
		return config, fmt.Errorf("os.UserHomeDir(), %w", err)
	}

	var pathToProjectDir string = dir + "/" + ConfigDirName
	var pathToProjectConfigFile string = pathToProjectDir + "/" + ConfigFileName

	err = utils.CreateDirectoryAndPath(pathToProjectDir, ConfigFileName)

	if err != nil {
		return config, fmt.Errorf("utils.CreateDirectoryAndPath(pathToProjectDir, ConfigFileName), %w", err)
	}

	// Manipulate Config file and parse
	buf, err := os.ReadFile(pathToProjectConfigFile)

	err = toml.Unmarshal(buf, &config)
	if err != nil {
		return config, fmt.Errorf("toml.Unmarshal(buf,&config ), %w", err)
	}

	// check if some legacy env variables are set to check if there is a need to migrate
	networkEnv := os.Getenv(NETWORK_ENV)
	mint_lightning_backendEnv := os.Getenv(MINT_LIGHTNING_BACKEND_ENV)

	writeToFile := false

	switch {
	// if env values are set and no config exists on toml file use those.
	// if MINT_LIGHTNING_BACKEND and NETWORK are empty we can assume the file is empty
	case (len(networkEnv) > 0 && len(config.NETWORK) == 0 && len(config.MINT_LIGHTNING_BACKEND) == 0):
		fmt.Println("inside env vars")
		config.UseEnviromentVars()
		writeToFile = true

	// if no config and no env set default to toml
	case (len(networkEnv) == 0 && len(mint_lightning_backendEnv) == 0 && len(config.NETWORK) == 0 && len(config.MINT_LIGHTNING_BACKEND) == 0):
		config.Default()
		writeToFile = true

	default:
		fmt.Println("running default")

		// if valid config value exists use those
	}

	if writeToFile {
		bytesForFile, err := toml.Marshal(config)
		if err != nil {
			return config, fmt.Errorf("toml.Marshal(config), %w", err)
		}

		err = os.WriteFile(pathToProjectConfigFile, bytesForFile, 0764)
		if err != nil {
			return config, fmt.Errorf("f.Write(bytesForFile) %w", err)
		}

	}

	// if not transfer from env file if they exists if not

	return config, nil
}
