package ldk

import (
	"fmt"
	"log/slog"
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
	resolvedDirPath, err := resolveSeedDirPath(dirPath)
	if err != nil {
		return "", fmt.Errorf("resolveSeedDirPath(dirPath): %w", err)
	}
	seed, created, err := readOrCreateSeedFile(resolvedDirPath)
	if err != nil {
		return "", fmt.Errorf("readOrCreateSeedFile(dirPath): %w", err)
	}
	if !created {
		slog.Info("loaded existing seed mnemonic")
		return seed, nil
	}
	slog.Info("created new seed mnemonic")

	return seed, nil
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
