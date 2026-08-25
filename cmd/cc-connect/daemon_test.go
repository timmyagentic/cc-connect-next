package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseDaemonInstallArgs_ConfigSetsWorkDir(t *testing.T) {
	cfg, force, err := parseDaemonInstallArgs([]string{"--config", "/tmp/example/config.toml"})
	if err != nil {
		t.Fatalf("parseDaemonInstallArgs returned error: %v", err)
	}
	if force {
		t.Fatalf("force = true, want false")
	}

	want := filepath.Clean("/tmp/example")
	if cfg.ConfigPath != filepath.Clean("/tmp/example/config.toml") {
		t.Fatalf("cfg.ConfigPath = %q, want explicit migrated config", cfg.ConfigPath)
	}
	if cfg.WorkDir != want {
		t.Fatalf("cfg.WorkDir = %q, want %q", cfg.WorkDir, want)
	}
}

func TestResolveDaemonDataDirUsesDaemonWorkDir(t *testing.T) {
	t.Setenv("HOME", "/Users/demo")
	for _, tt := range []struct {
		name     string
		dataDir  string
		workDir  string
		expected string
	}{
		{name: "relative", dataDir: "state", workDir: "/srv/cc", expected: "/srv/cc/state"},
		{name: "absolute", dataDir: "/var/lib/cc-next", workDir: "/srv/cc", expected: "/var/lib/cc-next"},
		{name: "home", dataDir: "~/state", workDir: "/srv/cc", expected: "/Users/demo/state"},
		{name: "empty", dataDir: "", workDir: "/srv/cc", expected: ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			expected := tt.expected
			if expected != "" {
				expected = filepath.Clean(expected)
			}
			if got := resolveDaemonDataDir(tt.dataDir, tt.workDir); got != expected {
				t.Fatalf("resolveDaemonDataDir(%q, %q) = %q, want %q", tt.dataDir, tt.workDir, got, tt.expected)
			}
		})
	}
}

func TestParseDaemonInstallArgs_ConfigEqualsFormSetsWorkDir(t *testing.T) {
	cfg, _, err := parseDaemonInstallArgs([]string{"--config=/tmp/example/config.toml"})
	if err != nil {
		t.Fatalf("parseDaemonInstallArgs returned error: %v", err)
	}

	want := filepath.Clean("/tmp/example")
	if cfg.WorkDir != want {
		t.Fatalf("cfg.WorkDir = %q, want %q", cfg.WorkDir, want)
	}
}

func TestParseDaemonInstallArgs_NoCaptureSecretsFlag(t *testing.T) {
	const envName = "CC_DAEMON_NO_CAPTURE_SECRETS"
	previous, existed := os.LookupEnv(envName)
	if err := os.Unsetenv(envName); err != nil {
		t.Fatalf("unset test environment: %v", err)
	}
	t.Cleanup(func() {
		var err error
		if existed {
			err = os.Setenv(envName, previous)
		} else {
			err = os.Unsetenv(envName)
		}
		if err != nil {
			t.Errorf("restore test environment: %v", err)
		}
	})

	cfg, _, err := parseDaemonInstallArgs([]string{"--no-capture-secrets"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !cfg.NoCaptureSecrets {
		t.Fatal("flag should set NoCaptureSecrets=true")
	}

	cfg2, _, err := parseDaemonInstallArgs(nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg2.NoCaptureSecrets {
		t.Fatal("default must be false when flag and env are unset")
	}
}

func TestParseDaemonInstallArgs_NoCaptureSecretsEnv(t *testing.T) {
	for _, v := range []string{"1", "true", "TRUE", "yes", "on"} {
		t.Run("truthy="+v, func(t *testing.T) {
			t.Setenv("CC_DAEMON_NO_CAPTURE_SECRETS", v)
			cfg, _, err := parseDaemonInstallArgs(nil)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if !cfg.NoCaptureSecrets {
				t.Fatalf("env=%q should opt out", v)
			}
		})
	}
	for _, v := range []string{"0", "false", "", "no", "off"} {
		t.Run("falsy="+v, func(t *testing.T) {
			t.Setenv("CC_DAEMON_NO_CAPTURE_SECRETS", v)
			cfg, _, err := parseDaemonInstallArgs(nil)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if cfg.NoCaptureSecrets {
				t.Fatalf("env=%q should NOT opt out", v)
			}
		})
	}
}

func TestParseDaemonInstallArgs_NoCaptureSecretsFlagAndEnvCombine(t *testing.T) {
	// OR semantics: env=truthy + flag=present → still true.
	t.Setenv("CC_DAEMON_NO_CAPTURE_SECRETS", "1")
	cfg, _, err := parseDaemonInstallArgs([]string{"--no-capture-secrets", "--force"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !cfg.NoCaptureSecrets {
		t.Fatal("flag+env both should leave NoCaptureSecrets=true")
	}
	// env=truthy without flag → still true.
	cfg2, _, err := parseDaemonInstallArgs([]string{"--force"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !cfg2.NoCaptureSecrets {
		t.Fatal("env=1 alone should opt out")
	}
}

func TestParseDaemonInstallArgs_WorkDirOverridesConfig(t *testing.T) {
	cfg, force, err := parseDaemonInstallArgs([]string{
		"--config", "/tmp/example/config.toml",
		"--work-dir", "/tmp/override",
		"--force",
	})
	if err != nil {
		t.Fatalf("parseDaemonInstallArgs returned error: %v", err)
	}
	if !force {
		t.Fatalf("force = false, want true")
	}

	want := filepath.Clean("/tmp/override")
	if cfg.WorkDir != want {
		t.Fatalf("cfg.WorkDir = %q, want %q", cfg.WorkDir, want)
	}
	if cfg.ConfigPath != filepath.Clean("/tmp/example/config.toml") {
		t.Fatalf("cfg.ConfigPath = %q, want config independent from work dir", cfg.ConfigPath)
	}
}
