package core

import (
	"runtime"
	"strings"
	"testing"
)

func TestScheduledShellMutedTracePasswordIsNotPersisted(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell fixture")
	}
	for _, kind := range []string{"cron", "timer"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			p := &scheduledShellPlatform{stubPlatformEngine{n: "test"}}
			e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
			t.Cleanup(func() { _ = e.Stop() })
			command := "printf '+ deploy --password synthetic-short-pass\\nTLS handshake timeout\\n' >&2; exit 7"
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
			if !strings.Contains(lastError, "[REDACTED-SHELL-OUTPUT]") || strings.Contains(lastError, "synthetic-short-pass") {
				t.Fatalf("persisted diagnostic = %q", lastError)
			}
			if len(p.getSent()) != 0 {
				t.Fatal("muted job sent output")
			}
		})
	}
}
