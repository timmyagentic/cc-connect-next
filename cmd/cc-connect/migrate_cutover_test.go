package main

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/timmyagentic/cc-connect-next/core"
	"github.com/timmyagentic/cc-connect-next/daemon"
)

func migrationRuntimeFixture(t *testing.T, ready *atomic.Bool) (string, func()) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Unix socket fixture")
	}
	dataDir, err := os.MkdirTemp("/tmp", "ccn-runtime-")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "run"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dataDir) })
	listener, err := net.Listen("unix", resolveSocketPath(dataDir))
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		health := core.RuntimeHealth{Ready: ready.Load()}
		if !health.Ready {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		_ = json.NewEncoder(w).Encode(health)
	})}
	go func() { _ = server.Serve(listener) }()
	cleanup := func() {
		_ = server.Close()
		_ = os.RemoveAll(dataDir)
	}
	return dataDir, cleanup
}

type cutoverTestDaemonManager struct {
	events       *[]string
	status       daemon.Status
	installErr   error
	uninstallErr error
}

func (m *cutoverTestDaemonManager) Install(cfg daemon.Config) error {
	*m.events = append(*m.events, "next-install:"+cfg.ConfigPath+":"+cfg.WorkDir)
	if m.installErr != nil {
		return m.installErr
	}
	m.status.Installed = true
	m.status.Running = true
	m.status.PID = 4242
	return nil
}

func (m *cutoverTestDaemonManager) Uninstall() error {
	*m.events = append(*m.events, "next-uninstall")
	if m.uninstallErr != nil {
		return m.uninstallErr
	}
	m.status = daemon.Status{Platform: m.status.Platform}
	return nil
}

func (m *cutoverTestDaemonManager) Start() error {
	*m.events = append(*m.events, "next-start")
	m.status.Running = true
	return nil
}

func (m *cutoverTestDaemonManager) Stop() error {
	*m.events = append(*m.events, "next-stop")
	m.status.Running = false
	return nil
}

func (m *cutoverTestDaemonManager) Restart() error {
	*m.events = append(*m.events, "next-restart")
	m.status.Running = true
	return nil
}

func (m *cutoverTestDaemonManager) Status() (*daemon.Status, error) {
	*m.events = append(*m.events, "next-status")
	copy := m.status
	return &copy, nil
}

func (m *cutoverTestDaemonManager) Platform() string { return m.status.Platform }

func cutoverTestPlan() *preparedMigration {
	return &preparedMigration{
		SourceWorkDir: "/official/runtime",
		Main: &migrationDestination{
			Target: "/next/data",
		},
	}
}

func cutoverTestDeps(t *testing.T, events *[]string, mgr *cutoverTestDaemonManager) migrationCutoverDeps {
	t.Helper()
	return migrationCutoverDeps{
		PrepareMigration: func(opts migrationOptions) (*preparedMigration, error) {
			*events = append(*events, "prepare")
			if !opts.Force {
				t.Fatal("direct cutover preflight must force a final target merge")
			}
			return cutoverTestPlan(), nil
		},
		RunMigration: func(opts migrationOptions) (migrationReport, error) {
			*events = append(*events, "migrate")
			return migrationReport{
				SourceWorkDir: "/official/runtime",
				ManifestPath:  "/next/data/migration-manifest.json",
			}, nil
		},
		CheckNextUnits: func(string) error { return nil },
		NewDaemonManager: func() (daemon.Manager, error) {
			*events = append(*events, "next-manager")
			return mgr, nil
		},
		ResolveDaemonConfig: func(cfg *daemon.Config) error {
			*events = append(*events, "next-resolve")
			cfg.BinaryPath = "/bin/cc-connect-next"
			cfg.LogFile = "/next/data/logs/cc-connect-next.log"
			cfg.LogMaxSize = daemon.DefaultLogMaxSize
			cfg.LogMaxBackups = daemon.DefaultLogMaxBackups
			return nil
		},
		SaveDaemonMeta: func(meta *daemon.Meta) error {
			*events = append(*events, "next-save-meta:"+meta.ConfigPath+":"+meta.WorkDir)
			return nil
		},
		WaitRuntimeReady: func(daemon.Manager, string) (*daemon.Status, core.RuntimeHealth, error) {
			*events = append(*events, "wait-runtime-ready")
			return &daemon.Status{Installed: true, Running: true, PID: 4242, Platform: "test"}, core.RuntimeHealth{Ready: true}, nil
		},
		WaitRuntimeDisarmed: func(daemon.Manager, string) error {
			*events = append(*events, "wait-runtime-disarmed")
			return nil
		},
		ProbeOfficial: func(_ *preparedMigration) officialInstallState {
			*events = append(*events, "official-probe")
			return officialInstallState{ServiceRegistered: true, AutostartArmed: true, Running: true}
		},
		SwitchOfficial: func(_ officialInstallState, _ func(string, ...any) bool) error {
			*events = append(*events, "official-stop-disable")
			return nil
		},
		RestoreOfficial: func(_ officialInstallState, _ func(string, ...any) bool) error {
			*events = append(*events, "official-enable-start")
			return nil
		},
	}
}

