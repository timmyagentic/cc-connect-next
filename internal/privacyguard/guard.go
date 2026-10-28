// Package privacyguard supplies macOS personal-directory policy to adapters
// with native filesystem enforcement. It never wraps an Agent in a second
// sandbox or changes the user's operating-system permissions.
package privacyguard

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"unicode"
)

type Settings struct {
	Enabled          bool
	AllowDirectories []string
}

var policy = struct {
	sync.RWMutex
	settings Settings
}{}

// Configure applies only to subsequently spawned processes. Existing processes
// retain their inherited policy until stopped. Runtime bootstrap enables the
// default policy after loading configuration; library embedders opt in here.
func Configure(settings Settings) error {
	if settings.Enabled {
		if err := ValidateDirectories(settings.AllowDirectories); err != nil {
			return err
		}
	}
	settings.AllowDirectories = append([]string(nil), settings.AllowDirectories...)
	policy.Lock()
	policy.settings = settings
	policy.Unlock()
	return nil
}

func ValidateDirectories(directories []string) error {
	for _, value := range directories {
		if !slices.Contains(protectedRelativePaths, value) {
			return fmt.Errorf("macos_privacy.allow_directories contains an unsupported directory category")
		}
	}
	return nil
}

func DirectoryNames() []string { return append([]string(nil), protectedRelativePaths...) }

func Active() bool { return runtime.GOOS == "darwin" && currentSettings().Enabled }

// Keep this list explicit: denying all of Library breaks tool configuration and
// keychain access. Do not enumerate these directories to discover protections.
var protectedRelativePaths = []string{
	"Desktop", "Documents", "Downloads", "Pictures", "Music", "Movies",
	"Library/Containers", "Library/Group Containers",
	"Library/Mail", "Library/Messages", "Library/Safari", "Library/Calendars",
	"Library/Reminders", "Library/HomeKit", "Library/IdentityServices",
	"Library/PersonalizationPortrait", "Library/Metadata/CoreSpotlight",
	"Library/Mobile Documents", "Library/CloudStorage",
}

func inside(root, path string) bool {
	return path == root || strings.HasPrefix(path, root+string(filepath.Separator))
}

func Paths(workDir string) (denied, allowed []string, err error) {
	if !Active() {
		return nil, nil, nil
	}
	home, err := userHome()
	if err != nil {
		return nil, nil, err
	}
	return paths(home, workDir, currentSettings())
}

func paths(home, workDir string, settings Settings) (denied, allowed []string, err error) {
	if !filepath.IsAbs(home) || filepath.Clean(home) == "/" || strings.ContainsFunc(home, unicode.IsControl) || strings.ContainsAny(home, "*?[]{}") {
		return nil, nil, fmt.Errorf("macOS privacy guard: invalid home directory")
	}
	if err := ValidateDirectories(settings.AllowDirectories); err != nil {
		return nil, nil, err
	}
	home = filepath.Clean(home)
	categories := append([]string(nil), settings.AllowDirectories...)
	if workDir != "" {
		abs, err := filepath.Abs(workDir)
		if err != nil {
			return nil, nil, fmt.Errorf("macOS privacy guard: resolve work directory: %w", err)
		}
		if strings.ContainsFunc(abs, unicode.IsControl) || strings.ContainsAny(abs, "*?[]{}") {
			return nil, nil, fmt.Errorf("macOS privacy guard: work directory must be a literal path without control characters or globs")
		}
		// Native deny rules also deny ancestor metadata used by Git. Preserve
		// the existing mode's access to the selected workspace's category.
		// A project at ~ or / must not exempt all personal directories.
		for _, rel := range protectedRelativePaths {
			if inside(filepath.Join(home, rel), abs) {
				categories = append(categories, rel)
				break
			}
		}
	}
	for _, rel := range protectedRelativePaths {
		root := filepath.Join(home, rel)
		denied = append(denied, root)
		if slices.Contains(categories, rel) {
			allowed = append(allowed, root)
		}
	}
	return denied, allowed, nil
}

func currentSettings() Settings {
	policy.RLock()
	defer policy.RUnlock()
	return policy.settings
}

func userHome() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("macOS privacy guard: home directory: %w", err)
	}
	// Resolve only the home itself, never probe protected children.
	resolved, err := filepath.EvalSymlinks(home)
	if err != nil {
		return "", fmt.Errorf("macOS privacy guard: resolve home directory: %w", err)
	}
	return resolved, nil
}
