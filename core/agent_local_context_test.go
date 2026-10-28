package core

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestBuildCapabilityPrompt_Issue166UsesEffectiveDataDirWithoutHomeSearch(t *testing.T) {
	e := &Engine{}
	e.SetCapabilityBrief("[cc-connect-next capability brief]")
	dir := filepath.Join(t.TempDir(), "custom state with \"quotes\"")
	e.SetDataDir(dir)
	state := &interactiveState{}
	first := e.buildCapabilityPrompt(state, "find the scheduled task")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(first, strconv.Quote(executable)) {
		t.Fatal("Agent must know the exact executable even when a login shell replaces PATH")
	}
	for _, want := range []string{strconv.Quote(dir), "Do not recursively search", "~/.cc-connect", "capabilities"} {
		if !strings.Contains(first, want) {
			t.Errorf("local Agent context is missing %q", want)
		}
	}
	if got := e.buildCapabilityPrompt(state, "continue"); got != "continue" {
		t.Fatal("local state context must not be repeated on every input")
	}
	if got := e.buildCapabilityPrompt(&interactiveState{}, "new conversation"); !strings.Contains(got, strconv.Quote(dir)) {
		t.Fatal("a new conversation must receive the effective data directory")
	}
}
