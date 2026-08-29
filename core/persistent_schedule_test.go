package core

import (
	"strings"
	"testing"
	"time"
)

func newPersistentScheduleBridgeEngine(t *testing.T) (*Engine, *BridgePlatform) {
	t.Helper()
	bs := NewBridgeServerInsecure(0, "", "/bridge/ws", nil)
	if bs == nil {
		t.Fatal("NewBridgeServerInsecure returned nil")
	}
	bp := bs.NewPlatform("test-project")
	engine := NewEngine("test-project", &stubAgent{}, []Platform{bp}, "", LangEnglish)
	bs.RegisterEngine("test-project", engine, bp)
	t.Cleanup(func() { _ = engine.Stop() })
	return engine, bp
}

func TestCronSchedulerRejectsPersistentBridgeTargetOnAdd(t *testing.T) {
	engine, _ := newPersistentScheduleBridgeEngine(t)
	store, err := NewCronStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	scheduler := NewCronScheduler(store)
	scheduler.RegisterEngine("test-project", engine)

	err = scheduler.AddJob(&CronJob{
		ID: "bridge-cron", Project: "test-project",
		SessionKey: "bridge:web-admin:test-project", CronExpr: "0 9 * * *",
		Prompt: "run later", Enabled: true, CreatedAt: time.Now(),
	})
	if err == nil || !strings.Contains(err.Error(), "persistent") {
		t.Fatalf("AddJob() error = %v, want persistent-delivery rejection", err)
	}
	if got := store.List(); len(got) != 0 {
		t.Fatalf("rejected cron was persisted: %#v", got)
	}
}

func TestTimerSchedulerRejectsPersistentBridgeTargetOnAdd(t *testing.T) {
	engine, _ := newPersistentScheduleBridgeEngine(t)
	store, err := NewTimerStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	scheduler := NewTimerScheduler(store)
	scheduler.RegisterEngine("test-project", engine)

	err = scheduler.AddJob(&TimerJob{
		ID: "bridge-timer", Project: "test-project",
		SessionKey:  "bridge:web-admin:test-project",
		ScheduledAt: time.Now().Add(time.Hour), Prompt: "run later", CreatedAt: time.Now(),
	})
	if err == nil || !strings.Contains(err.Error(), "persistent") {
		t.Fatalf("AddJob() error = %v, want persistent-delivery rejection", err)
	}
	if got := store.List(); len(got) != 0 {
		t.Fatalf("rejected timer was persisted: %#v", got)
	}
}

func TestCronSchedulerAllowsExplicitlyDurableBridgeAdapter(t *testing.T) {
	bs, wsURL := startTestBridge(t, "")
	bp := bs.NewPlatform("test-project")
	agentSession := newResultAgentSession("scheduled task complete")
	engine := NewEngine("test-project", &resultAgent{session: agentSession}, []Platform{bp}, "", LangEnglish)
	bs.RegisterEngine("test-project", engine, bp)
	t.Cleanup(func() { _ = engine.Stop() })
	conn := dialWS(t, wsURL, nil)
	register(t, conn, "mychat", []string{"text", "reconstruct_reply", "persistent_scheduled_delivery"})

	store, err := NewCronStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	scheduler := NewCronScheduler(store)
	scheduler.RegisterEngine("test-project", engine)
	job := &CronJob{
		ID: "durable-bridge", Project: "test-project",
		SessionKey: "mychat:room:user", CronExpr: "0 9 * * *",
		Prompt: "run later", Enabled: false, CreatedAt: time.Now(),
	}
	if err := scheduler.AddJob(job); err != nil {
		t.Fatalf("AddJob() rejected explicitly durable Bridge adapter: %v", err)
	}
	if got := store.List(); len(got) != 1 {
		t.Fatalf("stored durable Bridge jobs = %d, want 1", len(got))
	}
	if err := engine.ExecuteCronJob(job); err != nil {
		t.Fatalf("ExecuteCronJob() rejected durable Bridge route: %v", err)
	}
	if len(agentSession.sentPrompts) != 1 {
		t.Fatalf("Agent prompts = %d, want 1", len(agentSession.sentPrompts))
	}
}

func TestCronSchedulerRejectsPersistentBridgeTargetOnUpdateAndRun(t *testing.T) {
	engine, _ := newPersistentScheduleBridgeEngine(t)
	durable := &stubCronReplyTargetPlatform{stubPlatformEngine: stubPlatformEngine{n: "discord"}}
	engine.AddPlatform(durable)
	store, err := NewCronStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	scheduler := NewCronScheduler(store)
	scheduler.RegisterEngine("test-project", engine)

	job := &CronJob{
		ID: "existing", Project: "test-project",
		SessionKey: "discord:channel:user", CronExpr: "0 9 * * *",
		Prompt: "run later", Enabled: false, CreatedAt: time.Now(),
	}
	if err := scheduler.AddJob(job); err != nil {
		t.Fatalf("AddJob(durable) error = %v", err)
	}
	if err := scheduler.UpdateJob(job.ID, "session_key", "bridge:web-admin:test-project"); err == nil {
		t.Fatal("UpdateJob() accepted a non-persistent Bridge target")
	}
	if got := store.Get(job.ID).SessionKey; got != "discord:channel:user" {
		t.Fatalf("failed update mutated session_key to %q", got)
	}

	legacy := &CronJob{
		ID: "legacy-bridge", Project: "test-project",
		SessionKey: "bridge:web-admin:test-project", CronExpr: "0 9 * * *",
		Prompt: "legacy", Enabled: false, CreatedAt: time.Now(),
	}
	if err := store.Add(legacy); err != nil {
		t.Fatal(err)
	}
	if err := scheduler.RunJobNow(legacy.ID); err == nil {
		t.Fatal("RunJobNow() accepted a non-persistent Bridge target")
	}
	if err := scheduler.EnableJob(legacy.ID); err == nil {
		t.Fatal("EnableJob() accepted a non-persistent Bridge target")
	}
	if got := store.Get(legacy.ID).Enabled; got {
		t.Fatal("failed enable mutated the legacy Bridge job")
	}

	enabledLegacy := &CronJob{
		ID: "enabled-legacy-bridge", Project: "test-project",
		SessionKey: "bridge:web-admin:test-project", CronExpr: "0 9 * * *",
		Prompt: "legacy", Enabled: true, CreatedAt: time.Now(),
	}
	if err := store.Add(enabledLegacy); err != nil {
		t.Fatal(err)
	}
	if err := scheduler.UpdateJob(enabledLegacy.ID, "cron_expr", "5 9 * * *"); err == nil {
		t.Fatal("UpdateJob() rescheduled an enabled non-persistent Bridge target")
	}
	if got := store.Get(enabledLegacy.ID).CronExpr; got != "0 9 * * *" {
		t.Fatalf("failed update mutated cron_expr to %q", got)
	}
}
