package core

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Only process metadata is inspected. In particular, never request args/env,
// query TCC databases, launch an application, or request a privacy permission.
func checkDoctorProcess(ctx context.Context) []DoctorCheckResult {
	if runtime.GOOS != "darwin" {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return doctorProcessChecks(ctx, os.Getpid(), readDoctorProcess)
}

func readDoctorProcess(ctx context.Context, pid int) (int, string, error) {
	// comm is the executable name/path, not command (which includes arguments).
	out, err := exec.CommandContext(ctx, "/bin/ps", "-ww", "-p", strconv.Itoa(pid), "-o", "ppid=", "-o", "comm=").Output()
	if err != nil {
		return 0, "", err
	}
	return parseDoctorProcess(string(out))
}

func parseDoctorProcess(out string) (int, string, error) {
	out = strings.TrimSpace(out)
	end := strings.IndexAny(out, " \t")
	if end < 1 {
		return 0, "", fmt.Errorf("missing process metadata")
	}
	ppid, err := strconv.Atoi(out[:end])
	path := strings.TrimSpace(out[end:])
	if err != nil || ppid < 0 || path == "" || strings.ContainsAny(path, "\r\n") {
		return 0, "", fmt.Errorf("invalid process metadata")
	}
	return ppid, path, nil
}

func doctorProcessChecks(ctx context.Context, pid int, read func(context.Context, int) (int, string, error)) []DoctorCheckResult {
	identity := DoctorCheckResult{Name: "macOS Process Identity", Status: DoctorPass}
	seen := make(map[int]bool)
	for depth := 0; pid > 0; depth++ {
		if depth == 8 || seen[pid] || ctx.Err() != nil {
			identity.Status, identity.DetailKey = DoctorWarn, MsgDoctorProcessIncomplete
			break
		}
		seen[pid] = true
		ppid, path, err := read(ctx, pid)
		if err != nil {
			identity.Detail += fmt.Sprintf("PID=%d\n", pid)
			identity.Status, identity.DetailKey = DoctorWarn, MsgDoctorProcessIncomplete
			break
		}
		identity.Detail += fmt.Sprintf("PID=%d PPID=%d executable=%s\n", pid, ppid, path)
		pid = ppid
	}
	identity.Detail = strings.TrimSpace(identity.Detail)
	return []DoctorCheckResult{identity, {
		Name: "macOS Privacy Permissions", Status: DoctorWarn, DetailKey: MsgDoctorPrivacyUnknown,
	}}
}
