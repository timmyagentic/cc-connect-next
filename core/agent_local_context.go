package core

import (
	"fmt"
	"os"
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
	executable := ""
	if path, err := os.Executable(); err == nil {
		executable = fmt.Sprintf(" Runtime executable (quoted path): %q. If PATH lookup fails, invoke this exact path with proper shell quoting; do not search the filesystem for the binary.", path)
	}
	return fmt.Sprintf("\n[cc-connect-next local state]\nEffective data directory (quoted path): %q.%s Use `cc-connect-next capabilities` to find the supported read-only CLI/API before locating state files. Manage cron/timer jobs through that API/CLI, not by editing internal state files. Search only the project workspace, this effective data directory, and explicit installation locations relevant to the task. Do not assume the legacy ~/.cc-connect directory is active. Do not recursively search the user's home directory when a lookup fails; request a specific location if needed. Access Desktop, Documents, Downloads, Photos, or media libraries only when the user's task requires it. A macOS privacy guard may deny personal-directory access independently of Agent approval mode. On denial, stop that access and report the missing scope once; do not retry using another tool, external service, or broader search.\n", dir, executable)
}
