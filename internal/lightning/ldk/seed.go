package ldk

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	ldk_node "github.com/lescuer97/ldkgo/bindings/ldk_node_ffi"
	"github.com/lescuer97/nutmix/internal/utils"
)

const seedFileName = "ldk_seed"

func generateSeedMnemonic() string {
	wordCount := ldk_node.WordCountWords24
	return ldk_node.GenerateEntropyMnemonic(&wordCount)
}

func ReadOrCreateSeed(dirPath string) (string, error) {
	slog.Debug("attempting to load seed mnemonic", slog.String("dir_path", dirPath))
	seed, err := readSeed(dirPath)
	if err == nil {
		slog.Info("loaded existing seed mnemonic")
		return seed, nil
	}
	if !os.IsNotExist(err) {
		return "", fmt.Errorf("readSeed(dirPath): %w", err)
	}
	slog.Debug("seed mnemonic not found, generating new seed")

	seed = generateSeedMnemonic()
	if strings.TrimSpace(seed) == "" {
		return "", fmt.Errorf("generated seed mnemonic is empty")
	}

	slog.Debug("writing generated seed mnemonic to disk")
	err = writeSeed(dirPath, seed)
	if err != nil {
		return "", fmt.Errorf("writeSeed(dirPath, seed): %w", err)
	}
	slog.Info("created new seed mnemonic")

	return seed, nil
}

func readSeed(dirPath string) (string, error) {
	resolvedDirPath, err := resolveSeedDirPath(dirPath)
	if err != nil {
		return "", fmt.Errorf("resolveSeedDirPath(dirPath): %w", err)
	}

	seedPath := seedFilePath(resolvedDirPath)
	slog.Debug("checking seed file", slog.String("seed_path", seedPath))

	fileInfo, err := os.Lstat(seedPath)
	if err != nil {
		return "", err
	}
	if fileInfo.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("seed file is a symlink")
	}

	seedFile, err := os.ReadFile(seedPath)
	if err != nil {
		return "", err
	}
	slog.Debug("read seed file", slog.String("seed_path", seedPath))

	seed := strings.TrimSpace(string(seedFile))
	if err := validateSeedMnemonic(seed); err != nil {
		return "", fmt.Errorf("validateSeedMnemonic(seed): %w", err)
	}

	return seed, nil
}

func writeSeed(dirPath string, mnemonic string) error {
	resolvedDirPath, err := resolveSeedDirPath(dirPath)
	if err != nil {
		return fmt.Errorf("resolveSeedDirPath(dirPath): %w", err)
	}

	err = os.MkdirAll(resolvedDirPath, 0o750)
	if err != nil {
		return fmt.Errorf("os.MkdirAll(dirPath, 0750): %w", err)
	}

	seedPath := seedFilePath(resolvedDirPath)
	slog.Debug("preparing to write seed file", slog.String("seed_path", seedPath))
	if fileInfo, statErr := os.Lstat(seedPath); statErr == nil {
		if fileInfo.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("seed file is a symlink")
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("os.Lstat(seedPath): %w", statErr)
	}

	err = os.WriteFile(seedPath, []byte(mnemonic), 0o600)
	if err != nil {
		return fmt.Errorf("os.WriteFile(seedPath, mnemonic, 0600): %w", err)
	}
	slog.Debug("seed file written", slog.String("seed_path", seedPath))
	err = os.Chmod(seedPath, 0o600)
	if err != nil {
		return fmt.Errorf("os.Chmod(seedPath, 0600): %w", err)
	}
	slog.Debug("seed file permissions updated", slog.String("seed_path", seedPath))

	return nil
}

func resolveSeedDirPath(dirPath string) (string, error) {
	if strings.TrimSpace(dirPath) != "" {
		return dirPath, nil
	}

	configDirPath, err := utils.GetConfigDirectory()
	if err != nil {
		return "", fmt.Errorf("utils.GetConfigDirectory(): %w", err)
	}

	return configDirPath, nil
}

func seedFilePath(dirPath string) string {
	return filepath.Join(dirPath, seedFileName)
}

func validateSeedMnemonic(seed string) error {
	if seed == "" {
		return fmt.Errorf("seed file is empty")
	}

	wordCount := len(strings.Fields(seed))
	if wordCount != 24 {
		return fmt.Errorf("seed must contain 24 words, got %d", wordCount)
	}

	return nil
}
