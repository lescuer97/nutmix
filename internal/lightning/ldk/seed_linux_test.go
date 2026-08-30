//go:build linux

package ldk

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestReadOrCreateSeedConcurrentFirstStartUsesSameIdentity(t *testing.T) {
	dirPath := t.TempDir()
	const callers = 8

	seeds := make([]string, callers)
	errs := make([]error, callers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			seeds[i], errs[i] = ReadOrCreateSeed(dirPath)
		}()
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("ReadOrCreateSeed call %d: %v", i, err)
		}
		if seeds[i] != seeds[0] {
			t.Fatalf("seed %d differs from first caller", i)
		}
	}
}

func TestReadOrCreateSeedRejectsSeedSymlink(t *testing.T) {
	dirPath := t.TempDir()
	targetPath := filepath.Join(t.TempDir(), "target")
	targetContents := "do not replace"
	if err := os.WriteFile(targetPath, []byte(targetContents), 0o600); err != nil {
		t.Fatalf("os.WriteFile(targetPath): %v", err)
	}
	if err := os.Symlink(targetPath, filepath.Join(dirPath, seedFileName)); err != nil {
		t.Fatalf("os.Symlink(targetPath, seedPath): %v", err)
	}

	if _, err := ReadOrCreateSeed(dirPath); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("ReadOrCreateSeed symlink error = %v, want symlink rejection", err)
	}
	contents, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("os.ReadFile(targetPath): %v", err)
	}
	if string(contents) != targetContents {
		t.Fatalf("symlink target changed: got %q, want %q", contents, targetContents)
	}
}

func TestReadOrCreateSeedRejectsSymlinkedDirectory(t *testing.T) {
	targetDir := t.TempDir()
	dirPath := filepath.Join(t.TempDir(), "seed-dir")
	if err := os.Symlink(targetDir, dirPath); err != nil {
		t.Fatalf("os.Symlink(targetDir, dirPath): %v", err)
	}

	if _, err := ReadOrCreateSeed(dirPath); err == nil {
		t.Fatal("ReadOrCreateSeed symlinked directory succeeded")
	}
	if _, err := os.Lstat(filepath.Join(targetDir, seedFileName)); !os.IsNotExist(err) {
		t.Fatalf("symlink target seed exists or stat failed: %v", err)
	}
}

func TestReadOrCreateSeedRejectsIntermediateDirectorySymlink(t *testing.T) {
	root := t.TempDir()
	targetDir := t.TempDir()
	if err := os.Symlink(targetDir, filepath.Join(root, "linked")); err != nil {
		t.Fatalf("os.Symlink(targetDir, linked): %v", err)
	}
	dirPath := filepath.Join(root, "linked", "ldk")

	if _, err := ReadOrCreateSeed(dirPath); err == nil {
		t.Fatal("ReadOrCreateSeed intermediate symlink succeeded")
	}
	if _, err := os.Lstat(filepath.Join(targetDir, "ldk", seedFileName)); !os.IsNotExist(err) {
		t.Fatalf("intermediate symlink target seed exists or stat failed: %v", err)
	}
}

func TestReadOrCreateSeedRejectsNonRegularFile(t *testing.T) {
	dirPath := t.TempDir()
	seedPath := filepath.Join(dirPath, seedFileName)
	if err := os.Mkdir(seedPath, 0o700); err != nil {
		t.Fatalf("os.Mkdir(seedPath): %v", err)
	}

	if _, err := ReadOrCreateSeed(dirPath); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("ReadOrCreateSeed non-regular error = %v, want regular-file rejection", err)
	}
	info, err := os.Stat(seedPath)
	if err != nil {
		t.Fatalf("os.Stat(seedPath): %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("non-regular seed was replaced: mode %v", info.Mode())
	}
}

func TestReadOrCreateSeedPreservesMalformedExistingFile(t *testing.T) {
	dirPath := t.TempDir()
	seedPath := filepath.Join(dirPath, seedFileName)
	malformed := []byte("not a seed")
	if err := os.WriteFile(seedPath, malformed, 0o644); err != nil {
		t.Fatalf("os.WriteFile(seedPath): %v", err)
	}

	if _, err := ReadOrCreateSeed(dirPath); err == nil {
		t.Fatal("ReadOrCreateSeed malformed seed succeeded")
	}
	contents, err := os.ReadFile(seedPath)
	if err != nil {
		t.Fatalf("os.ReadFile(seedPath): %v", err)
	}
	if string(contents) != string(malformed) {
		t.Fatalf("malformed seed changed: got %q, want %q", contents, malformed)
	}
	info, err := os.Stat(seedPath)
	if err != nil {
		t.Fatalf("os.Stat(seedPath): %v", err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("malformed seed mode changed: got %o, want 644", info.Mode().Perm())
	}
}

func TestReadOrCreateSeedRejectsOversizedExistingFile(t *testing.T) {
	dirPath := t.TempDir()
	seedPath := filepath.Join(dirPath, seedFileName)
	contents := []byte(strings.Repeat("x", maxSeedFileSize+1))
	if err := os.WriteFile(seedPath, contents, 0o600); err != nil {
		t.Fatalf("os.WriteFile(seedPath): %v", err)
	}

	if _, err := ReadOrCreateSeed(dirPath); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("ReadOrCreateSeed oversized error = %v, want bounded-read rejection", err)
	}
	info, err := os.Stat(seedPath)
	if err != nil {
		t.Fatalf("os.Stat(seedPath): %v", err)
	}
	if info.Size() != int64(len(contents)) {
		t.Fatalf("oversized seed changed size: got %d, want %d", info.Size(), len(contents))
	}
}

func TestReadOrCreateSeedEnforcesExistingFileMode(t *testing.T) {
	dirPath := t.TempDir()
	seedPath := filepath.Join(dirPath, seedFileName)
	seed := "alpha beta gamma delta epsilon zeta eta theta iota kappa lambda mu nu xi omicron pi rho sigma tau upsilon phi chi psi omega"
	if err := os.WriteFile(seedPath, []byte(seed), 0o644); err != nil {
		t.Fatalf("os.WriteFile(seedPath): %v", err)
	}

	if _, err := ReadOrCreateSeed(dirPath); err != nil {
		t.Fatalf("ReadOrCreateSeed(dirPath): %v", err)
	}
	info, err := os.Stat(seedPath)
	if err != nil {
		t.Fatalf("os.Stat(seedPath): %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("seed mode = %o, want 600", info.Mode().Perm())
	}
}
