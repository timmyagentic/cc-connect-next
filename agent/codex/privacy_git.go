package codex

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var errPrivacyGitDenied = errors.New("Git metadata resolves into a protected path")

func privacyPathWithin(root, path string) bool {
	// Default macOS volumes are case-insensitive. Treat case variants
	// conservatively even on case-sensitive volumes before following pointers.
	if runtime.GOOS == "darwin" {
		root, path = strings.ToLower(root), strings.ToLower(path)
	}
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func privacyGitPathAllowed(path string, denied, allowed []string) bool {
	length, permitted := -1, true
	for _, root := range denied {
		if privacyPathWithin(root, path) && len(root) > length {
			length, permitted = len(root), false
		}
	}
	for _, root := range allowed {
		if privacyPathWithin(root, path) && len(root) >= length {
			length, permitted = len(root), true
		}
	}
	return permitted
}

// Resolve only metadata paths, checking every symlink target before inspecting
// it. EvalSymlinks alone could traverse a protected target before we check it.
// The explicitly selected cwd is the trusted starting directory.
func resolvePrivacyGitPath(path, cwd string, denied, allowed []string) (string, os.FileInfo, error) {
	path = filepath.Clean(path)
	for links := 0; links < 40; links++ {
		if !privacyGitPathAllowed(path, denied, allowed) {
			return "", nil, errPrivacyGitDenied
		}
		base := string(filepath.Separator)
		if privacyPathWithin(cwd, path) {
			base = cwd
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return "", nil, err
		}
		parts := strings.Split(rel, string(filepath.Separator))
		var info os.FileInfo
		restart := false
		for i, part := range parts {
			base = filepath.Join(base, part)
			// An explicitly allowed child requires metadata traversal through
			// its denied ancestors, but never directory enumeration. Recheck
			// the complete destination after resolving every symlink.
			info, err = os.Lstat(base)
			if os.IsNotExist(err) {
				return filepath.Join(append([]string{base}, parts[i+1:]...)...), nil, nil
			}
			if err != nil {
				return "", nil, err
			}
			if info.Mode()&os.ModeSymlink == 0 {
				continue
			}
			target, err := os.Readlink(base)
			if err != nil {
				return "", nil, err
			}
			if !filepath.IsAbs(target) {
				target = filepath.Join(filepath.Dir(base), target)
			}
			path = filepath.Join(append([]string{target}, parts[i+1:]...)...)
			if !privacyGitPathAllowed(path, denied, allowed) {
				return "", nil, errPrivacyGitDenied
			}
			restart = true
			break
		}
		if !restart {
			return base, info, nil
		}
	}
	return "", nil, fmt.Errorf("too many Git metadata symlinks")
}

func readPrivacyGitPointer(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 4097))
	if err != nil {
		return "", err
	}
	if len(data) > 4096 {
		return "", fmt.Errorf("Git metadata pointer exceeds 4096 bytes")
	}
	return strings.TrimSpace(string(data)), nil
}

// A normal repository needs .git write access; linked worktrees additionally
// need their gitdir and common directory. No Git subprocess is used for lookup.
// Denied targets receive no grant, even if the project's .git points to them.
func privacyGitWritePaths(cwd string, denied, allowed []string) ([]string, error) {
	fallback := filepath.Join(cwd, ".git")
	if _, err := os.Stat(cwd); os.IsNotExist(err) {
		return []string{fallback}, nil
	}
	for dir := cwd; ; dir = filepath.Dir(dir) {
		marker, info, err := resolvePrivacyGitPath(filepath.Join(dir, ".git"), cwd, denied, allowed)
		if errors.Is(err, errPrivacyGitDenied) {
			if dir != cwd && privacyGitPathAllowed(fallback, denied, allowed) {
				return []string{fallback}, nil
			}
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("codex macOS privacy inspect Git metadata: %w", err)
		}
		if info != nil {
			gitDir := marker
			if !info.IsDir() {
				if !info.Mode().IsRegular() {
					return nil, fmt.Errorf("codex macOS privacy: unsupported Git metadata file")
				}
				pointer, err := readPrivacyGitPointer(marker)
				if err != nil {
					return nil, err
				}
				target, ok := strings.CutPrefix(pointer, "gitdir: ")
				if !ok {
					return nil, fmt.Errorf("codex macOS privacy: invalid Git directory pointer")
				}
				if !filepath.IsAbs(target) {
					target = filepath.Join(filepath.Dir(marker), target)
				}
				gitDir, _, err = resolvePrivacyGitPath(target, cwd, denied, allowed)
				if errors.Is(err, errPrivacyGitDenied) {
					return nil, nil
				}
				if err != nil {
					return nil, err
				}
			}
			commonFile, commonInfo, err := resolvePrivacyGitPath(filepath.Join(gitDir, "commondir"), cwd, denied, allowed)
			if errors.Is(err, errPrivacyGitDenied) {
				return nil, nil
			}
			if err != nil {
				return nil, err
			}
			if commonInfo == nil {
				return []string{gitDir}, nil
			}
			if !commonInfo.Mode().IsRegular() {
				return nil, fmt.Errorf("codex macOS privacy: unsupported Git common directory pointer")
			}
			target, err := readPrivacyGitPointer(commonFile)
			if err != nil {
				return nil, err
			}
			if !filepath.IsAbs(target) {
				target = filepath.Join(gitDir, target)
			}
			commonDir, _, err := resolvePrivacyGitPath(target, cwd, denied, allowed)
			if errors.Is(err, errPrivacyGitDenied) {
				return nil, nil
			}
			if err != nil {
				return nil, err
			}
			return []string{gitDir, commonDir}, nil
		}
		if dir == filepath.Dir(dir) {
			break
		}
	}
	return []string{fallback}, nil
}
