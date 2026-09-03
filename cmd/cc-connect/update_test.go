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

func TestParseUpdateOptionsRejectsUnknownAndAmbiguousValues(t *testing.T) {
	for _, args := range [][]string{{"--force"}, {"--channel"}, {"--channel", "nightly"}, {"stable", "beta"}} {
		if _, err := parseUpdateOptions(args); err == nil {
			t.Fatalf("parseUpdateOptions(%v) accepted invalid input", args)
		}
	}
}

func TestParseUpdateOptionsRejectsLegacyPrereleaseAliases(t *testing.T) {
	for _, alias := range []string{"--pre", "--beta"} {
		if _, err := parseUpdateOptions([]string{alias}); err == nil {
			t.Fatalf("parseUpdateOptions(%q) accepted a documented rejected alias", alias)
		}
	}
}

func TestParseUpdateOptionsSupportsExplicitStableAndBetaChannels(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{want: "stable"},
		{args: []string{"stable"}, want: "stable"},
		{args: []string{"beta"}, want: "beta"},
		{args: []string{"--channel", "beta"}, want: "beta"},
	} {
		got, err := parseUpdateOptions(test.args)
		if err != nil || string(got.Channel) != test.want {
			t.Fatalf("parseUpdateOptions(%v) = %#v, %v; want %s", test.args, got, err, test.want)
		}
	}
	if _, err := parseUpdateOptions([]string{"--channel", "nightly"}); err == nil {
		t.Fatal("unknown update channel was accepted")
	}
}
