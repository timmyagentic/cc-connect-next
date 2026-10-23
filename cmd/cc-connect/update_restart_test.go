package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/timmyagentic/cc-connect-next/core"
)

func TestUpdateRestartUsesVerifiedInstallationTarget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new-binary")
	if err := os.WriteFile(path, []byte("fixture"), 0700); err != nil {
		t.Fatal(err)
	}
	got, err := executableForRestart(core.RestartRequest{UpdateID: "operation", ExecutablePath: path})
	if err != nil || got != path {
		t.Fatalf("restart selected running test inode instead of updated target: %q %v", got, err)
	}
	if _, err = executableForRestart(core.RestartRequest{UpdateID: "operation", ExecutablePath: "relative-binary"}); err == nil {
		t.Fatal("accepted relative restart target")
	}
	if _, err = executableForRestart(core.RestartRequest{UpdateID: "operation", ExecutablePath: t.TempDir()}); err == nil {
		t.Fatal("accepted directory as executable")
	}
}