func TestRunDirectMigrationCutover_SwitchesAndStartsSuccessor(t *testing.T) {
	var events []string
	mgr := &cutoverTestDaemonManager{
		events: &events,
		status: daemon.Status{Platform: "test"},
	}
	deps := cutoverTestDeps(t, &events, mgr)

	result, err := runDirectMigrationCutover(migrationOptions{}, deps, func(string, ...any) bool { return true })
	if err != nil {
		t.Fatalf("runDirectMigrationCutover() error: %v", err)
	}
	if result.DaemonStatus == nil || !result.DaemonStatus.Running || result.DaemonStatus.PID != 4242 {
		t.Fatalf("daemon status = %+v, want running PID 4242", result.DaemonStatus)
	}
	if !result.RuntimeHealth.Ready {
		t.Fatalf("runtime health = %+v, want ready", result.RuntimeHealth)
	}
	if result.ConfigPath != "/next/data/config.toml" || result.WorkDir != "/official/runtime" {
		t.Fatalf("result paths = config %q work %q", result.ConfigPath, result.WorkDir)
	}

	want := []string{
		"prepare",
		"next-manager",
		"next-status",
		"official-probe",
		"official-stop-disable",
		"migrate",
		"next-resolve",
		"next-install:/next/data/config.toml:/official/runtime",
		"next-save-meta:/next/data/config.toml:/official/runtime",
		"wait-runtime-ready",
	}
	if got := strings.Join(events, "\n"); got != strings.Join(want, "\n") {
		t.Fatalf("cutover order:\n%s\nwant:\n%s", got, strings.Join(want, "\n"))
	}
}

func TestRunDirectMigrationCutover_RejectsExistingSuccessorBeforeOfficialMutation(t *testing.T) {
	var events []string
	mgr := &cutoverTestDaemonManager{
		events: &events,
		status: daemon.Status{Installed: true, Running: true, Platform: "test"},
	}
	deps := cutoverTestDeps(t, &events, mgr)
	_, err := runDirectMigrationCutover(migrationOptions{}, deps, func(string, ...any) bool { return true })
	if err == nil || !strings.Contains(err.Error(), "daemon uninstall") {
		t.Fatalf("error = %v", err)
	}
	joined := strings.Join(events, "\n")
	if strings.Contains(joined, "official-probe") || strings.Contains(joined, "official-stop-disable") {
		t.Fatalf("official service was touched:\n%s", joined)
	}
}

func TestRunDirectMigrationCutover_RejectsOppositeSystemdScopeBeforeOfficialMutation(t *testing.T) {
	var events []string
	mgr := &cutoverTestDaemonManager{events: &events, status: daemon.Status{Platform: "test"}}
	deps := cutoverTestDeps(t, &events, mgr)
	deps.CheckNextUnits = func(string) error {
		events = append(events, "next-registration-check")
		return errors.New("existing system-scope cc-connect-next service")
	}

	_, err := runDirectMigrationCutover(migrationOptions{}, deps, func(string, ...any) bool { return true })
	if err == nil || !strings.Contains(err.Error(), "existing system-scope") {
		t.Fatalf("error = %v", err)
	}
	if got := strings.Join(events, "\n"); got != "prepare\nnext-registration-check" {
		t.Fatalf("cutover continued after registration preflight:\n%s", got)
	}
}

