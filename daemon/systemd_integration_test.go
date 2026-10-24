//go:build linux && systemd_integration

package daemon

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Run only on a disposable Linux machine with systemd as PID 1:
// go test -c -tags=systemd_integration -o /tmp/daemon-systemd.test ./daemon
// sudo /tmp/daemon-systemd.test -test.run '^TestSystemdWorkingDirectoryRegression$' -test.v
// An unavailable service manager is a failure, not a skipped acceptance check.
func TestSystemdWorkingDirectoryRegression(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("real systemd validation requires root on a disposable Linux machine")
	}
	comm, err := os.ReadFile("/proc/1/comm")
	if err != nil || strings.TrimSpace(string(comm)) != "systemd" {
		t.Fatalf("real systemd validation requires systemd as PID 1: %q (%v)", comm, err)
	}
	for _, name := range []string{"systemctl", "systemd-analyze"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Fatal(err)
		}
	}

	for i, name := range []string{
		"plain",
		"workspace with spaces",
		"percent-%n-%i-100%",
		`quotes-"double"-'single'-back\slash`,
		"trailing-backslash\\",
		"项目目录",
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			workDir := filepath.Join(root, name)
			binDir := filepath.Join(root, "bin space%")
			configDir := filepath.Join(root, "independent config%")
			for _, dir := range []string{workDir, binDir, configDir} {
				if err := os.MkdirAll(dir, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			configPath := filepath.Join(configDir, "config.toml")
			if err := os.WriteFile(configPath, []byte("fixture"), 0o600); err != nil {
				t.Fatal(err)
			}
			binaryPath := filepath.Join(binDir, "probe")
			// Check the independently located --config argument before staying alive
			// long enough for the parent to inspect the process's actual cwd.
			probe := "#!/bin/sh\nset -eu\n[ \"$#\" = 2 ]\n[ \"$1\" = --config ]\n[ -f \"$2\" ]\nprintf '%s' \"$2\" > \"$2.proof\"\nexec /bin/sleep 60\n"
			if err := os.WriteFile(binaryPath, []byte(probe), 0o700); err != nil {
				t.Fatal(err)
			}

			unitName := fmt.Sprintf("cc-connect-next-systemd-test-%d-%d.service", os.Getpid(), i)
			unitPath := filepath.Join("/run/systemd/system", unitName)
			f, err := os.OpenFile(unitPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if t.Failed() {
					out, _ := systemdTestCommand("systemctl", "show", unitName, "--property=WorkingDirectory,Result,ExecMainStatus")
					t.Logf("failed service: %s", out)
				}
				if out, err := systemdTestCommand("systemctl", "stop", unitName); err != nil {
					if !strings.Contains(string(out), "not loaded") {
						t.Errorf("cleanup stop: %s (%v)", out, err)
					}
				}
				if out, err := systemdTestCommand("systemctl", "reset-failed", unitName); err != nil {
					if !strings.Contains(string(out), "not loaded") {
						t.Errorf("cleanup reset-failed: %s (%v)", out, err)
					}
				}
				if err := os.Remove(unitPath); err != nil {
					t.Error(err)
				}
				if out, err := systemdTestCommand("systemctl", "daemon-reload"); err != nil {
					t.Errorf("cleanup daemon-reload: %s (%v)", out, err)
				}
			})
			unit := (&systemdManager{system: true}).buildUnit(Config{
				BinaryPath: binaryPath,
				ConfigPath: configPath,
				WorkDir:    workDir,
				LogFile:    filepath.Join(root, "service.log"),
			})
			_, writeErr := f.WriteString(unit)
			closeErr := f.Close()
			if writeErr != nil || closeErr != nil {
				t.Fatalf("write unit: %v; close: %v", writeErr, closeErr)
			}
			for _, command := range [][]string{
				{"systemd-analyze", "verify", unitPath},
				{"systemctl", "daemon-reload"},
				{"systemctl", "start", unitName},
			} {
				if out, err := systemdTestCommand(command[0], command[1:]...); err != nil {
					t.Fatalf("%v: %s (%v)", command, out, err)
				}
			}
			out, err := systemdTestCommand("systemctl", "show", unitName, "--property=MainPID", "--value")
			if err != nil {
				t.Fatalf("read MainPID: %s (%v)", out, err)
			}
			pid, err := strconv.Atoi(strings.TrimSpace(string(out)))
			if err != nil || pid <= 0 {
				t.Fatalf("invalid service MainPID: %q (%v)", out, err)
			}
			proofPath := configPath + ".proof"
			deadline := time.Now().Add(5 * time.Second)
			for {
				proof, err := os.ReadFile(proofPath)
				if err == nil && string(proof) == configPath {
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("service did not receive the literal config argument: %q (%v)", proof, err)
				}
				time.Sleep(20 * time.Millisecond)
			}
			cwd, err := os.Readlink(fmt.Sprintf("/proc/%d/cwd", pid))
			if err != nil || cwd != workDir {
				t.Fatalf("service cwd = %q (%v), want %q", cwd, err, workDir)
			}
		})
	}
}

func systemdTestCommand(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}
