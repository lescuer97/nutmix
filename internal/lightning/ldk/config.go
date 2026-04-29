package ldk

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/utils"
)

type RPCConfig struct {
	Address  string
	Username string
	Password string
	Port     uint16
}

type ChainSourceType string

const (
	ChainSourceBitcoind ChainSourceType = "bitcoind"
	ChainSourceElectrum ChainSourceType = "electrum"
	ChainSourceEsplora  ChainSourceType = "esplora"
)

type PersistedConfig struct {
	ConfigDirectory   string
	ChainSourceType   ChainSourceType
	ElectrumServerURL string
	EsploraServerURL  string
	Rpc               RPCConfig
}

func DefaultConfigDirectory() (string, error) {
	configDir, err := utils.GetConfigDirectory()
	if err != nil {
		return "", fmt.Errorf("utils.GetConfigDirectory(): %w", err)
	}

	defaultConfigDirectory := filepath.Join(configDir, "ldk")
	normalizedConfigDirectory, err := normalizeConfigDirectory(defaultConfigDirectory)
	if err != nil {
		return "", fmt.Errorf("normalizeConfigDirectory(defaultConfigDirectory): %w", err)
	}

	return normalizedConfigDirectory, nil
}

func NewPersistedConfig(rpc RPCConfig, configDirectory string) (PersistedConfig, error) {
	return NewPersistedConfigWithChainSource(ChainSourceBitcoind, rpc, "", "", configDirectory)
}

func NewPersistedConfigWithChainSource(chainSourceType ChainSourceType, rpc RPCConfig, electrumServerURL string, esploraServerURL string, configDirectory string) (PersistedConfig, error) {
	return normalizePersistedConfig(PersistedConfig{
		ConfigDirectory:   configDirectory,
		ChainSourceType:   chainSourceType,
		ElectrumServerURL: electrumServerURL,
		EsploraServerURL:  esploraServerURL,
		Rpc:               rpc,
	})
}

func normalizePersistedConfig(config PersistedConfig) (PersistedConfig, error) {
	chainSourceType, err := normalizeChainSourceType(config.ChainSourceType)
	if err != nil {
		return PersistedConfig{}, fmt.Errorf("normalizeChainSourceType(config.ChainSourceType): %w", err)
	}
	config.ChainSourceType = chainSourceType
	config.ElectrumServerURL = strings.TrimSpace(config.ElectrumServerURL)
	config.EsploraServerURL = strings.TrimSpace(config.EsploraServerURL)
	config.Rpc.Address = strings.TrimSpace(config.Rpc.Address)
	config.Rpc.Username = strings.TrimSpace(config.Rpc.Username)
	config.Rpc.Password = strings.TrimSpace(config.Rpc.Password)

	normalizedConfigDirectory, err := normalizeConfigDirectory(config.ConfigDirectory)
	if err != nil {
		return PersistedConfig{}, fmt.Errorf("normalizeConfigDirectory(config.ConfigDirectory): %w", err)
	}
	config.ConfigDirectory = normalizedConfigDirectory

	if err := validatePersistedConfig(config); err != nil {
		return PersistedConfig{}, fmt.Errorf("validatePersistedConfig(config): %w", err)
	}

	return config, nil
}

func normalizeChainSourceType(chainSourceType ChainSourceType) (ChainSourceType, error) {
	normalizedChainSourceType := ChainSourceType(strings.ToLower(strings.TrimSpace(string(chainSourceType))))
	if normalizedChainSourceType == "" {
		return ChainSourceBitcoind, nil
	}

	switch normalizedChainSourceType {
	case ChainSourceBitcoind, ChainSourceElectrum, ChainSourceEsplora:
		return normalizedChainSourceType, nil
	default:
		return "", fmt.Errorf("unknown chain source type %q", chainSourceType)
	}
}

