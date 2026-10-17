package core

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestScheduledShellFailurePreservesSanitizedDiagnostics(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell fixture")
	}
	for _, kind := range []string{"cron", "timer"} {
		t.Run(kind, func(t *testing.T) {
			p := &scheduledShellPlatform{stubPlatformEngine{n: "test"}}
			e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
			t.Cleanup(func() { _ = e.Stop() })
			command := "printf 'app_secret = sk1234567890abcdef\\nTLS handshake timeout\\n' >&2; exit 7"
			var err error
			if kind == "cron" {
				err = e.ExecuteCronJob(&CronJob{ID: "test", SessionKey: "test:scheduled", Exec: command, Mute: true})
			} else {
				err = e.ExecuteTimerJob(&TimerJob{ID: "test", SessionKey: "test:scheduled", Exec: command, Mute: true})
			}
			if err == nil || !strings.Contains(err.Error(), "exit code 7") || !strings.Contains(err.Error(), "TLS handshake timeout") {
				t.Fatalf("missing diagnostic: %v", err)
			}
			if strings.Contains(err.Error(), "sk1234567890abcdef") {
				t.Fatalf("secret leaked: %v", err)
			}
			if len(p.getSent()) != 0 {
				t.Fatal("muted job sent output")
			}
		})
	}
}

func TestScheduledShellInterruptionPreservesContextCause(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell fixture")
	}
	for _, timeout := range []bool{false, true} {
		t.Run(map[bool]string{false: "cancel", true: "timeout"}[timeout], func(t *testing.T) {
			p := &scheduledShellPlatform{stubPlatformEngine{n: "test"}}
			e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
			var ctx context.Context
			var cancel context.CancelFunc
			want := context.Canceled
			if timeout {
				ctx, cancel = context.WithTimeout(context.Background(), 50*time.Millisecond)
				want = context.DeadlineExceeded
			} else {
				ctx, cancel = context.WithCancel(context.Background())
				timer := time.AfterFunc(50*time.Millisecond, cancel)
				defer timer.Stop()
			}
			defer cancel()
			e.ctx = ctx
			err := e.ExecuteCronJob(&CronJob{ID: "test", SessionKey: "test:scheduled", Exec: "exec sleep 5", Mute: true})
			if !errors.Is(err, want) {
				t.Fatalf("error = %v; want cause %v", err, want)
			}
		})
	}
}

func TestScheduledShellDiagnosticBoundsRedactsBeforeTruncation(t *testing.T) {
	secret := strings.Repeat("x", 5000)
	redacted := scheduledShellDiagnostic("app_secret = " + secret + "\nfinal TLS error")
	if strings.Contains(redacted, "xxxxx") {
		t.Fatal("credential suffix survived truncation")
	}
	output := scheduledShellDiagnostic(strings.Repeat("diagnostic line\n", 300) + "final TLS error")
	if len([]rune(output)) > scheduledShellDiagnosticLimit+1 {
		t.Fatalf("unbounded diagnostic: %d", len([]rune(output)))
	}
	if !strings.HasSuffix(output, "final TLS error") {
		t.Fatalf("missing error tail: %q", output)
	}
	if got := scheduledShellFailure(4, " \n").Error(); got != "shell: exit code 4" {
		t.Fatalf("empty output: %q", got)
	}
}

func TestScheduledShellFailurePersistsDiagnosticWithoutUnmuting(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell fixture")
	}
	for _, kind := range []string{"cron", "timer"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			p := &scheduledShellPlatform{stubPlatformEngine{n: "test"}}
			e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
			t.Cleanup(func() { _ = e.Stop() })
			command := "printf 'app_secret = sk1234567890abcdef\\nTLS handshake timeout\\n' >&2; exit 7"
			var lastError string
			if kind == "cron" {
				store, err := NewCronStore(dir)
				if err != nil {
					t.Fatal(err)
				}
				job := &CronJob{ID: "test", Project: "test", SessionKey: "test:scheduled", CronExpr: "* * * * *", Exec: command, Mute: true, Enabled: true}
				if err := store.Add(job); err != nil {
					t.Fatal(err)
				}
				scheduler := NewCronScheduler(store)
				scheduler.RegisterEngine("test", e)
				scheduler.executeJob(job.ID)
				reloaded, err := NewCronStore(dir)
				if err != nil {
					t.Fatal(err)
				}
				lastError = reloaded.Get(job.ID).LastError
			} else {
				store, err := NewTimerStore(dir)
				if err != nil {
					t.Fatal(err)
				}
				job := &TimerJob{ID: "test", Project: "test", SessionKey: "test:scheduled", Exec: command, Mute: true}
				if err := store.Add(job); err != nil {
					t.Fatal(err)
				}
				scheduler := NewTimerScheduler(store)
				scheduler.RegisterEngine("test", e)
				scheduler.executeJob(job.ID)
				reloaded, err := NewTimerStore(dir)
				if err != nil {
					t.Fatal(err)
				}
				lastError = reloaded.Get(job.ID).LastError
			}
			if !strings.Contains(lastError, "TLS handshake timeout") || strings.Contains(lastError, "sk1234567890abcdef") {
				t.Fatalf("persisted diagnostic = %q", lastError)
			}
			if len(p.getSent()) != 0 {
				t.Fatal("muted job sent output")
			}
		})
	}
}
