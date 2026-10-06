package claudecode

import (
	"context"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/timmyagentic/cc-connect-next/agent/internal/processgroup"
	"github.com/timmyagentic/cc-connect-next/core"
)

func TestClaudeLifecycleHelper(t *testing.T) {
	mode := os.Getenv("CC_CLAUDE_LIFECYCLE_HELPER")
	if mode == "" {
		return
	}
	switch mode {
	case "inherited-child":
		time.Sleep(8 * time.Second)
	case "inherit-stderr":
		bin, _ := os.Executable()
		child := exec.Command(bin, "-test.run=^TestClaudeLifecycleHelper$", "--")
		child.Env = core.MergeEnv(os.Environ(), []string{"CC_CLAUDE_LIFECYCLE_HELPER=inherited-child"})
		child.Stderr = os.Stderr
		if err := child.Start(); err != nil {
			os.Exit(2)
		}
	case "hook":
		_, _ = io.Copy(io.Discard, os.Stdin)
		time.Sleep(6 * time.Second)
	case "busy":
		time.Sleep(30 * time.Second)
	}
	os.Exit(0)
}

func lifecycleSession(t *testing.T, mode string) *claudeSession {
	t.Helper()
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	s, err := newClaudeSession(ctx, dir, bin, []string{"-test.run=^TestClaudeLifecycleHelper$", "--"}, "", "", "", "", "auto", "", "", nil, nil, nil, []string{"CC_CLAUDE_LIFECYCLE_HELPER=" + mode}, "", false, core.SpawnOptions{}, 0, dir)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = processgroup.Kill(s.cmd)
		cancel()
		select {
		case <-s.done:
		case <-time.After(3 * time.Second):
			t.Error("helper did not exit during cleanup")
		}
	})
	return s
}

func TestClaudeSession_WaitBoundsInheritedStderr(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows descendant handles require native Windows runner")
	}
	s := lifecycleSession(t, "inherit-stderr")
	select {
	case <-s.done:
	case <-time.After(6 * time.Second):
		t.Fatal("exited CLI still waits on descendant stderr; teardown is unbounded")
	}
}

func TestClaudeSession_UserStopUsesShortGraceAndNormalClosePreservesHooks(t *testing.T) {
	t.Run("normal hook", func(t *testing.T) {
		s := lifecycleSession(t, "hook")
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
		if s.cmd.ProcessState.ExitCode() != 0 {
			t.Fatal("normal cleanup killed a still-running Stop hook")
		}
	})
	t.Run("explicit stop", func(t *testing.T) {
		s := lifecycleSession(t, "busy")
		closer, ok := any(s).(interface{ CloseForStop() error })
		if !ok {
			t.Fatal("user stop has no bounded close policy separate from Stop-hook cleanup")
		}
		start := time.Now()
		if err := closer.CloseForStop(); err != nil {
			t.Fatal(err)
		}
		if elapsed := time.Since(start); elapsed > 8*time.Second {
			t.Fatalf("user stop took %v", elapsed)
		}
	})
}

func TestClaudeSession_StopUnblocksSendWhenCLIStopsReadingStdin(t *testing.T) {
	s := lifecycleSession(t, "busy")
	s.gracefulStopTimeout = 50 * time.Millisecond
	sendDone := make(chan error, 1)
	go func() { sendDone <- s.Send(strings.Repeat("x", 2<<20), nil, nil) }()
	deadline := time.Now().Add(2 * time.Second)
	for s.stdinMu.TryLock() {
		s.stdinMu.Unlock()
		if time.Now().After(deadline) {
			t.Fatal("Send did not start writing to the CLI")
		}
		time.Sleep(10 * time.Millisecond)
	}
	select {
	case err := <-sendDone:
		t.Fatalf("Send unexpectedly completed before stopping the non-reading CLI: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	closeDone := make(chan error, 1)
	go func() { closeDone <- s.CloseForStop() }()
	select {
	case err := <-closeDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Stop waited on the blocked Send lock instead of closing stdin")
	}
	select {
	case err := <-sendDone:
		if err == nil {
			t.Fatal("interrupted Send should report a closed pipe")
		}
	case <-time.After(time.Second):
		t.Fatal("Send remained blocked after Stop")
	}
}
