package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/timmyagentic/cc-connect-next/config"
	"github.com/timmyagentic/cc-connect-next/core"
)

type doctorSteerAgent struct {
	available bool
	detail    string
}

func (a *doctorSteerAgent) Name() string { return "doctor-steer" }
func (a *doctorSteerAgent) StartSession(context.Context, string) (core.AgentSession, error) {
	return nil, nil
}
func (a *doctorSteerAgent) ListSessions(context.Context) ([]core.AgentSessionInfo, error) {
	return nil, nil
}
func (a *doctorSteerAgent) Stop() error { return nil }
func (a *doctorSteerAgent) NativeSteerStatus() (bool, string) {
	return a.available, a.detail
}

func findCheck(t *testing.T, results []core.DoctorCheckResult, name string) core.DoctorCheckResult {
	t.Helper()
	for _, r := range results {
		if r.Name == name {
			return r
		}
	}
	t.Fatalf("check %q missing from %+v", name, results)
	return core.DoctorCheckResult{}
}

func TestPlatformConfigChecks_NeverReportAConnectionItDidNotMake(t *testing.T) {
	proj := config.ProjectConfig{
		Name: "alpha",
		Platforms: []config.PlatformConfig{{
			Type: "feishu",
			Options: map[string]any{
				"app_id":     "cli_real_app_id",
				"app_secret": "real-secret",
			},
		}},
	}

	got := findCheck(t, platformConfigChecks(proj), "Platform (feishu)")
	if got.Status != core.DoctorPass {
		t.Fatalf("valid feishu platform = %+v, want pass", got)
	}
	if got.Detail == "connected" {
		t.Fatalf("doctor claimed a connection it never opened: %+v", got)
	}
	if !strings.Contains(got.Detail, "does not open a connection") {
		t.Fatalf("detail %q does not say the platform was not contacted", got.Detail)
	}
}

func TestPlatformConfigChecks_FailOnMissingCredentials(t *testing.T) {
	proj := config.ProjectConfig{
		Name:      "alpha",
		Platforms: []config.PlatformConfig{{Type: "feishu", Options: map[string]any{}}},
	}

	got := findCheck(t, platformConfigChecks(proj), "Platform (feishu)")
	if got.Status != core.DoctorFail {
		t.Fatalf("feishu without credentials = %+v, want fail", got)
	}
	if !strings.Contains(got.Detail, "app_id") {
		t.Fatalf("detail %q does not name the missing credential", got.Detail)
	}
}

func TestPlatformConfigChecks_FailOnUnknownPlatform(t *testing.T) {
	proj := config.ProjectConfig{
		Name:      "alpha",
		Platforms: []config.PlatformConfig{{Type: "not-a-platform"}},
	}

	got := findCheck(t, platformConfigChecks(proj), "Platform (not-a-platform)")
	if got.Status != core.DoctorFail {
		t.Fatalf("unknown platform = %+v, want fail", got)
	}
}

func TestPlatformConfigChecks_FailWhenNoPlatformIsConfigured(t *testing.T) {
	got := findCheck(t, platformConfigChecks(config.ProjectConfig{Name: "alpha"}), "Platforms")
	if got.Status != core.DoctorFail {
		t.Fatalf("project without platforms = %+v, want fail", got)
	}
}

func TestProjectConfigChecks_FailOnStarterPlaceholdersAndSayHowToFixThem(t *testing.T) {
	cfg, path := loadStarterConfig(t)

	got := findCheck(t, projectConfigChecks(cfg, cfg.Projects[0], path), "Config File")
	if got.Status != core.DoctorFail {
		t.Fatalf("starter config = %+v, want fail", got)
	}
	for _, want := range []string{path, "app_id", "cc-connect-next feishu setup"} {
		if !strings.Contains(got.Detail, want) {
			t.Fatalf("detail is missing %q:\n%s", want, got.Detail)
		}
	}
}

