//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || windows

package appfeatures

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	featureupdater "github.com/timmyagentic/awesome-agent-app-features/updater"
)

func TestHostUpdateLockExcludesOtherProcesses(t *testing.T) {
	if lockPath := os.Getenv("CC_NEXT_TEST_UPDATE_LOCK_PATH"); lockPath != "" {
		lock, err := tryHostUpdateLock(lockPath)
		if lock != nil {
			_ = lock.release()
		}
		if !errors.Is(err, featureupdater.ErrUpdateInProgress) {
			t.Fatalf("second process entered an active update: %v", err)
		}
		return
	}
	lockPath := filepath.Join(t.TempDir(), "target.update.lock")
	lock, err := tryHostUpdateLock(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(os.Args[0], "-test.run=^TestHostUpdateLockExcludesOtherProcesses$")
	command.Env = append(os.Environ(), "CC_NEXT_TEST_UPDATE_LOCK_PATH="+lockPath)
	output, childErr := command.CombinedOutput()
	if err := lock.release(); err != nil {
		t.Fatal(err)
	}
	if childErr != nil {
		t.Fatalf("cross-process exclusion failed: %v\n%s", childErr, output)
	}
	reacquired, err := tryHostUpdateLock(lockPath)
	if err != nil {
		t.Fatalf("released lock could not be acquired: %v", err)
	}
	if err := reacquired.release(); err != nil {
		t.Fatal(err)
	}
}
