package configTest

import (
	"fmt"
	"os"

	"github.com/lescuer97/nutmix/internal/mint"
)

type ConfigFiles struct {
	TomlFile []byte
}

func CopyConfigFiles(filepath string) (ConfigFiles, error) {

	var config ConfigFiles

	file, err := os.ReadFile(filepath)

	if err != nil {

		return config, fmt.Errorf("could not read file: %w", err)

	}

	config.TomlFile = file

	return config, nil
}

func RemoveConfigFile(filepath string) error {

	err := os.Remove(filepath)
	if err != nil {
		return fmt.Errorf("os.Remove(), %w", err)
	}

	return nil
}

func WriteConfigFile(file []byte) error {
	dir, err := os.UserConfigDir()

	if err != nil {
		return fmt.Errorf("os.UserHomeDir(), %w", err)
	}
	var pathToProjectDir = dir + "/" + mint.ConfigDirName
	var pathToProjectConfigFile = pathToProjectDir + "/" + mint.ConfigFileName

	err = os.WriteFile(pathToProjectConfigFile, file, 0764)
	if err != nil {
		return fmt.Errorf("os.WriteFile(pathToProjectConfigFile, file, 0764), %w", err)
	}

	return nil
}