func TestCheckMigrationSuccessorUnitPaths_RejectsEitherSystemdScope(t *testing.T) {
	for _, scope := range []string{"user", "system"} {
		t.Run(scope, func(t *testing.T) {
			root := t.TempDir()
			paths := []string{
				filepath.Join(root, "user", "cc-connect-next.service"),
				filepath.Join(root, "system", "cc-connect-next.service"),
			}
			index := 0
			if scope == "system" {
				index = 1
			}
			if err := os.MkdirAll(filepath.Dir(paths[index]), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(paths[index], []byte("[Unit]\n"), 0o600); err != nil {
				t.Fatal(err)
			}

			err := checkMigrationSuccessorUnitPaths(paths)
			if err == nil || !strings.Contains(err.Error(), paths[index]) || !strings.Contains(err.Error(), "daemon uninstall") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestRunDirectMigrationCutover_MigrationFailureRestoresOriginalServices(t *testing.T) {
	var events []string
	mgr := &cutoverTestDaemonManager{
		events: &events,
		status: daemon.Status{Platform: "test"},
	}
	deps := cutoverTestDeps(t, &events, mgr)
	deps.RunMigration = func(migrationOptions) (migrationReport, error) {
		events = append(events, "migrate-failed")
		return migrationReport{}, errors.New("source changed during final sync")
	}

	_, err := runDirectMigrationCutover(migrationOptions{}, deps, func(string, ...any) bool { return true })
	if err == nil || !strings.Contains(err.Error(), "source changed during final sync") {
		t.Fatalf("error = %v, want final sync failure", err)
	}
	joined := strings.Join(events, "\n")
	for _, want := range []string{"official-stop-disable", "migrate-failed", "official-enable-start"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("recovery events missing %q:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "next-start") {
		t.Fatalf("unexpected pre-existing successor recovery:\n%s", joined)
	}
}

func TestRunDirectMigrationCutover_ActivationFailureDisarmsNextAndRestoresOfficial(t *testing.T) {
	var events []string
	mgr := &cutoverTestDaemonManager{
		events:     &events,
		status:     daemon.Status{Platform: "test"},
		installErr: errors.New("launchd bootstrap failed"),
	}
	deps := cutoverTestDeps(t, &events, mgr)

	_, err := runDirectMigrationCutover(migrationOptions{}, deps, func(string, ...any) bool { return true })
	if err == nil || !strings.Contains(err.Error(), "launchd bootstrap failed") {
		t.Fatalf("error = %v, want successor activation failure", err)
	}
	joined := strings.Join(events, "\n")
	for _, want := range []string{"migrate", "next-install", "next-stop", "next-uninstall", "wait-runtime-disarmed", "official-enable-start"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("activation recovery missing %q:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "next-start") {
		t.Fatalf("old successor must not restart after the target config was replaced:\n%s", joined)
	}
}

func TestRunDirectMigrationCutover_ActivationFailureKeepsOfficialDisabledWhenDisarmProofFails(t *testing.T) {
	var events []string
	mgr := &cutoverTestDaemonManager{
		events:     &events,
		status:     daemon.Status{Platform: "test"},
		installErr: errors.New("launch failed"),
	}
	deps := cutoverTestDeps(t, &events, mgr)
	deps.WaitRuntimeDisarmed = func(daemon.Manager, string) error {
		events = append(events, "disarm-proof-failed")
		return errors.New("config lock remains held")
	}

	_, err := runDirectMigrationCutover(migrationOptions{}, deps, func(string, ...any) bool { return true })
	if err == nil || !strings.Contains(err.Error(), "official CC Connect remains disabled") || !strings.Contains(err.Error(), "config lock remains held") {
		t.Fatalf("error = %v", err)
	}
	if strings.Contains(strings.Join(events, "\n"), "official-enable-start") {
		t.Fatalf("official restored without disarm proof: %v", events)
	}
}

func TestRunDirectMigrationCutover_ReadinessFailureDisarmsNextAndRestoresOfficial(t *testing.T) {
	var events []string
	mgr := &cutoverTestDaemonManager{events: &events, status: daemon.Status{Platform: "test"}}
	deps := cutoverTestDeps(t, &events, mgr)
	deps.WaitRuntimeReady = func(daemon.Manager, string) (*daemon.Status, core.RuntimeHealth, error) {
		events = append(events, "runtime-unavailable")
		return &daemon.Status{Installed: true, Running: true, Platform: "test"}, core.RuntimeHealth{}, errors.New("alpha/feishu unavailable")
	}

	_, err := runDirectMigrationCutover(migrationOptions{}, deps, func(string, ...any) bool { return true })
	if err == nil || !strings.Contains(err.Error(), "alpha/feishu unavailable") {
		t.Fatalf("error = %v", err)
	}
	joined := strings.Join(events, "\n")
	for _, want := range []string{"runtime-unavailable", "next-stop", "next-uninstall", "wait-runtime-disarmed", "official-enable-start"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("readiness recovery missing %q:\n%s", want, joined)
		}
	}
}

func TestRunDirectMigrationCutover_ActivationFailureKeepsOfficialDisabledWhenNextCannotDisarm(t *testing.T) {
	var events []string
	mgr := &cutoverTestDaemonManager{
		events:       &events,
		status:       daemon.Status{Platform: "test"},
		installErr:   errors.New("launch failed"),
		uninstallErr: errors.New("service still armed"),
	}
	deps := cutoverTestDeps(t, &events, mgr)
	_, err := runDirectMigrationCutover(migrationOptions{}, deps, func(string, ...any) bool { return true })
	if err == nil || !strings.Contains(err.Error(), "official CC Connect remains disabled") {
		t.Fatalf("error = %v", err)
	}
	if strings.Contains(strings.Join(events, "\n"), "official-enable-start") {
		t.Fatalf("official was restored before Next disarm: %v", events)
	}
}

func TestRunDirectMigrationCutover_RealMigrationEngineProducesRunnableTarget(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, ".cc-connect")
	target := filepath.Join(root, ".cc-connect-next")
	writeMigrationFixture(t, filepath.Join(source, "config.toml"), `[[projects]]
name = "direct-cutover"
[projects.agent]
type = "codex"
[[projects.platforms]]
type = "feishu"
`)
	writeMigrationFixture(t, filepath.Join(source, "sessions", "session.json"), `{"id":"session-1"}`)

	var events []string
	mgr := &cutoverTestDaemonManager{events: &events, status: daemon.Status{Platform: "test"}}
	deps := cutoverTestDeps(t, &events, mgr)
	deps.PrepareMigration = prepareLegacyMigration
	deps.RunMigration = migrateLegacyDataWithOptions

	result, err := runDirectMigrationCutover(migrationOptions{
		Source:             source,
		Target:             target,
		Home:               root,
		RuntimeWorkDir:     source,
		SourceVersion:      "auto",
		IncludeProjectData: true,
	}, deps, func(string, ...any) bool { return true })
	if err != nil {
		t.Fatalf("runDirectMigrationCutover() error: %v", err)
	}
	canonicalTarget, err := canonicalExistingDirectory(target)
	if err != nil {
		t.Fatal(err)
	}
	if result.ConfigPath != filepath.Join(canonicalTarget, "config.toml") {
		t.Fatalf("ConfigPath = %q, want canonical target %q", result.ConfigPath, canonicalTarget)
	}
	if _, err := os.Stat(filepath.Join(target, "sessions", "session.json")); err != nil {
		t.Fatalf("migrated session missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, migrationManifestFilename)); err != nil {
		t.Fatalf("migration manifest missing: %v", err)
	}
	configBytes, err := os.ReadFile(filepath.Join(target, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(configBytes), filepath.ToSlash(canonicalTarget)) {
		t.Fatalf("migrated config did not point data_dir at target:\n%s", configBytes)
	}
}

func TestRunDirectMigrationCutover_ReportsPrimaryAndRecoveryFailures(t *testing.T) {
	var events []string
	mgr := &cutoverTestDaemonManager{events: &events, status: daemon.Status{Platform: "test"}}
	deps := cutoverTestDeps(t, &events, mgr)
	deps.RunMigration = func(migrationOptions) (migrationReport, error) {
		return migrationReport{}, errors.New("final sync failed")
	}
	deps.RestoreOfficial = func(officialInstallState, func(string, ...any) bool) error {
		return errors.New("official restart failed")
	}

	_, err := runDirectMigrationCutover(migrationOptions{}, deps, func(string, ...any) bool { return true })
	if err == nil {
		t.Fatal("expected cutover and recovery failure")
	}
	for _, want := range []string{"final sync failed", "official restart failed"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("combined error missing %q: %v", want, err)
		}
	}
}

func TestRunDirectMigrationCutover_RespectsDaemonSecretCaptureOptOut(t *testing.T) {
	t.Setenv("CC_DAEMON_NO_CAPTURE_SECRETS", "1")
	var events []string
	mgr := &cutoverTestDaemonManager{events: &events, status: daemon.Status{Platform: "test"}}
	deps := cutoverTestDeps(t, &events, mgr)
	deps.ResolveDaemonConfig = func(cfg *daemon.Config) error {
		if !cfg.NoCaptureSecrets {
			t.Fatal("direct cutover ignored CC_DAEMON_NO_CAPTURE_SECRETS")
		}
		cfg.BinaryPath = "/bin/cc-connect-next"
		return nil
	}

	if _, err := runDirectMigrationCutover(migrationOptions{}, deps, func(string, ...any) bool { return true }); err != nil {
		t.Fatalf("runDirectMigrationCutover() error: %v", err)
	}
}

func TestRunDirectMigrationCutover_OfficialProbeFailureRestoresNextAndDoesNotMutateServices(t *testing.T) {
	var events []string
	mgr := &cutoverTestDaemonManager{
		events: &events,
		status: daemon.Status{Platform: "test"},
	}
	deps := cutoverTestDeps(t, &events, mgr)
	deps.ProbeOfficial = func(*preparedMigration) officialInstallState {
		events = append(events, "official-probe-failed")
		return officialInstallState{ProbeErr: errors.New("Task Scheduler unavailable")}
	}

	_, err := runDirectMigrationCutover(migrationOptions{}, deps, func(string, ...any) bool { return true })
	if err == nil || !strings.Contains(err.Error(), "Task Scheduler unavailable") {
		t.Fatalf("error = %v, want strict official probe failure", err)
	}
	joined := strings.Join(events, "\n")
	if strings.Contains(joined, "next-start") || strings.Contains(joined, "official-stop-disable") || strings.Contains(joined, "migrate") {
		t.Fatalf("cutover mutated services after probe failure:\n%s", joined)
	}
}

func TestWaitForMigrationRuntimeReadyRequiresHealthEndpointReady(t *testing.T) {
	var ready atomic.Bool
	dataDir, cleanup := migrationRuntimeFixture(t, &ready)
	defer cleanup()
	var events []string
	mgr := &cutoverTestDaemonManager{events: &events, status: daemon.Status{Installed: true, Running: true, Platform: "test"}}

	go func() {
		time.Sleep(30 * time.Millisecond)
		ready.Store(true)
	}()
	status, health, err := waitForMigrationRuntimeReadyWithin(mgr, dataDir, time.Second, 10*time.Millisecond)
	if err != nil || status == nil || !status.Running || !health.Ready {
		t.Fatalf("status=%+v health=%+v err=%v", status, health, err)
	}
}

func TestWaitForMigrationRuntimeDisarmedRequiresFreeConfigLock(t *testing.T) {
	var events []string
	mgr := &cutoverTestDaemonManager{events: &events, status: daemon.Status{Platform: "test"}}
	root, err := os.MkdirTemp("/tmp", "ccn-lock-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	configPath := filepath.Join(root, "config.toml")
	if err := os.WriteFile(configPath, []byte("language = \"zh\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	lock, err := AcquireInstanceLock(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := waitForMigrationRuntimeDisarmedWithin(mgr, configPath, 40*time.Millisecond, 5*time.Millisecond); err == nil || !strings.Contains(err.Error(), "another cc-connect-next instance") {
		t.Fatalf("held-lock disarm error = %v", err)
	}
	lock.Release()
	if err := waitForMigrationRuntimeDisarmedWithin(mgr, configPath, time.Second, 5*time.Millisecond); err != nil {
		t.Fatalf("disarm after lock release: %v", err)
	}
}

func TestWaitForMigrationRuntimeDisarmedRejectsAnsweringSocket(t *testing.T) {
	var ready atomic.Bool
	ready.Store(true)
	dataDir, cleanup := migrationRuntimeFixture(t, &ready)
	var events []string
	mgr := &cutoverTestDaemonManager{events: &events, status: daemon.Status{Platform: "test"}}
	configPath := filepath.Join(dataDir, "config.toml")
	if err := os.WriteFile(configPath, []byte("language = \"zh\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := waitForMigrationRuntimeDisarmedWithin(mgr, configPath, 40*time.Millisecond, 5*time.Millisecond); err == nil || !strings.Contains(err.Error(), "runtime API socket still answers") {
		t.Fatalf("answering-socket disarm error = %v", err)
	}
	cleanup()
	if err := waitForMigrationRuntimeDisarmedWithin(mgr, configPath, time.Second, 5*time.Millisecond); err != nil {
		t.Fatalf("disarm after socket close: %v", err)
	}
}
