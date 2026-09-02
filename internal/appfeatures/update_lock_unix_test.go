//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package appfeatures

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHostUpdateLockRejectsSymlinkWithoutTouchingTarget(t *testing.T) {
	victim := filepath.Join(t.TempDir(), "victim")
	if err := os.WriteFile(victim, []byte("important"), 0o600); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(filepath.Dir(victim), "update.lock")
	if err := os.Symlink(victim, lockPath); err != nil {
		t.Fatal(err)
	}
	if _, err := tryHostUpdateLock(lockPath); err == nil {
		t.Fatal("symlink update lock was accepted")
	}
	content, err := os.ReadFile(victim)
	if err != nil || string(content) != "important" {
		t.Fatalf("victim changed: %q, %v", content, err)
	}
}
