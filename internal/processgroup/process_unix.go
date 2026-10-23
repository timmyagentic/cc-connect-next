//go:build unix

package processgroup

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// Prepare isolates a child in its own process group so its descendants can be
// terminated together instead of surviving as orphans.
func Prepare(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// Terminate asks the whole process group to exit gracefully.
func Terminate(cmd *exec.Cmd) error {
	return signal(cmd, syscall.SIGTERM)
}

// Kill forcefully terminates the whole process group.
func Kill(cmd *exec.Cmd) error {
	return signal(cmd, syscall.SIGKILL)
}

func signal(cmd *exec.Cmd, sig syscall.Signal) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	if err := syscall.Kill(-cmd.Process.Pid, sig); err != nil &&
		!errors.Is(err, os.ErrProcessDone) && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}
