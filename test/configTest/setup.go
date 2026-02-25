package configTest

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lescuer97/nutmix/internal/utils"
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
	pathToProjectDir, err := utils.GetConfigDirectory()
	if err != nil {
		return fmt.Errorf("utils.GetConfigDirectory(), %w", err)
	}
	pathToProjectConfigFile := filepath.Join(pathToProjectDir, utils.ConfigFileName)

	err = utils.CreateDirectoryAndPath(pathToProjectDir, utils.ConfigFileName)
	if err != nil {
		return fmt.Errorf("utils.CreateDirectoryAndPath(pathToProjectDir, utils.ConfigFileName), %w", err)
	}

	err = os.WriteFile(pathToProjectConfigFile, file, 0600)
	if err != nil {
		return fmt.Errorf("os.WriteFile(pathToProjectConfigFile, file, 0764), %w", err)
	}

	return nil
}
