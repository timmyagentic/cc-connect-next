package privacyguard

import (
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

func TestPolicyRejectsInvalidDirectoryCategories(t *testing.T) {
	for _, path := range []string{"", "relative", "/", "/tmp/bad\npath", "/tmp/bad\x00path", "/tmp/glob*"} {
		if err := ValidateDirectories([]string{path}); err == nil {
			t.Errorf("accepted %q", path)
		}
	}
}

func TestProfileHomeWorkdirDoesNotAllowPersonalDirectories(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	denied, allowed, err := paths(home, home, Settings{Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(allowed) != 0 {
		t.Fatal("home work_dir must not exempt protected descendants")
	}
	if !slices.Contains(denied, filepath.Join(home, "Library", "Containers")) {
		t.Fatal("App Data container protection missing")
	}
}

func TestPathsExemptOnlySelectedWorkspaceCategory(t *testing.T) {
	home := filepath.Join(t.TempDir(), `home "quoted"`)
	project := filepath.Join(home, "Documents", `project\name`)
	denied, allowed, err := paths(home, project, Settings{Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(allowed) != 1 || allowed[0] != filepath.Join(home, "Documents") {
		t.Fatal("selected workspace directory category missing")
	}
	if !slices.Contains(denied, filepath.Join(home, "Desktop")) {
		t.Fatal("workspace exception removed unrelated category protection")
	}
}

func TestConfigureCopiesSettings(t *testing.T) {
	old := currentSettings()
	t.Cleanup(func() { _ = Configure(old) })
	paths := []string{"Documents"}
	if err := Configure(Settings{Enabled: true, AllowDirectories: paths}); err != nil {
		t.Fatal(err)
	}
	want := paths[0]
	paths[0] = "changed"
	if currentSettings().AllowDirectories[0] != want {
		t.Fatal("configuration retained caller-owned mutable slice")
	}
}

func TestNonDarwinPolicyRemainsInactive(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("non-macOS shared configuration")
	}
	if err := Configure(Settings{Enabled: true, AllowDirectories: []string{"Documents"}}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Configure(Settings{}) })
	if Active() {
		t.Fatal("macOS policy active on another platform")
	}
}
