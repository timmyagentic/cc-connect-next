//go:build unix

package appfeatures

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUpdateCommandTimeoutStopsInstallerChildren(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "orphan-write")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err := runUpdateCommand(ctx, "sh", "-c", `(sleep 0.3; touch "$1") & wait`, "fixture", marker)
	if err == nil {
		t.Fatal("installer did not time out")
	}
	time.Sleep(350 * time.Millisecond)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("installer child survived timeout and changed files")
	}
}
