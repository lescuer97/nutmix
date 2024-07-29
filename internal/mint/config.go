package mint

import (
	"fmt"
	"io"
	"os"

	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/lightning"
	"github.com/pelletier/go-toml/v2"
)

const ConfigFileName string = "config.toml"
const ConfigDirName string = ".nutmix"

type Config struct {
	NAME             string
	DESCRIPTION      string
	DESCRIPTION_LONG string
	MOTD             string
	EMAIL            string
	NOSTR            string

	NETWORK string

	MINT_LIGHTNING_BACKEND string
	LND_GRPC_HOST          string
	LND_TLS_CERT           string

	MINT_LNBITS_ENDPOINT string
	MINT_LNBITS_KEY      string

	DATABASE_TYPE     string
	DATABASE_URL      string
	POSTGRES_USER     string
	POSTGRES_PASSWORD string

	ADMIN_NOSTR_NPUB string
}

func (c *Config) Default() {
	c.NAME = ""
	c.DESCRIPTION = ""
	c.DESCRIPTION_LONG = ""
	c.MOTD = ""
	c.EMAIL = ""
	c.NOSTR = ""

	c.NETWORK = lightning.MAINNET

	c.MINT_LIGHTNING_BACKEND = ""

	c.LND_GRPC_HOST = ""
	c.LND_TLS_CERT = ""

	c.MINT_LNBITS_ENDPOINT = ""
	c.MINT_LNBITS_KEY = ""

	c.DATABASE_TYPE = database.DOCKERDATABASE
	c.POSTGRES_USER = "admin"
	c.POSTGRES_PASSWORD = ""

	c.ADMIN_NOSTR_NPUB = ""
}
func (c *Config) UseEnviromentVars() {
	c.NAME = os.Getenv("NAME")
	c.DESCRIPTION = os.Getenv("DESCRIPTION")
	c.DESCRIPTION_LONG = os.Getenv("DESCRIPTION_LONG")
	c.MOTD = os.Getenv("MOTD")
	c.EMAIL = os.Getenv("EMAIL")
	c.NOSTR = os.Getenv("NOSTR")

	c.NETWORK = os.Getenv("NETWORK")

	c.MINT_LIGHTNING_BACKEND = os.Getenv("MINT_LIGHTNING_BACKEND")

	c.LND_GRPC_HOST = os.Getenv("LND_GRPC_HOST")
	c.LND_TLS_CERT = os.Getenv("LND_TLS_CERT")

	c.MINT_LNBITS_ENDPOINT = os.Getenv("MINT_LNBITS_ENDPOINT")
	c.MINT_LNBITS_KEY = os.Getenv("MINT_LNBITS_KEY")

	c.DATABASE_TYPE = database.CUSTOMDATABASE
	c.DATABASE_URL = os.Getenv("DATABASE_URL")
	c.POSTGRES_USER = os.Getenv("POSTGRES_USER")
	c.POSTGRES_PASSWORD = os.Getenv("POSTGRES_PASSWORD")

	c.ADMIN_NOSTR_NPUB = os.Getenv("ADMIN_NOSTR_NPUB")
}

func SetUpConfigFile() (Config, error) {
	dir, err := os.UserHomeDir()

	var config Config

	if err != nil {
		return config, fmt.Errorf("os.UserHomeDir(), %w", err)
	}
	var pathToProjectDir string = dir + "/" + ConfigDirName
	var pathToProjectConfigFile string = pathToProjectDir + "/" + ConfigFileName

	_, err = os.Stat(pathToProjectDir)

	if os.IsNotExist(err) {
		err = os.MkdirAll(pathToProjectDir, 0764)
		if err != nil {
			return config, fmt.Errorf("os.MkdirAll(pathToProjectDir, 0764) %w", err)
		}
	}

	_, err = os.Stat(pathToProjectConfigFile)
	if os.IsNotExist(err) {
		_, err := os.Create(pathToProjectConfigFile)
		if err != nil {
			return config, fmt.Errorf("os.Create(pathToProjectConfigFile) %w", err)
		}
	}

	// Manipulate Config file
	f, err := os.OpenFile(pathToProjectConfigFile, os.O_RDWR, 0764)
	if err != nil {
		return config, fmt.Errorf("os.OpenFile(pathToProjectConfigFile, os.O_RDWR, 0764), %w", err)
	}

	var buf []byte
	for {
		// read a chunk
		n, err := f.Read(buf)
		if err != nil && err != io.EOF {
			panic(err)
		}
		if n == 0 {
			break
		}

	}

	err = toml.Unmarshal(buf, &config)
	if err != nil {
		return config, fmt.Errorf("toml.Unmarshal(buf,&config ), %w", err)
	}

	networkEnv := os.Getenv(NETWORK_ENV)
	mint_lightning_backendEnv := os.Getenv(MINT_LIGHTNING_BACKEND_ENV)

	writeToFile := false
	switch {
	// if env values are set and no config exists on toml file use those.
	// if MINT_LIGHTNING_BACKEND and NETWORK are empty we can assume the file is empty
	case len(networkEnv) > 0 && len(mint_lightning_backendEnv) > 0 && len(config.NETWORK) == 0 && len(config.MINT_LIGHTNING_BACKEND) == 0:
		config.UseEnviromentVars()
		writeToFile = true

	// if no config and no env set default to toml
	case len(networkEnv) == 0 && len(mint_lightning_backendEnv) == 0 && len(config.NETWORK) == 0 && len(config.MINT_LIGHTNING_BACKEND) == 0:
		config.Default()
		writeToFile = true

		// if valid config value exists use those
	}

	if writeToFile {
		bytesForFile, err := toml.Marshal(config)
		if err != nil {
			return config, fmt.Errorf("toml.Marshal(config), %w", err)
		}

		_, err = f.Write(bytesForFile)
		if err != nil {
			return config, fmt.Errorf("f.Write(bytesForFile) %w", err)
		}

	}

	// if not transfer from env file if they exists if not

	return config, nil
}
