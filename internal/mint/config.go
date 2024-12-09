package mint

import (
	"database/sql"
	"errors"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/utils"
)

const ConfigFileName string = "config.toml"
const ConfigDirName string = "nutmix"
const LogFileName string = "nutmix.log"

func getConfigFile() ([]byte, error) {
	dir, err := os.UserConfigDir()

	if err != nil {
		return []byte{}, fmt.Errorf("os.UserHomeDir(), %w", err)
	}

	var pathToProjectDir string = dir + "/" + ConfigDirName
	var pathToProjectConfigFile string = pathToProjectDir + "/" + ConfigFileName
	err = utils.CreateDirectoryAndPath(pathToProjectDir, ConfigFileName)

	if err != nil {
		return []byte{}, fmt.Errorf("utils.CreateDirectoryAndPath(pathToProjectDir, ConfigFileName), %w", err)
	}

	// Manipulate Config file and parse
	return os.ReadFile(pathToProjectConfigFile)
}

func SetUpConfigFile() (utils.Config, error) {

	var config utils.Config
	file, err := getConfigFile()

	if err != nil {
		return config, fmt.Errorf("getConfigFile(), %w", err)
	}

	err = toml.Unmarshal(file, &config)
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
		err := config.SetTOMLFile()

		if err != nil {
			return config, fmt.Errorf("config.SetTOMLFile() %w", err)
		}

	}

	// if not transfer from env file if they exists if not

	return config, nil
}

// will not look for os.variable config only file config
func SetUpConfigDB(db database.MintDB) (utils.Config, error) {

	var config utils.Config
	// check if config in db exists if it doesn't check for config file or set default
	config, err := db.GetConfig()
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return config, fmt.Errorf("db.GetConfig(), %w", err)
	}

	if errors.Is(err, sql.ErrNoRows) {
		// check if config file exists
		file, err := getConfigFile()
		if err != nil {
			return config, fmt.Errorf("getConfigFile(), %w", err)
		}

		err = toml.Unmarshal(file, &config)
		if err != nil {
			return config, fmt.Errorf("toml.Unmarshal(buf,&config ), %w", err)
		}

		switch {

		// if no config  set default to toml
		case (len(config.NETWORK) == 0 && len(config.MINT_LIGHTNING_BACKEND) == 0):
			config.Default()

		default:
			fmt.Println("running default")

			// if valid config value exists use those
		}

		// if the file config is set use that to set, if nothing is set do default
		// write to config db
		err = db.SetConfig(config)
		if err != nil {
			return config, fmt.Errorf("db.SetConfig(config) %w", err)
		}
	}

	return config, nil
}
