package core

import (
	"strings"
	"testing"
	"time"
)

func TestCronExprToHuman_TimezonePrefix(t *testing.T) {
	for _, prefix := range []string{"CRON_TZ=", "TZ="} {
		if got := CronExprToHuman(prefix+"America/New_York 0 15 * * *", LangChinese); got != "每天 15:00" {
			t.Errorf("human schedule = %q, want 每天 15:00", got)
		}
	}
}

func TestCronDisplay_UsesScheduleTimezone(t *testing.T) {
	store, err := NewCronStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	expr := "CRON_TZ=America/New_York 0 15 * * *"
	last, _ := time.Parse(time.RFC3339, "2026-09-19T19:00:00Z")
	if err := store.Add(&CronJob{ID: "display", Project: "test", SessionKey: "test:ch1", CronExpr: expr, Prompt: "task", Enabled: true, LastRun: last}); err != nil {
		t.Fatal(err)
	}
	e := NewEngine("test", &stubAgent{}, nil, "", LangChinese)
	e.cronScheduler = NewCronScheduler(store)
	defer e.cronScheduler.Stop()
	if err := e.cronScheduler.Start(); err != nil {
		t.Fatal(err)
	}
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for e.cronScheduler.NextRun("display").IsZero() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	next := e.cronScheduler.NextRun("display").In(loc)
	if next.IsZero() || next.Hour() != 15 {
		t.Fatalf("next run = %v", next)
	}
	wantNext := "下次执行: " + next.Format(cronTimeFormat(next, time.Now().In(loc)))
	text := e.renderCronCard("test:ch1", "").RenderText()
	for _, want := range []string{"每天 15:00 (America/New_York)", "09-19 15:00", wantNext} {
		if !strings.Contains(text, want) {
			t.Errorf("card missing %q:\n%s", want, text)
		}
	}
	p := &stubPlatformEngine{n: "test"}
	e.cmdCronList(p, &Message{SessionKey: "test:ch1"})
	plain := strings.Join(p.getSent(), "\n")
	for _, want := range []string{"每天 15:00 (America/New_York)", "09-19 15:00", wantNext} {
		if !strings.Contains(plain, want) {
			t.Errorf("list missing %q:\n%s", want, plain)
		}
	}
}
