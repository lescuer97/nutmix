package utils

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/nbd-wtf/go-nostr"
)

func TestReadOrCreateNostrNotificationNsecCreatesFile(t *testing.T) {
	dirPath := t.TempDir()

	nsec, err := ReadOrCreateNostrNotificationNsecFromDir(dirPath, nil)
	if err != nil {
		t.Fatalf("ReadOrCreateNostrNotificationNsecFromDir(dirPath, nil): %v", err)
	}

	if len(nsec) == 0 {
		t.Fatal("expected generated nostr notification nsec")
	}

	filePath := filepath.Join(dirPath, NostrNotificationNsecFileName)
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("os.Stat(filePath): %v", err)
	}

	if fileInfo.Mode().Perm() != 0o600 {
		t.Fatalf("expected file mode 0600, got %#o", fileInfo.Mode().Perm())
	}

	loadedNsec, err := ReadNostrNotificationNsecFromDir(dirPath)
	if err != nil {
		t.Fatalf("ReadNostrNotificationNsecFromDir(dirPath): %v", err)
	}

	if !bytes.Equal(loadedNsec, nsec) {
		t.Fatal("expected loaded nostr notification nsec to match stored value")
	}
}

func TestReadOrCreateNostrNotificationNsecReusesExistingFile(t *testing.T) {
	dirPath := t.TempDir()
	privateKeyHex := nostr.GeneratePrivateKey()
	privateKeyBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		t.Fatalf("hex.DecodeString(privateKeyHex): %v", err)
	}

	if err := WriteNostrNotificationNsecToDir(dirPath, privateKeyBytes); err != nil {
		t.Fatalf("WriteNostrNotificationNsecToDir(dirPath, privateKeyBytes): %v", err)
	}

	loadedNsec, err := ReadOrCreateNostrNotificationNsecFromDir(dirPath, nil)
	if err != nil {
		t.Fatalf("ReadOrCreateNostrNotificationNsecFromDir(dirPath, nil): %v", err)
	}

	if !bytes.Equal(loadedNsec, privateKeyBytes) {
		t.Fatal("expected existing nostr notification nsec to be preserved")
	}
}

func TestReadNostrNotificationNsecRejectsSymlink(t *testing.T) {
	dirPath := t.TempDir()
	targetPath := filepath.Join(dirPath, "target")
	if err := os.WriteFile(targetPath, []byte("secret"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(targetPath, ...): %v", err)
	}

	filePath := filepath.Join(dirPath, NostrNotificationNsecFileName)
	if err := os.Symlink(targetPath, filePath); err != nil {
		t.Fatalf("os.Symlink(targetPath, filePath): %v", err)
	}

	if _, err := ReadNostrNotificationNsecFromDir(dirPath); err == nil {
		t.Fatal("expected symlinked nostr notification nsec file to be rejected")
	}
}
