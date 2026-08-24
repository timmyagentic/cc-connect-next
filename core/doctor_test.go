package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckDataDirectoryUsesConfiguredPath(t *testing.T) {
	configured := filepath.Join(t.TempDir(), "custom-data")
	if err := os.MkdirAll(configured, 0o755); err != nil {
		t.Fatalf("create configured data dir: %v", err)
	}

	result := checkDataDirectory(configured)
	if result.Name != "Data Directory" || result.Status != DoctorPass {
		t.Fatalf("checkDataDirectory() = %+v, want passing data-directory check", result)
	}
	if result.Detail != configured {
		t.Fatalf("data directory detail = %q, want configured path %q", result.Detail, configured)
	}
}
