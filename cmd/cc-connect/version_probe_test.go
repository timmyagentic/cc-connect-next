package main

import (
	"bytes"
	"context"
	"flag"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	featureupdater "github.com/timmyagentic/awesome-agent-app-features/updater"
)

// Run the actual entry point in an isolated process: flags, os.Exit, and daemon
// log setup must be exercised together to reproduce issue #126.
func TestVersionProbeSubprocess(t *testing.T) {
	if os.Getenv("CCN_TEST_VERSION_PROBE") != "1" {
		return
	}
	flag.CommandLine = flag.NewFlagSet("cc-connect-next", flag.ExitOnError)
	os.Args = []string{"cc-connect-next", "--version"}
	version, commit, buildTime = "v1.2.3", "probe-commit", "probe-time"
	http.DefaultTransport = capabilityRoundTripFunc(func(*http.Request) (*http.Response, error) { return nil, context.Canceled })
	main()
	os.Exit(0)
}

func TestVersionProbeWithDaemonLoggingDoesNotTouchLogs(t *testing.T) {
	for _, scenario := range []string{"new log", "existing log", "invalid log parent"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			logPath := filepath.Join(dir, "daemon.log")
			original := []byte("existing daemon log must survive unchanged\n")
			if scenario == "existing log" {
				if err := os.WriteFile(logPath, original, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "invalid log parent" {
				parent := filepath.Join(dir, "not-a-directory")
				if err := os.WriteFile(parent, original, 0o600); err != nil {
					t.Fatal(err)
				}
				logPath = filepath.Join(parent, "daemon.log")
			}
			t.Setenv("CCN_TEST_VERSION_PROBE", "1")
			t.Setenv("CC_LOG_FILE", logPath)
			t.Setenv("CC_LOG_MAX_SIZE", "1")
			t.Setenv("CC_LOG_MAX_BACKUPS", "1")
			binary, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(binary, "-test.run=^TestVersionProbeSubprocess$")
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("version probe: %v; stderr=%q", err, stderr.String())
			}
			if got, want := stdout.String(), "cc-connect-next v1.2.3\ncommit:  probe-commit\nbuilt:   probe-time\n"; got != want {
				t.Errorf("stdout=%q, want %q", got, want)
			}
			if stderr.Len() != 0 {
				t.Errorf("version probe polluted stderr: %q", stderr.String())
			}
			// Keep compatibility with old updaters that merge stdout and stderr.
			verifier := featureupdater.ExactVersionLine("cc-connect-next")
			verifier.Args = []string{"-test.run=^TestVersionProbeSubprocess$"}
			if err := verifier.Verify(context.Background(), binary, "v1.2.3"); err != nil {
				t.Errorf("old updater rejects new binary: %v", err)
			}
			if scenario == "existing log" {
				data, err := os.ReadFile(logPath)
				if err != nil || !bytes.Equal(data, original) {
					t.Errorf("version probe changed log: %q, %v", data, err)
				}
			} else if scenario == "new log" {
				if _, err := os.Stat(logPath); !os.IsNotExist(err) {
					t.Errorf("version probe created log: %v", err)
				}
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), "daemon.log.") {
					t.Errorf("version probe rotated log: %s", entry.Name())
				}
			}
		})
	}
}

func TestVersionProbeSkipsAsyncUpdateCheck(t *testing.T) {
	for _, args := range [][]string{{"--version"}, {"-version"}, {"--version=true"}, {"--version=1"}, {"--config", "unused.toml", "--version"}} {
		if shouldCheckUpdateAsync(args) {
			t.Errorf("version probe can perform network I/O: %v", args)
		}
	}
	for _, args := range [][]string{nil, {"--version=false"}, {"--version=0"}, {"capabilities"}} {
		if !shouldCheckUpdateAsync(args) {
			t.Errorf("ordinary startup lost update check: %v", args)
		}
	}
}
