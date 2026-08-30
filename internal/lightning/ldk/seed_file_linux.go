//go:build linux

package ldk

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const maxSeedFileSize = 4096

func readOrCreateSeedFile(dirPath string) (seed string, created bool, err error) {
	dirFD, err := openSeedDirectory(dirPath)
	if err != nil {
		return "", false, fmt.Errorf("open seed directory: %w", err)
	}
	defer syscall.Close(dirFD)

	if err := syscall.Flock(dirFD, syscall.LOCK_EX); err != nil {
		return "", false, fmt.Errorf("lock seed directory: %w", err)
	}
	defer syscall.Flock(dirFD, syscall.LOCK_UN)

	seedFD, err := syscall.Openat(dirFD, seedFileName, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err == nil {
		seed, err := readExistingSeedFile(dirFD, seedFD)
		return seed, false, err
	}
	if !errors.Is(err, syscall.ENOENT) {
		if errors.Is(err, syscall.ELOOP) {
			return "", false, fmt.Errorf("seed file is a symlink")
		}
		return "", false, fmt.Errorf("open seed file: %w", err)
	}

	seed = generateSeedMnemonic()
	if strings.TrimSpace(seed) == "" {
		return "", false, fmt.Errorf("generated seed mnemonic is empty")
	}
	if err := validateSeedMnemonic(seed); err != nil {
		return "", false, fmt.Errorf("validate generated seed mnemonic: %w", err)
	}

	seedFD, err = syscall.Openat(dirFD, seedFileName, syscall.O_WRONLY|syscall.O_CREAT|syscall.O_EXCL|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0o600)
	if err != nil {
		return "", false, fmt.Errorf("create seed file: %w", err)
	}
	created = true
	seedFile := os.NewFile(uintptr(seedFD), seedFileName)
	cleanup := true
	defer func() {
		if seedFile != nil {
			_ = seedFile.Close()
		}
		if cleanup {
			_ = syscall.Unlinkat(dirFD, seedFileName)
			_ = syscall.Fsync(dirFD)
		}
	}()

	if err := seedFile.Chmod(0o600); err != nil {
		return "", false, fmt.Errorf("chmod seed file: %w", err)
	}
	if _, err := io.WriteString(seedFile, seed); err != nil {
		return "", false, fmt.Errorf("write seed file: %w", err)
	}
	if err := seedFile.Sync(); err != nil {
		return "", false, fmt.Errorf("sync seed file: %w", err)
	}
	if err := seedFile.Close(); err != nil {
		seedFile = nil
		return "", false, fmt.Errorf("close seed file: %w", err)
	}
	seedFile = nil
	if err := syscall.Fsync(dirFD); err != nil {
		return "", false, fmt.Errorf("sync seed directory: %w", err)
	}

	cleanup = false
	return seed, true, nil
}

func openSeedDirectory(dirPath string) (int, error) {
	cleanPath := filepath.Clean(dirPath)
	if !filepath.IsAbs(cleanPath) {
		return -1, fmt.Errorf("seed directory must be an absolute path")
	}

	currentFD, err := syscall.Open("/", syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return -1, err
	}
	for _, component := range strings.Split(strings.TrimPrefix(cleanPath, "/"), "/") {
		if component == "" {
			continue
		}
		nextFD, openErr := syscall.Openat(currentFD, component, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
		if errors.Is(openErr, syscall.ENOENT) {
			if mkdirErr := syscall.Mkdirat(currentFD, component, 0o750); mkdirErr != nil && !errors.Is(mkdirErr, syscall.EEXIST) {
				_ = syscall.Close(currentFD)
				return -1, fmt.Errorf("create directory %q: %w", component, mkdirErr)
			}
			nextFD, openErr = syscall.Openat(currentFD, component, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
		}
		if openErr != nil {
			_ = syscall.Close(currentFD)
			return -1, fmt.Errorf("open directory %q: %w", component, openErr)
		}
		_ = syscall.Close(currentFD)
		currentFD = nextFD
	}
	return currentFD, nil
}

func readExistingSeedFile(dirFD, seedFD int) (string, error) {
	seedFile := os.NewFile(uintptr(seedFD), seedFileName)
	defer seedFile.Close()

	var stat syscall.Stat_t
	if err := syscall.Fstat(seedFD, &stat); err != nil {
		return "", fmt.Errorf("stat seed file: %w", err)
	}
	if stat.Mode&syscall.S_IFMT != syscall.S_IFREG {
		return "", fmt.Errorf("seed file is not a regular file")
	}

	contents, err := io.ReadAll(io.LimitReader(seedFile, maxSeedFileSize+1))
	if err != nil {
		return "", fmt.Errorf("read seed file: %w", err)
	}
	if len(contents) > maxSeedFileSize {
		return "", fmt.Errorf("seed file exceeds %d bytes", maxSeedFileSize)
	}

	seed := strings.TrimSpace(string(contents))
	if err := validateSeedMnemonic(seed); err != nil {
		return "", fmt.Errorf("validate seed mnemonic: %w", err)
	}

	if stat.Mode&0o7777 != 0o600 {
		if err := seedFile.Chmod(0o600); err != nil {
			return "", fmt.Errorf("chmod seed file: %w", err)
		}
		if err := seedFile.Sync(); err != nil {
			return "", fmt.Errorf("sync seed file: %w", err)
		}
		if err := syscall.Fsync(dirFD); err != nil {
			return "", fmt.Errorf("sync seed directory: %w", err)
		}
	}

	return seed, nil
}
