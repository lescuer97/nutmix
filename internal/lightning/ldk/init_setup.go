package ldk

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	ldk_node "github.com/lescuer97/ldkgo/bindings/ldk_node_ffi"
	"github.com/lescuer97/nutmix/internal/utils"
)

func (l *LDK) prepareInitConfig(ctx context.Context) (string, string, ldk_node.Network, PersistedConfig, error) {
	if l == nil {
		return "", "", 0, PersistedConfig{}, fmt.Errorf("ldk backend is nil")
	}
	if l.db == nil {
		return "", "", 0, PersistedConfig{}, fmt.Errorf("ldk database is nil")
	}
	if l.network == "" {
		return "", "", 0, PersistedConfig{}, fmt.Errorf("ldk network is empty")
	}

	config, err := GetPersistedConfig(ctx, l.db)
	if err != nil {
		return "", "", 0, PersistedConfig{}, fmt.Errorf("GetPersistedConfig(ctx, l.db): %w", err)
	}

	storageDirPath, err := l.resolveStorageDir(config)
	if err != nil {
		return "", "", 0, PersistedConfig{}, fmt.Errorf("l.resolveStorageDir(config): %w", err)
	}

	seedMnemonic, err := ReadOrCreateSeed(storageDirPath)
	if err != nil {
		return "", "", 0, PersistedConfig{}, fmt.Errorf("ReadOrCreateSeed(storageDirPath): %w", err)
	}

	chainParams, err := utils.CheckChainParams(l.network)
	if err != nil {
		return "", "", 0, PersistedConfig{}, fmt.Errorf("utils.CheckChainParams(l.network): %w", err)
	}

	network, err := convertChaninParamsToLdkNetwork(chainParams)
	if err != nil {
		return "", "", 0, PersistedConfig{}, fmt.Errorf("convertChaninParamsToLdkNetwork(chainParams): %w", err)
	}

	return seedMnemonic, storageDirPath, network, config, nil
}

func (l *LDK) resolveStorageDir(config PersistedConfig) (string, error) {
	if strings.TrimSpace(l.storageDir()) != "" {
		normalizedStorageDir, err := normalizeConfigDirectory(l.storageDir())
		if err != nil {
			return "", fmt.Errorf("normalizeConfigDirectory(l.storageDir()): %w", err)
		}
		return normalizedStorageDir, nil
	}

	return normalizeConfigDirectory(config.ConfigDirectory)
}

func validatePersistedConfig(config PersistedConfig) error {
	var chainSourceHost string
	switch config.ChainSourceType {
	case ChainSourceElectrum:
		if err := ValidateElectrumServerURL(config.ElectrumServerURL); err != nil {
			return err
		}
		if parsedURL, err := url.Parse(config.ElectrumServerURL); err == nil {
			chainSourceHost = parsedURL.Hostname()
		}
	case ChainSourceEsplora:
		if err := ValidateEsploraServerURL(config.EsploraServerURL); err != nil {
			return err
		}
		if parsedURL, err := url.Parse(config.EsploraServerURL); err == nil {
			chainSourceHost = parsedURL.Hostname()
		}
	case ChainSourceBitcoind:
		if strings.TrimSpace(config.Rpc.Address) == "" {
			return fmt.Errorf("rpc address is empty")
		}
		if config.Rpc.Port == 0 {
			return fmt.Errorf("rpc port is empty")
		}
		if strings.TrimSpace(config.Rpc.Username) == "" {
			return fmt.Errorf("rpc username is empty")
		}
		if strings.TrimSpace(config.Rpc.Password) == "" {
			return fmt.Errorf("rpc password is empty")
		}
		chainSourceHost = config.Rpc.Address
	default:
		return fmt.Errorf("chain source type is invalid")
	}
	if config.TorOnly {
		if err := validateTorOnlyHost(chainSourceHost); err != nil {
			return fmt.Errorf("chain source address %q is not reachable over tor: %w", chainSourceHost, err)
		}
	}
	if _, err := normalizeConfigDirectory(config.ConfigDirectory); err != nil {
		return fmt.Errorf("config directory is invalid: %w", err)
	}
	if config.TorProxyAddress != nil {
		if err := validateTorProxySocketAddress(*config.TorProxyAddress); err != nil {
			return err
		}
	}

	return nil
}

func validateTorOnlyListeningAddresses(addresses []string) error {
	for _, address := range addresses {
		if err := validateTorOnlySocketAddress(address); err != nil {
			return fmt.Errorf("listening address %q is not a valid tor-only address: %w", address, err)
		}
	}
	return nil
}

func validateTorOnlySocketAddress(address string) error {
	host, port, err := net.SplitHostPort(strings.TrimSpace(address))
	if err != nil {
		return fmt.Errorf("address must be in the format host:port")
	}
	portNumber, err := strconv.ParseUint(port, 10, 16)
	if err != nil || portNumber == 0 {
		return fmt.Errorf("address port is invalid")
	}

	return validateTorOnlyHost(host)
}