func validateServerURL(serverType string, serverURL string) error {
	trimmedServerURL := strings.TrimSpace(serverURL)
	if trimmedServerURL == "" {
		return fmt.Errorf("%s server url is required", serverType)
	}

	parsedURL, err := url.Parse(trimmedServerURL)
	if err != nil {
		return fmt.Errorf("%s server url is invalid: %w", serverType, err)
	}
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return fmt.Errorf("%s server url must include a scheme and host", serverType)
	}

	return nil
}

func ValidateElectrumServerURL(electrumServerURL string) error {
	return validateServerURL("electrum", electrumServerURL)
}

func ValidateEsploraServerURL(esploraServerURL string) error {
	return validateServerURL("esplora", esploraServerURL)
}

func normalizeConfigDirectory(configDirectory string) (string, error) {
	trimmedConfigDirectory := strings.TrimSpace(configDirectory)
	if trimmedConfigDirectory == "" {
		return "", fmt.Errorf("config directory is required")
	}

	normalizedConfigDirectory := filepath.Clean(trimmedConfigDirectory)
	if !filepath.IsAbs(normalizedConfigDirectory) {
		return "", fmt.Errorf("config directory must be an absolute path")
	}

	fileInfo, err := os.Lstat(normalizedConfigDirectory)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return normalizedConfigDirectory, nil
		}
		return "", fmt.Errorf("os.Lstat(configDirectory): %w", err)
	}

	if fileInfo.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("config directory is a symlink")
	}
	if !fileInfo.IsDir() {
		return "", fmt.Errorf("config directory is not a directory")
	}

	return normalizedConfigDirectory, nil
}

func GetPersistedConfig(ctx context.Context, db database.MintDB) (PersistedConfig, error) {
	if db == nil {
		return PersistedConfig{}, fmt.Errorf("ldk database is nil")
	}

	slog.Debug("loading persisted ldk config from database")
	config, err := db.GetLDKConfig(ctx)
	if err != nil {
		return PersistedConfig{}, fmt.Errorf("db.GetLDKConfig(ctx): %w", err)
	}
	slog.Info("loaded persisted ldk config from database")

	persistedConfig, err := normalizePersistedConfig(PersistedConfig{
		ConfigDirectory:   config.ConfigDirectory,
		ChainSourceType:   ChainSourceType(config.ChainSourceType),
		ElectrumServerURL: config.ElectrumServerURL,
		EsploraServerURL:  config.EsploraServerURL,
		Rpc:               RPCConfig(config.Rpc),
	})
	if err != nil {
		return PersistedConfig{}, fmt.Errorf("normalizePersistedConfig(...): %w", err)
	}

	return persistedConfig, nil
}

func SaveConfig(ctx context.Context, db database.MintDB, config PersistedConfig) error {
	if db == nil {
		return fmt.Errorf("ldk database is nil")
	}

	normalizedConfig, err := normalizePersistedConfig(config)
	if err != nil {
		return fmt.Errorf("normalizePersistedConfig(config): %w", err)
	}

	slog.Debug("saving persisted ldk config to database")
	if err := db.SetLDKConfig(ctx, database.LDKConfig{
		ConfigDirectory:   normalizedConfig.ConfigDirectory,
		ChainSourceType:   database.LDKChainSourceType(normalizedConfig.ChainSourceType),
		ElectrumServerURL: normalizedConfig.ElectrumServerURL,
		EsploraServerURL:  normalizedConfig.EsploraServerURL,
		Rpc:               database.LDKRPCConfig(normalizedConfig.Rpc),
	}); err != nil {
		return fmt.Errorf("db.SetLDKConfig(ctx, config): %w", err)
	}
	slog.Info("saved persisted ldk config to database")

	return nil
}

func (l *LDK) PersistedConfig(ctx context.Context) (PersistedConfig, error) {
	if l == nil {
		return PersistedConfig{}, fmt.Errorf("ldk backend is nil")
	}

	return GetPersistedConfig(ctx, l.db)
}

func (l *LDK) SaveConfig(ctx context.Context, config PersistedConfig) error {
	if l == nil {
		return fmt.Errorf("ldk backend is nil")
	}

	return SaveConfig(ctx, l.db, config)
}
