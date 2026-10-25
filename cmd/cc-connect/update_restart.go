package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/timmyagentic/cc-connect-next/core"
)

// npm may rename the directory containing the running binary. An update must
// restart the verified installation target, not the old process's moved inode.
func executableForRestart(request core.RestartRequest) (string, error) {
	if request.UpdateID != "" && request.ExecutablePath != "" {
		if !filepath.IsAbs(request.ExecutablePath) {
			return "", fmt.Errorf("update executable path must be absolute")
		}
		info, err := os.Stat(request.ExecutablePath)
		if err != nil {
			return "", err
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("update executable is not a regular file")
		}
		return request.ExecutablePath, nil
	}
	return os.Executable()
}
