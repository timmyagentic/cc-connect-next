package iflow

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/timmyagentic/cc-connect-next/core"
)

func TestNormalizeMode(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", "default"},
		{"default", "default"},
		{"AUTO-EDIT", "auto-edit"},
		{"auto_edit", "auto-edit"},
		{"edit", "auto-edit"},
		{"plan", "plan"},
		{"yolo", "yolo"},
		{"force", "yolo"},
		{"unknown", "default"},
	}
	for _, tc := range cases {
		if got := normalizeMode(tc.in); got != tc.want {
			t.Fatalf("normalizeMode(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestProviderEnvLocked(t *testing.T) {
	a := &Agent{}
	a.SetProviders([]core.ProviderConfig{{
		Name:    "custom",
		APIKey:  "k1",
		BaseURL: "https://example.com/v1",
		Env:     map[string]string{"FOO": "bar"},
	}})
	a.SetActiveProvider("custom")

	got := a.providerEnvLocked()
	wantSubset := []string{
		"IFLOW_API_KEY=k1",
		"IFLOW_apiKey=k1",
		"IFLOW_BASE_URL=https://example.com/v1",
		"IFLOW_baseUrl=https://example.com/v1",
		"FOO=bar",
	}

	for _, item := range wantSubset {
		if !contains(got, item) {
			t.Fatalf("providerEnvLocked() missing %q; got=%v", item, got)
		}
	}
}

func TestIFlowProjectKey(t *testing.T) {
	path := "/Users/test/project"
	got := iflowProjectKey(path)
	if got != "-Users-test-project" {
		t.Fatalf("iflowProjectKey(%q) = %q", path, got)
	}

	if got := iflowProjectKey(""); got != "" {
		t.Fatalf("iflowProjectKey(\"\") = %q, want empty", got)
	}
}

func TestIFlowResolvedWorkDir(t *testing.T) {
	base := t.TempDir()
	realDir := filepath.Join(base, "real")
	linkDir := filepath.Join(base, "link")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if err := os.Symlink(realDir, linkDir); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	want, err := filepath.EvalSymlinks(realDir)
	if err != nil {
		t.Fatalf("EvalSymlinks(realDir): %v", err)
	}

	if got := iflowResolvedWorkDir(linkDir); got != want {
		t.Fatalf("iflowResolvedWorkDir(%q) = %q, want %q", linkDir, got, want)
	}
}

func TestExtractIFlowContentText(t *testing.T) {
	if got := extractIFlowContentText("hello"); got != "hello" {
		t.Fatalf("extractIFlowContentText string = %q", got)
	}

	arr := []any{map[string]any{"type": "text", "text": "from-array"}}
	if got := extractIFlowContentText(arr); got != "from-array" {
		t.Fatalf("extractIFlowContentText array = %q", got)
	}

	if got := extractIFlowContentText(123); got != "" {
		t.Fatalf("extractIFlowContentText unexpected = %q", got)
	}
}

func TestPermissionModesKeys(t *testing.T) {
	a := &Agent{}
	modes := a.PermissionModes()
	got := make([]string, 0, len(modes))
	for _, m := range modes {
		got = append(got, m.Key)
	}
	want := []string{"default", "auto-edit", "plan", "yolo"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("PermissionModes keys = %v, want %v", got, want)
	}
}

func TestConfiguredModels_BoundaryConditions(t *testing.T) {
	a := &Agent{}
	a.SetProviders([]core.ProviderConfig{
		{Name: "first", Models: []core.ModelOption{{Name: "first"}}},
		{Name: "second", Models: []core.ModelOption{{Name: "second"}}},
	})
	if got := a.configuredModels(); got != nil {
		t.Fatalf("configuredModels() without active provider = %v, want nil", got)
	}
	if !a.SetActiveProvider("second") {
		t.Fatal("SetActiveProvider(second) = false")
	}
	if got := a.configuredModels(); len(got) != 1 || got[0].Name != "second" {
		t.Fatalf("configuredModels() = %v, want second", got)
	}
}

func contains(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}

// TestIFlowAgent_WorkDirRaceFreeReaders pins the bug where ListSessions,
// DeleteSession, and ProjectMemoryFile read a.workDir without holding
// a.mu, while SetWorkDir mutates it under the lock. Running this with
// -race flags the data race; with the fix the race detector stays quiet.
func TestIFlowAgent_WorkDirRaceFreeReaders(t *testing.T) {
	dir := t.TempDir()
	a := &Agent{workDir: dir}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				a.SetWorkDir(filepath.Join(dir, "a"))
			} else {
				a.SetWorkDir(filepath.Join(dir, "b"))
			}
		}(i)
	}
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = a.ListSessions(context.Background())
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = a.DeleteSession(context.Background(), "no-such-session")
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = a.ProjectMemoryFile()
		}()
	}
	wg.Wait()
}
