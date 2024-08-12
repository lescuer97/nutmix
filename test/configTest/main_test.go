package configTest

import (
	"fmt"
	"github.com/lescuer97/nutmix/internal/mint"
	"testing"
)

func TestSetupConfigWithAlreadyExistingEnv(t *testing.T) {
	originalCopyFile, err := CopyConfigFiles()
	if err != nil {
		t.Errorf("Could not setup Config File")
	}

	err = RemoveConfigFile()
	if err != nil {
		t.Errorf("Could not setup Config File")
	}

	// Setup Existing Env Variables

	t.Setenv("NAME", "test-name")
	t.Setenv("DESCRIPTION", "mint description")
	t.Setenv("MOTD", "important")

	t.Setenv("NETWORK", "signet")
	t.Setenv("MINT_LIGHTNING_BACKEND", "LndGrpcWallet")

	// Setup Config
	config, err := mint.SetUpConfigFile()

	if err != nil {
		t.Errorf("Could not setup Config File")
	}

	if config.NAME != "test-name" {
		t.Errorf("Could not check")
	}

	if config.DESCRIPTION != "mint description" {
		t.Errorf("Could not check")
	}

	if config.MOTD != "important" {
		t.Errorf("Could not check")
	}

	if config.NETWORK != "signet" {
		t.Errorf("Could not check")
	}

	if config.MINT_LIGHTNING_BACKEND != "LndGrpcWallet" {
		t.Errorf("Could not check")
	}

	err = WriteConfigFile(originalCopyFile.TomlFile)

	if err != nil {
		t.Errorf("Could not rewrite config file to original %+v", err)
	}

}

func TestSetupConfigWithoutEnvVars(t *testing.T) {
	originalCopyFile, err := CopyConfigFiles()
	if err != nil {
		t.Errorf("Could not setup Config File")
	}

	err = RemoveConfigFile()
	if err != nil {
		t.Errorf("Could not setup Config File")
	}

	// Setup Config
	config, err := mint.SetUpConfigFile()
	if err != nil {
		t.Errorf("Could not setup Config File")
	}
	fmt.Printf("config %+v", config)

	if config.NETWORK != "mainnet" {
		t.Errorf("Network is not default")
	}
	if config.NAME != "" {
		t.Errorf("name is not default")
	}
	if config.MINT_LIGHTNING_BACKEND != "FakeWallet" {
		t.Errorf("Mint lightning backend is not default")
	}
	if config.POSTGRES_USER != "admin" {
		t.Errorf("postgres user is not default")
	}

	err = WriteConfigFile(originalCopyFile.TomlFile)

	if err != nil {
		t.Errorf("Could not rewrite config file to original %+v", err)
	}

}
