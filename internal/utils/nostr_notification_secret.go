package utils

import (
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/nbd-wtf/go-nostr"
)

const NostrNotificationNsecFileName = "nostr_notification_nsec"

func GetConfigDirectory() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("could not get config dir: %w", err)
	}

	return filepath.Join(configDir, ConfigDirName), nil
}

func SyncNostrNotificationNsec(config *Config, createIfMissing bool) error {
	if config == nil {
		return fmt.Errorf("config is nil")
	}

	nsec, err := ReadNostrNotificationNsec()
	if err == nil {
		config.NOSTR_NOTIFICATION_NSEC = nsec
		return nil
	}

	if errors.Is(err, os.ErrNotExist) {
		if !config.NOSTR_NOTIFICATIONS {
			config.NOSTR_NOTIFICATION_NSEC = nil
			return nil
		}

		if !createIfMissing {
			return fmt.Errorf("nostr notifications are enabled but the nsec file does not exist")
		}

		nsec, err = ReadOrCreateNostrNotificationNsec(config.NOSTR_NOTIFICATION_NSEC)
		if err != nil {
			return fmt.Errorf("ReadOrCreateNostrNotificationNsec(config.NOSTR_NOTIFICATION_NSEC): %w", err)
		}

		config.NOSTR_NOTIFICATION_NSEC = nsec
		return nil
	}

	return fmt.Errorf("ReadNostrNotificationNsec(): %w", err)
}

func ReadOrCreateNostrNotificationNsec(nsec []byte) ([]byte, error) {
	resolvedDirPath, err := GetConfigDirectory()
	if err != nil {
		return nil, fmt.Errorf("GetConfigDirectory(): %w", err)
	}

	return ReadOrCreateNostrNotificationNsecFromDir(resolvedDirPath, nsec)
}

func ReadOrCreateNostrNotificationNsecFromDir(dirPath string, nsec []byte) ([]byte, error) {
	slog.Debug("attempting to load nostr notification nsec", slog.String("dir_path", dirPath))

	loadedNsec, err := ReadNostrNotificationNsecFromDir(dirPath)
	if err == nil {
		slog.Debug("loaded existing nostr notification nsec")
		return loadedNsec, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("ReadNostrNotificationNsecFromDir(dirPath): %w", err)
	}

	if len(nsec) == 0 {
		privateKeyHex := nostr.GeneratePrivateKey()
		generatedNsec, decodeErr := hex.DecodeString(privateKeyHex)
		if decodeErr != nil {
			return nil, fmt.Errorf("hex.DecodeString(privateKeyHex): %w", decodeErr)
		}
		nsec = generatedNsec
	}

	if err := WriteNostrNotificationNsecToDir(dirPath, nsec); err != nil {
		return nil, fmt.Errorf("WriteNostrNotificationNsecToDir(dirPath, nsec): %w", err)
	}

	storedNsec := make([]byte, len(nsec))
	copy(storedNsec, nsec)
	return storedNsec, nil
}

func ReadNostrNotificationNsec() ([]byte, error) {
	resolvedDirPath, err := GetConfigDirectory()
	if err != nil {
		return nil, fmt.Errorf("GetConfigDirectory(): %w", err)
	}

	return ReadNostrNotificationNsecFromDir(resolvedDirPath)
}

func ReadNostrNotificationNsecFromDir(dirPath string) ([]byte, error) {

	nsecPath := nostrNotificationNsecFilePath(dirPath)
	fileInfo, err := os.Lstat(nsecPath)
	if err != nil {
		return nil, err
	}
	if fileInfo.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("nostr notification nsec file is a symlink")
	}

	nsec, err := os.ReadFile(nsecPath)
	if err != nil {
		return nil, err
	}

	if err := validateNostrNotificationNsec(nsec); err != nil {
		return nil, fmt.Errorf("validateNostrNotificationNsec(nsec): %w", err)
	}

	return nsec, nil
}

func WriteNostrNotificationNsec(nsec []byte) error {
	resolvedDirPath, err := GetConfigDirectory()
	if err != nil {
		return fmt.Errorf("GetConfigDirectory(): %w", err)
	}

	return WriteNostrNotificationNsecToDir(resolvedDirPath, nsec)
}

func WriteNostrNotificationNsecToDir(dirPath string, nsec []byte) error {
	if err := validateNostrNotificationNsec(nsec); err != nil {
		return fmt.Errorf("validateNostrNotificationNsec(nsec): %w", err)
	}

	if err := os.MkdirAll(dirPath, 0o750); err != nil {
		return fmt.Errorf("os.MkdirAll(dirPath, 0750): %w", err)
	}

	nsecPath := nostrNotificationNsecFilePath(dirPath)
	if fileInfo, statErr := os.Lstat(nsecPath); statErr == nil {
		if fileInfo.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("nostr notification nsec file is a symlink")
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("os.Lstat(nsecPath): %w", statErr)
	}

	if err := os.WriteFile(nsecPath, nsec, 0o600); err != nil {
		return fmt.Errorf("os.WriteFile(nsecPath, nsec, 0600): %w", err)
	}

	if err := os.Chmod(nsecPath, 0o600); err != nil {
		return fmt.Errorf("os.Chmod(nsecPath, 0600): %w", err)
	}

	return nil
}

func nostrNotificationNsecFilePath(dirPath string) string {
	return filepath.Join(dirPath, NostrNotificationNsecFileName)
}

func validateNostrNotificationNsec(nsec []byte) error {
	if len(nsec) == 0 {
		return fmt.Errorf("nostr notification nsec is empty")
	}

	if _, err := nostr.GetPublicKey(hex.EncodeToString(nsec)); err != nil {
		return fmt.Errorf("invalid nostr private key: %w", err)
	}

	return nil
}
