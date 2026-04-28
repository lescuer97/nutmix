package mint

import (
	"context"
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

	var pathToProjectDir = dir + "/" + ConfigDirName
	var pathToProjectConfigFile = pathToProjectDir + "/" + ConfigFileName
	err = utils.CreateDirectoryAndPath(pathToProjectDir, ConfigFileName)

	if err != nil {
		return []byte{}, fmt.Errorf("utils.CreateDirectoryAndPath(pathToProjectDir, ConfigFileName), %w", err)
	}

	// Manipulate Config file and parse
	return os.ReadFile(pathToProjectConfigFile)
}

type bootstrapNostrNotificationConfig struct {
	NOSTR_NOTIFICATIONS         bool
	NOSTR_NOTIFICATION_NIP04_DM bool
}

// will not look for os.variable config only file config
func SetUpConfigDB(ctx context.Context, db database.MintDB) (utils.Config, *utils.NostrNotificationConfig, error) {
	tx, err := db.GetTx(ctx)
	if err != nil {
		return utils.Config{}, nil, fmt.Errorf("db.GetTx(ctx): %w", err)
	}

	defer func() {
		_ = db.Rollback(ctx, tx)
	}()

	var config utils.Config
	var nostrNotificationConfig *utils.NostrNotificationConfig
	configMissingFromDB := false
	// check if config in db exists if it doesn't check for config file or set default
	config, err = db.GetConfig(tx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return config, nil, fmt.Errorf("db.GetConfig(tx): %w", err)
	}

	if errors.Is(err, sql.ErrNoRows) {
		configMissingFromDB = true
		var fileNostrConfig bootstrapNostrNotificationConfig
		// check if config file exists
		file, err := getConfigFile()
		if err != nil {
			return config, nil, fmt.Errorf("getConfigFile(), %w", err)
		}

		//nolint:musttag // Config intentionally reuses db-tagged struct for bootstrap config file values.
		err = toml.Unmarshal(file, &config)
		if err != nil {
			return config, nil, fmt.Errorf("toml.Unmarshal(buf,&config), %w", err)
		}

		//nolint:musttag // Bootstrap struct fields map directly to legacy uppercase config keys.
		err = toml.Unmarshal(file, &fileNostrConfig)
		if err != nil {
			return config, nil, fmt.Errorf("toml.Unmarshal(buf,&fileNostrConfig): %w", err)
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
		err = db.SetConfig(tx, config)
		if err != nil {
			return config, nil, fmt.Errorf("db.SetConfig(tx, config): %w", err)
		}

		if fileNostrConfig.NOSTR_NOTIFICATIONS || fileNostrConfig.NOSTR_NOTIFICATION_NIP04_DM {
			nostrNotificationConfig = &utils.NostrNotificationConfig{
				NOSTR_NOTIFICATIONS:         fileNostrConfig.NOSTR_NOTIFICATIONS,
				NOSTR_NOTIFICATION_NIP04_DM: fileNostrConfig.NOSTR_NOTIFICATION_NIP04_DM,
				NOSTR_NOTIFICATION_NSEC:     nil,
				NOSTR_NOTIFICATION_NPUBS:    nil,
			}
			err = db.UpdateNostrNotificationConfig(tx, *nostrNotificationConfig)
			if err != nil {
				return config, nil, fmt.Errorf("db.UpdateNostrNotificationConfig(tx, *nostrNotificationConfig): %w", err)
			}
		}
	}

	if nostrNotificationConfig == nil {
		nostrNotificationConfig, err = db.GetNostrNotificationConfig(tx)
		if err != nil {
			return config, nil, fmt.Errorf("db.GetNostrNotificationConfig(tx): %w", err)
		}
	}

	err = db.Commit(ctx, tx)
	if err != nil {
		return config, nil, fmt.Errorf("db.Commit(ctx, tx): %w", err)
	}

	if nostrNotificationConfig != nil {
		err = utils.SyncNostrNotificationNsec(nostrNotificationConfig, configMissingFromDB)
		if err != nil {
			return config, nil, fmt.Errorf("utils.SyncNostrNotificationNsec(nostrNotificationConfig, configMissingFromDB): %w", err)
		}
	}

	return config, nostrNotificationConfig, nil
}
