package codex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func resolveCodexHome(extraEnv []string) (string, error) {
	if value := getenvFromList(extraEnv, "CODEX_HOME"); value != "" {
		return strings.TrimSpace(value), nil
	}
	if value := strings.TrimSpace(os.Getenv("CODEX_HOME")); value != "" {
		return value, nil
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(homeDir, ".codex"), nil
}

func getenvFromList(env []string, key string) string {
	prefix := key + "="
	for i := len(env) - 1; i >= 0; i-- {
		entry := env[i]
		if strings.HasPrefix(entry, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(entry, prefix))
		}
	}
	return ""
}
