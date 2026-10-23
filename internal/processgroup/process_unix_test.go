//go:build unix

package processgroup

import (
	"os/exec"
	"syscall"
	"testing"
)

func TestPrepareIsolatesProcessGroup(t *testing.T) {
	cmd := exec.Command("/bin/true")
	cmd.SysProcAttr = &syscall.SysProcAttr{Foreground: false}
	Prepare(cmd)
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setpgid {
		t.Fatal("Prepare did not preserve SysProcAttr and enable Setpgid")
	}
	Prepare(nil)
}

func TestSignalsWithoutProcessAreNoOps(t *testing.T) {
	cmd := exec.Command("/bin/true")
	if err := Terminate(cmd); err != nil {
		t.Fatalf("Terminate(unstarted) = %v", err)
	}
	if err := Kill(cmd); err != nil {
		t.Fatalf("Kill(unstarted) = %v", err)
	}
	if err := Kill(nil); err != nil {
		t.Fatalf("Kill(nil) = %v", err)
	}
}

func TestKillTerminatesChildProcessGroup(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-c", "sleep 60 & wait")
	Prepare(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { _ = Kill(cmd); _ = cmd.Wait() })
	if err := Kill(cmd); err != nil {
		t.Fatalf("Kill: %v", err)
	}
	_ = cmd.Wait()
	if err := Kill(cmd); err != nil {
		t.Fatalf("second Kill should be a no-op: %v", err)
	}
}