func TestProjectConfigChecks_PassOnAnEditedConfig(t *testing.T) {
	workDir := t.TempDir()
	doc := strings.NewReplacer(
		config.PlaceholderWorkDir, workDir,
		config.PlaceholderFeishuAppID, "cli_real_app_id",
		config.PlaceholderFeishuAppSecret, "real-secret",
	).Replace(config.StarterConfigTOML())
	cfg := configFromTOML(t, doc)

	results := projectConfigChecks(cfg, cfg.Projects[0], "config.toml")
	if got := findCheck(t, results, "Config File"); got.Status != core.DoctorPass {
		t.Fatalf("edited config = %+v, want pass", got)
	}
	if got := findCheck(t, results, "Work Directory"); got.Status != core.DoctorPass || got.Detail != workDir {
		t.Fatalf("work directory check = %+v, want pass for %s", got, workDir)
	}
}

func TestProjectConfigChecks_FailOnAMissingWorkDir(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "gone")
	doc := strings.NewReplacer(
		config.PlaceholderWorkDir, missing,
		config.PlaceholderFeishuAppID, "cli_real_app_id",
		config.PlaceholderFeishuAppSecret, "real-secret",
	).Replace(config.StarterConfigTOML())
	cfg := configFromTOML(t, doc)

	got := findCheck(t, projectConfigChecks(cfg, cfg.Projects[0], "config.toml"), "Work Directory")
	if got.Status != core.DoctorFail {
		t.Fatalf("missing work_dir = %+v, want fail", got)
	}
	if !strings.Contains(got.Detail, missing) {
		t.Fatalf("detail %q does not name the directory", got.Detail)
	}
}

func TestDoctorHasFailure(t *testing.T) {
	if doctorHasFailure([]core.DoctorCheckResult{{Status: core.DoctorPass}, {Status: core.DoctorWarn}}) {
		t.Fatal("warnings must not fail the command")
	}
	if !doctorHasFailure([]core.DoctorCheckResult{{Status: core.DoctorPass}, {Status: core.DoctorFail}}) {
		t.Fatal("a failed check must fail the command")
	}
}

func TestBusyMessageSteerCheck_WarnsWhenRequestedBackendCannotSteer(t *testing.T) {
	cfg := &config.Config{}
	proj := config.ProjectConfig{BusyMessageMode: config.BusyMessageModeSteer}
	agent := &doctorSteerAgent{
		detail: `Codex exec backend cannot steer; set backend = "app_server" or busy_message_mode = "queue"`,
	}

	got := busyMessageSteerCheck(cfg, proj, agent)
	if got.Status != core.DoctorWarn {
		t.Fatalf("steer with incapable backend = %+v, want warning", got)
	}
	for _, want := range []string{"cannot steer", `backend = "app_server"`, `busy_message_mode = "queue"`} {
		if !strings.Contains(got.Detail, want) {
			t.Fatalf("detail %q does not contain %q", got.Detail, want)
		}
	}
}

func TestBusyMessageSteerCheck_PassesForNativeBackendOrExplicitQueue(t *testing.T) {
	cfg := &config.Config{}
	capable := &doctorSteerAgent{available: true, detail: "native turn/steer is available"}

	steer := busyMessageSteerCheck(cfg, config.ProjectConfig{}, capable)
	if steer.Status != core.DoctorPass || !strings.Contains(steer.Detail, "available") {
		t.Fatalf("default steer with capable backend = %+v, want pass", steer)
	}

	queue := busyMessageSteerCheck(cfg, config.ProjectConfig{BusyMessageMode: config.BusyMessageModeQueue}, &doctorSteerAgent{})
	if queue.Status != core.DoctorPass || !strings.Contains(queue.Detail, "FIFO queue") {
		t.Fatalf("explicit queue with incapable backend = %+v, want pass", queue)
	}
}

func TestRunDoctorHealthCheck_ReportsAMissingConfig(t *testing.T) {
	if code := runDoctorHealthCheck([]string{"--config", filepath.Join(t.TempDir(), "absent.toml")}); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}

func TestRunDoctorHealthCheck_ReportsAnUnknownProject(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := bootstrapConfig(path); err != nil {
		t.Fatalf("bootstrapConfig() error = %v", err)
	}
	if code := runDoctorHealthCheck([]string{"--config", path, "--project", "no-such-project"}); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}
