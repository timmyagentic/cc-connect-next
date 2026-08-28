//go:build windows

package processgroup

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// Prepare creates a process group that taskkill /T can terminate as one tree.
func Prepare(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= syscall.CREATE_NEW_PROCESS_GROUP
}

// Terminate asks the process tree to close without taskkill's force flag.
func Terminate(cmd *exec.Cmd) error {
	return taskkill(cmd, false)
}

// Kill forcefully terminates the process tree.
func Kill(cmd *exec.Cmd) error {
	return taskkill(cmd, true)
}

func taskkill(cmd *exec.Cmd, force bool) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	args := []string{"/T"}
	if force {
		args = append(args, "/F")
	}
	args = append(args, "/PID", strconv.Itoa(cmd.Process.Pid))
	output, err := exec.Command("taskkill", args...).CombinedOutput()
	if err == nil || taskkillProcessGone(output) {
		return nil
	}

	var fallbackErr error
	if force {
		fallbackErr = cmd.Process.Kill()
	} else {
		fallbackErr = cmd.Process.Signal(os.Interrupt)
	}
	if fallbackErr == nil || errors.Is(fallbackErr, os.ErrProcessDone) {
		return nil
	}
	return fmt.Errorf("taskkill failed: %w: %s; process signal fallback failed: %w", err, taskkillOutput(output), fallbackErr)
}

func taskkillProcessGone(output []byte) bool {
	lower := bytes.ToLower(output)
	return bytes.Contains(lower, []byte("there is no running instance")) ||
		bytes.Contains(lower, []byte("not found"))
}

func taskkillOutput(output []byte) string {
	if trimmed := strings.TrimSpace(string(output)); trimmed != "" {
		return trimmed
	}
	return "(empty output)"
}
