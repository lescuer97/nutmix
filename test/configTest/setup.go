package configTest

import (
	"fmt"
	"os"

	"github.com/lescuer97/nutmix/internal/mint"
)

type ConfigFiles struct {
	TomlFile []byte
}

func CopyConfigFiles() (ConfigFiles, error) {
	dir, err := os.UserHomeDir()

	var config ConfigFiles

	if err != nil {
		return config, fmt.Errorf("os.UserHomeDir(), %w", err)
	}
	var pathToProjectDir string = dir + "/" + mint.ConfigDirName
	var pathToProjectConfigFile string = pathToProjectDir + "/" + mint.ConfigFileName

	file, err := os.ReadFile(pathToProjectConfigFile)

	if err != nil {

		return config, fmt.Errorf("Could not read file %w", err)

	}

	config.TomlFile = file

	return config, nil
}

func RemoveConfigFile() error {
	dir, err := os.UserHomeDir()

	if err != nil {
		return fmt.Errorf("os.UserHomeDir(), %w", err)
	}
	var pathToProjectDir string = dir + "/" + mint.ConfigDirName
	var pathToProjectConfigFile string = pathToProjectDir + "/" + mint.ConfigFileName

	err = os.Remove(pathToProjectConfigFile)
	if err != nil {
		return fmt.Errorf("os.Remove(), %w", err)
	}

	return nil
}

func WriteConfigFile(file []byte) error {
	dir, err := os.UserHomeDir()

	if err != nil {
		return fmt.Errorf("os.UserHomeDir(), %w", err)
	}
	var pathToProjectDir string = dir + "/" + mint.ConfigDirName
	var pathToProjectConfigFile string = pathToProjectDir + "/" + mint.ConfigFileName

	err = os.WriteFile(pathToProjectConfigFile, file, 0764)
	if err != nil {
		return fmt.Errorf("os.WriteFile(pathToProjectConfigFile, file, 0764), %w", err)
	}

	return nil
}
