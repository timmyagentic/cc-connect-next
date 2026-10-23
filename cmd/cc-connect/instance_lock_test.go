//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFirstRun_LockThenBootstrapKeepsConfigDirectoryPrivate(t *testing.T) {
	configDir := filepath.Join(t.TempDir(), ".cc-connect-next")
	configPath := filepath.Join(configDir, "config.toml")

	// main acquires its lock before generating the first-run configuration.
	lock, err := AcquireInstanceLock(configPath)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()
	if err := bootstrapConfig(configPath); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]os.FileMode{configDir: 0o700, configPath: 0o600} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Errorf("%s permissions = %#o, want %#o", filepath.Base(path), got, want)
		}
	}
}

func TestAcquireInstanceLock_PreservesExistingDirectoryPermissions(t *testing.T) {
	configDir := t.TempDir()
	if err := os.Chmod(configDir, 0o750); err != nil {
		t.Fatal(err)
	}
	lock, err := AcquireInstanceLock(filepath.Join(configDir, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()
	info, err := os.Stat(configDir)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o750 {
		t.Fatalf("existing directory permissions = %#o, want 0750", got)
	}
}

func TestAcquireInstanceLock_Success(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.toml")

	lock, err := AcquireInstanceLock(cfg)
	if err != nil {
		t.Fatalf("AcquireInstanceLock: %v", err)
	}
	if lock == nil || !lock.acquired {
		t.Fatal("expected acquired lock")
	}
	defer lock.Release()

	if lock.Path() == "" {
		t.Fatal("expected non-empty lock path")
	}
}

func TestAcquireInstanceLock_AlreadyLocked(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.toml")

	first, err := AcquireInstanceLock(cfg)
	if err != nil {
		t.Fatalf("first AcquireInstanceLock: %v", err)
	}
	defer first.Release()

	_, err = AcquireInstanceLock(cfg)
	if err == nil {
		t.Fatal("second AcquireInstanceLock should fail while lock held")
	}
}
