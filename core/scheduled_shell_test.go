package core

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCronAndTimerShellShareSuccessContract(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture uses POSIX printf")
	}
	tests := []struct {
		name string
		run  func(*Engine, Platform, string) error
	}{
		{"cron", func(e *Engine, p Platform, dir string) error {
			return e.executeCronShell(p, nil, &CronJob{Exec: "printf scheduled-ok", WorkDir: dir})
		}},
		{"timer", func(e *Engine, p Platform, dir string) error {
			return e.executeTimerShell(p, nil, &TimerJob{Exec: "printf scheduled-ok", WorkDir: dir})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			platform := &stubPlatformEngine{n: "test"}
			engine := NewEngine("test", &stubAgent{}, []Platform{platform}, "", LangEnglish)
			t.Cleanup(func() {
				if err := engine.Stop(); err != nil {
					t.Errorf("stop engine: %v", err)
				}
			})
			if err := tt.run(engine, platform, t.TempDir()); err != nil {
				t.Fatalf("scheduled shell error = %v", err)
			}
			sent := strings.Join(platform.getSent(), "\n")
			if !strings.Contains(sent, "✅") || !strings.Contains(sent, "scheduled-ok") {
				t.Fatalf("scheduled shell output = %q", sent)
			}
		})
	}
}

func TestCronAndTimerShellShareTimeoutContract(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture uses POSIX sleep")
	}
	tests := []struct {
		name string
		run  func(*Engine, Platform, string) error
	}{
		{"cron", func(e *Engine, p Platform, dir string) error {
			return e.executeCronShell(p, nil, &CronJob{Exec: "sleep 5", WorkDir: dir})
		}},
		{"timer", func(e *Engine, p Platform, dir string) error {
			return e.executeTimerShell(p, nil, &TimerJob{Exec: "sleep 5", WorkDir: dir})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			platform := &stubPlatformEngine{n: "test"}
			engine := NewEngine("test", &stubAgent{}, []Platform{platform}, "", LangEnglish)
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			engine.ctx = ctx
			if err := tt.run(engine, platform, t.TempDir()); err == nil {
				t.Fatal("scheduled shell timeout error = nil")
			}
			if sent := strings.Join(platform.getSent(), "\n"); !strings.Contains(sent, "timeout") {
				t.Fatalf("scheduled shell timeout output = %q", sent)
			}
		})
	}
}

func TestScheduledShellContextAllowsExplicitNoTimeout(t *testing.T) {
	ctx, cancel := scheduledShellContext(context.Background(), 0)
	defer cancel()
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		t.Fatal("zero scheduled-shell timeout unexpectedly created a deadline")
	}
}
