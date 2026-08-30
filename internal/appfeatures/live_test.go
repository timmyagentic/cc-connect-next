package appfeatures

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// This opt-in test installs the current public stable release over a disposable
// executable. It never touches the running daemon, config, or installed binary.
func TestLiveUpdateServiceAgainstPublishedStableRelease(t *testing.T) {
	if os.Getenv("CC_NEXT_RUN_LIVE_FEATURE_FOUNDATION") != "1" {
		t.Skip("set CC_NEXT_RUN_LIVE_FEATURE_FOUNDATION=1 to exercise public release assets")
	}
	target := filepath.Join(t.TempDir(), ProductName)
	if err := os.WriteFile(target, versionScript("v0.2.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	service, err := NewUpdateService(UpdateConfig{CurrentVersion: "v0.2.0", ExecutablePath: target})
	if err != nil {
		t.Fatalf("NewUpdateService: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	plan, err := service.Prepare(ctx)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if !plan.Available() {
		t.Fatalf("expected a public release newer than v0.2.0: %#v", plan.Release())
	}
	t.Logf("prepared release=%s asset=%s notes_bytes=%d", plan.Release().Tag, plan.ArchiveAsset().Name, len(plan.Release().Notes))
	result, err := service.Apply(ctx, plan)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !result.Updated || result.Release.Tag != plan.Release().Tag {
		t.Fatalf("result = %#v, plan = %#v", result, plan.Release())
	}
	output, err := exec.Command(target, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("run installed release: %v", err)
	}
	firstLine, _, _ := strings.Cut(strings.TrimSpace(string(output)), "\n")
	if firstLine != ProductName+" "+result.Release.Tag {
		t.Fatalf("installed output = %q, release = %q", output, result.Release.Tag)
	}
}
