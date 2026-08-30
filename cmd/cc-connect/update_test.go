package main

import (
	"strings"
	"testing"
	"time"
)

func withCLIUpdateVersion(t *testing.T, value string) {
	t.Helper()
	previous := version
	version = value
	t.Cleanup(func() { version = previous })
}

func setCachedUpdate(version string, at time.Time) {
	cachedLatestVersion.mu.Lock()
	cachedLatestVersion.version = version
	cachedLatestVersion.timestamp = at
	cachedLatestVersion.mu.Unlock()
}

func TestGetUpdateHintIfAvailableNeverBlocksOnCacheMiss(t *testing.T) {
	withCLIUpdateVersion(t, "v1.0.0")
	setCachedUpdate("", time.Time{})
	started := time.Now()
	if hint := getUpdateHintIfAvailable(); hint != "" {
		t.Fatalf("cache miss hint = %q", hint)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("cache-only hint blocked for %v", elapsed)
	}
}

func TestGetUpdateHintIfAvailableUsesStableFoundationComparison(t *testing.T) {
	withCLIUpdateVersion(t, "v1.2.3-beta.2")
	setCachedUpdate("v1.2.3", time.Now())
	if hint := getUpdateHintIfAvailable(); !strings.Contains(hint, "v1.2.3-beta.2 → v1.2.3") {
		t.Fatalf("stable promotion hint = %q", hint)
	}

	withCLIUpdateVersion(t, "v1.2.3")
	setCachedUpdate("v1.2.3", time.Now())
	if hint := getUpdateHintIfAvailable(); hint != "" {
		t.Fatalf("same-version hint = %q", hint)
	}
}

func TestGetUpdateHintIfAvailableSkipsDev(t *testing.T) {
	withCLIUpdateVersion(t, "dev")
	setCachedUpdate("v9.9.9", time.Now())
	if hint := getUpdateHintIfAvailable(); hint != "" {
		t.Fatalf("dev hint = %q", hint)
	}
}

func TestValidateStableUpdateArgsRejectsPrereleaseAndUnknownFlags(t *testing.T) {
	if err := validateStableUpdateArgs(nil); err != nil {
		t.Fatalf("no args: %v", err)
	}
	for _, argument := range []string{"--pre", "--beta"} {
		err := validateStableUpdateArgs([]string{argument})
		if err == nil || !strings.Contains(err.Error(), "stable") {
			t.Fatalf("%s error = %v", argument, err)
		}
	}
	if err := validateStableUpdateArgs([]string{"--force"}); err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("unknown flag error = %v", err)
	}
}
