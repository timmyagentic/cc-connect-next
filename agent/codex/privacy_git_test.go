package codex

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestPrivacyGitExplicitChildAllowsAncestorTraversalOnly(t *testing.T) {
	root := t.TempDir()
	desktop := filepath.Join(root, "Desktop")
	repo := filepath.Join(root, "project")
	gitdir := filepath.Join(desktop, "allowed", "gitdir")
	for _, dir := range []string{repo, gitdir} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, ".git"), []byte("gitdir: "+gitdir+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	paths, err := privacyGitWritePaths(repo, []string{desktop}, []string{filepath.Dir(gitdir)})
	if err != nil || !reflect.DeepEqual(paths, []string{gitdir}) {
		t.Fatalf("explicit child rejected: paths=%v err=%v", paths, err)
	}
	paths, err = privacyGitWritePaths(repo, []string{desktop}, nil)
	if err != nil || len(paths) != 0 {
		t.Fatalf("protected target granted without exception: paths=%v err=%v", paths, err)
	}
}

func TestPrivacyGitCaseVariantCannotReopenDeniedMetadata(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS case-insensitive path protection")
	}
	if privacyGitPathAllowed("/synthetic/desktop/private", []string{"/synthetic/Desktop"}, nil) {
		t.Fatal("case variant bypassed personal-directory policy")
	}
}

func TestPrivacyGitNewProjectBelowDeniedParentGetsOnlyOwnMetadata(t *testing.T) {
	desktop := filepath.Join(t.TempDir(), "Desktop")
	repo := filepath.Join(desktop, "project")
	if err := os.MkdirAll(repo, 0700); err != nil {
		t.Fatal(err)
	}
	paths, err := privacyGitWritePaths(repo, []string{desktop}, []string{repo})
	if err != nil || !reflect.DeepEqual(paths, []string{filepath.Join(repo, ".git")}) {
		t.Fatalf("new project metadata scope: paths=%v err=%v", paths, err)
	}
}
