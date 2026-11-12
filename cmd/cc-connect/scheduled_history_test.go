package main

import (
	"bytes"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/timmyagentic/cc-connect-next/config"
	"github.com/timmyagentic/cc-connect-next/core"
)

func TestOpenScheduledHistory_OptIn(t *testing.T) {
	for _, tt := range []struct {
		name, input string
		enabled     bool
	}{
		{"omitted", "", false},
		{"disabled", "[history]\nenabled = false", false},
		{"enabled", "[history]\nenabled = true", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var cfg config.Config
			if _, err := toml.Decode(tt.input, &cfg); err != nil {
				t.Fatal(err)
			}
			cfg.DataDir = filepath.Join(t.TempDir(), "data")
			history, err := openScheduledHistory(&cfg)
			if err != nil {
				t.Fatal(err)
			}
			if history != nil {
				t.Cleanup(func() { _ = history.Close() })
			}
			if (history != nil) != tt.enabled {
				t.Fatalf("history = %v, enabled = %v", history, tt.enabled)
			}
			if !tt.enabled {
				if _, err := os.Stat(cfg.DataDir); !os.IsNotExist(err) {
					t.Fatalf("disabled history touched the data directory: %v", err)
				}
			}

			// Both JSON stores keep working with recording disabled, and
			// both record definitions when the same startup flag is enabled.
			cron, err := core.NewCronStore(cfg.DataDir)
			if err != nil {
				t.Fatal(err)
			}
			cron.SetHistory(history)
			if err := cron.Add(&core.CronJob{ID: "cron", Exec: "echo cron", CronExpr: "0 * * * *", Enabled: true}); err != nil {
				t.Fatal(err)
			}
			timer, err := core.NewTimerStore(cfg.DataDir)
			if err != nil {
				t.Fatal(err)
			}
			timer.SetHistory(history)
			if err := timer.Add(&core.TimerJob{ID: "timer", Exec: "echo timer", ScheduledAt: time.Now().Add(time.Hour)}); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(cfg.DataDir, "history.sqlite")
			if !tt.enabled {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("disabled scheduler created history: %v", err)
				}
				return
			}
			db, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := db.Close(); err != nil {
					t.Errorf("close history database: %v", err)
				}
			}()
			var kinds int
			if err := db.QueryRow("SELECT count(DISTINCT kind) FROM job_versions").Scan(&kinds); err != nil || kinds != 2 {
				t.Fatalf("recorded scheduler kinds = %d, err = %v", kinds, err)
			}
		})
	}
}

func TestOpenScheduledHistory_DisabledPreservesExistingDatabase(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.sqlite")
	want := []byte("existing database must not be opened, migrated, or removed")
	if err := os.WriteFile(path, want, 0o600); err != nil {
		t.Fatal(err)
	}
	history, err := openScheduledHistory(&config.Config{DataDir: dir})
	if history != nil || err != nil {
		t.Fatalf("disabled history = %v, err = %v", history, err)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("existing history changed: %q, err = %v", got, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("disabled history created side files: %v, err = %v", entries, err)
	}
}

func TestHistoryConfig_RejectsNonBooleanEnabled(t *testing.T) {
	var cfg config.Config
	if _, err := toml.Decode("[history]\nenabled = \"true\"", &cfg); err == nil {
		t.Fatal("history.enabled accepted a string instead of a boolean")
	}
}
