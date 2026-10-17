package core

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

type doctorPathAgent struct {
	stubAgent
	binary string
}

func (a doctorPathAgent) CLIBinaryName() string  { return a.binary }
func (a doctorPathAgent) CLIDisplayName() string { return "fixture" }

func TestCheckAgentBinary_PreservesExecutablePathWithVersion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	path := filepath.Join(t.TempDir(), "fixture-agent")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf 'fixture 1.0\\n'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	results := checkAgentBinary(context.Background(), &doctorPathAgent{binary: path})
	if len(results) != 1 || !strings.Contains(results[0].Detail, path) || !strings.Contains(results[0].Detail, "fixture 1.0") {
		t.Fatalf("doctor must retain executable identity and version: %+v", results)
	}
}
