package core

import (
	"fmt"
	"path/filepath"
	"strings"
)

// localStateBrief is private to the local Agent prompt, not the public Manifest.
// Supply the effective directory without listing its contents or configuration.
func (e *Engine) localStateBrief() string {
	if strings.TrimSpace(e.dataDir) == "" {
		return ""
	}
	dir, err := filepath.Abs(e.dataDir)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("\n[cc-connect-next local state]\nEffective data directory (quoted path): %q. Use `cc-connect-next capabilities` to find the supported read-only CLI/API before locating state files. Search only the project workspace, this effective data directory, and explicit installation locations relevant to the task. Do not assume the legacy ~/.cc-connect directory is active. Do not recursively search the user's home directory when a lookup fails; request a specific location if needed. Access Desktop, Documents, Downloads, Photos, or media libraries only when the user's task requires it.\n", dir)
}
