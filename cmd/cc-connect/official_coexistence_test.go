package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/timmyagentic/cc-connect-next/config"
)

const officialPlistRunAtLoad = `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict>
  <key>Label</key><string>com.cc-connect.service</string>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><dict><key>SuccessfulExit</key><false/></dict>
</dict></plist>`

func writeOfficialFixture(t *testing.T, home, configTOML, plist string) {
	t.Helper()
	if configTOML != "" {
		dir := filepath.Join(home, ".cc-connect")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(configTOML), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if plist != "" {
		dir := filepath.Join(home, "Library", "LaunchAgents")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, officialServiceLabel+".plist"), []byte(plist), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func testProbe(home string) officialProbe {
	return officialProbe{
		Home:     home,
		GOOS:     "darwin",
		UID:      501,
		LookPath: func(string) (string, error) { return "", fmt.Errorf("not found") },
		RunCommand: func(name string, args ...string) (string, error) {
			return "", fmt.Errorf("no command runner in test")
		},
		DialTimeout: func(network, address string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout(network, address, timeout)
		},
		Sleep: func(time.Duration) {},
	}
}

func TestProbeOfficialInstall_ArmedAutostartAndCredentials(t *testing.T) {
	home := t.TempDir()
	writeOfficialFixture(t, home, `
[[projects]]
[[projects.platforms]]
type = "feishu"
[projects.platforms.options]
app_id = "cli_officialapp001"
app_secret = "not-used-by-the-probe"
`, officialPlistRunAtLoad)

	st := probeOfficialInstall(testProbe(home))
	if !st.ServiceRegistered {
		t.Fatal("ServiceRegistered = false, want true")
	}
	// launchctl unavailable in the test probe → cannot prove disabled →
	// conservatively armed.
	if !st.AutostartArmed {
		t.Fatal("AutostartArmed = false, want true (RunAtLoad plist, disabled state unknown)")
	}
	if st.Running {
		t.Fatal("Running = true, want false (no socket)")
	}
	if len(st.AppIDs) != 1 || st.AppIDs[0] != "cli_officialapp001" {
		t.Fatalf("AppIDs = %v", st.AppIDs)
	}
}

func TestProbeOfficialInstall_DisabledAutostart(t *testing.T) {
	home := t.TempDir()
	writeOfficialFixture(t, home, "", officialPlistRunAtLoad)

	p := testProbe(home)
	p.RunCommand = func(name string, args ...string) (string, error) {
		if name == "launchctl" && len(args) > 0 && args[0] == "print-disabled" {
			return `disabled services = {\n\t\t"com.cc-connect.service" => disabled\n\t}`, nil
		}
		return "", fmt.Errorf("unexpected command %s %v", name, args)
	}
	st := probeOfficialInstall(p)
	if st.AutostartArmed {
		t.Fatal("AutostartArmed = true, want false (launchctl reports disabled)")
	}
	if !st.ServiceRegistered {
		t.Fatal("ServiceRegistered = false, want true")
	}
}

func TestProbeOfficialInstall_RunningViaSocket(t *testing.T) {
	// Unix socket paths have a ~104-byte limit; build the fixture under a
	// short base instead of t.TempDir().
	base, err := os.MkdirTemp("", "ccx")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(base) })

	runDir := filepath.Join(base, ".cc-connect", "run")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("unix", filepath.Join(runDir, "api.sock"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	st := probeOfficialInstall(testProbe(base))
	if !st.Running {
		t.Fatal("Running = false, want true (live socket)")
	}
	if !strings.Contains(st.RunningVia, "api.sock") {
		t.Fatalf("RunningVia = %q", st.RunningVia)
	}
}

func TestProbeOfficialInstall_StaleSocketIsNotRunning(t *testing.T) {
	base, err := os.MkdirTemp("", "ccy")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(base) })
	runDir := filepath.Join(base, ".cc-connect", "run")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A socket file with no listener behind it: daemon crashed / was killed.
	ln, err := net.Listen("unix", filepath.Join(runDir, "api.sock"))
	if err != nil {
		t.Fatal(err)
	}
	_ = ln.Close() // leaves the path behind on some platforms; recreate to be sure
	_ = os.WriteFile(filepath.Join(runDir, "api.sock"), nil, 0o600)

	if st := probeOfficialInstall(testProbe(base)); st.Running {
		t.Fatal("Running = true for a stale socket file, want false")
	}
}

func TestOfficialCredentialOverlap(t *testing.T) {
	st := officialInstallState{AppIDs: []string{"cli_a", "cli_b"}}
	if got := officialCredentialOverlap(st, []string{"cli_b", "cli_c"}); len(got) != 1 || got[0] != "cli_b" {
		t.Fatalf("overlap = %v, want [cli_b]", got)
	}
	if got := officialCredentialOverlap(st, []string{"cli_x"}); len(got) != 0 {
		t.Fatalf("overlap = %v, want empty", got)
	}
}

func TestCollectConfigAppIDs(t *testing.T) {
	cfg := &config.Config{Projects: []config.ProjectConfig{{
		Platforms: []config.PlatformConfig{
			{Type: "feishu", Options: map[string]any{"app_id": "cli_one"}},
			{Type: "telegram", Options: map[string]any{"token": "irrelevant"}},
			{Type: "feishu", Options: map[string]any{"app_id": "cli_one"}}, // dup
		},
	}}}
	got := collectConfigAppIDs(cfg)
	if len(got) != 1 || got[0] != "cli_one" {
		t.Fatalf("collectConfigAppIDs = %v, want [cli_one]", got)
	}
}

func TestOfficialConflictRefusal(t *testing.T) {
	running := officialInstallState{Running: true, RunningVia: "live API socket /x/api.sock"}
	msg := officialConflictRefusal(running, []string{"cli_a"}, "darwin", 501)
	if msg == "" {
		t.Fatal("want refusal for running daemon with overlap")
	}
	for _, want := range []string{"Refusing to start", "cc-connect-next migrate --switch", "stops and disables official CC Connect", "CC_NEXT_ALLOW_OFFICIAL_CONFLICT"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("refusal missing %q:\n%s", want, msg)
		}
	}
	if strings.Contains(msg, "cli_a") {
		t.Fatal("refusal must not echo credential values")
	}

	if got := officialConflictRefusal(running, nil, "darwin", 501); got != "" {
		t.Fatalf("no overlap must not refuse, got %q", got)
	}
	armed := officialInstallState{AutostartArmed: true}
	if got := officialConflictRefusal(armed, []string{"cli_a"}, "darwin", 501); got != "" {
		t.Fatalf("armed-but-not-running must not refuse, got %q", got)
	}
}

func TestOfficialConflictRefusal_ProbeFailureWithSharedCredentialsFailsClosed(t *testing.T) {
	st := officialInstallState{ProbeErr: errors.New("scheduled task probe failed")}
	msg := officialConflictRefusal(st, []string{"cli_shared"}, "darwin", 501)
	for _, want := range []string{"Refusing to start", "could not verify", "migrate --switch"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("refusal missing %q:\n%s", want, msg)
		}
	}
	if strings.Contains(msg, "cli_shared") {
		t.Fatal("probe-failure refusal leaked credential value")
	}
}

func TestRenderOfficialCoexistenceGuidance(t *testing.T) {
	quiet := officialInstallState{}
	out := renderOfficialCoexistenceGuidance(quiet, nil)
	if !strings.Contains(out, "was not modified or stopped") || strings.Contains(out, "⚠") {
		t.Fatalf("quiet state should keep the single passive line, got:\n%s", out)
	}

	hot := officialInstallState{Running: true, RunningVia: "live API socket /x", AutostartArmed: true, ServicePath: "/p"}
	out = renderOfficialCoexistenceGuidance(hot, []string{"cli_a"})
	for _, want := range []string{
		"RUNNING right now",
		"armed to autostart",
		"credential(s) are shared",
		"migrate --switch",
		"installs and starts cc-connect-next",
		"Advanced side-by-side trials",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("guidance missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "cli_a") {
		t.Fatal("guidance must not echo credential values")
	}
}

func TestRunOfficialSwitchover_FailsClosedWhenStillRunning(t *testing.T) {
	base, err := os.MkdirTemp("", "ccz")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(base) })
	runDir := filepath.Join(base, ".cc-connect", "run")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("unix", filepath.Join(runDir, "api.sock"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() }) // stop commands are fakes; daemon "survives"

	var commands []string
	p := testProbe(base)
	p.RunCommand = func(name string, args ...string) (string, error) {
		commands = append(commands, name+" "+strings.Join(args, " "))
		return "", nil
	}
	st := probeOfficialInstall(p)
	if !st.Running {
		t.Fatal("fixture daemon should be 'running'")
	}

	err = runOfficialSwitchover(p, st, func(string, ...any) bool { return true })
	if err == nil || !strings.Contains(err.Error(), "still running") {
		t.Fatalf("want fail-closed error, got %v (commands: %v)", err, commands)
	}
	joined := strings.Join(commands, "\n")
	if !strings.Contains(joined, "launchctl bootout gui/501/com.cc-connect.service") {
		t.Fatalf("expected native stop attempt, got:\n%s", joined)
	}
}

func TestRunOfficialSwitchover_StopsAndDisarms(t *testing.T) {
	base, err := os.MkdirTemp("", "ccw")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(base) })
	runDir := filepath.Join(base, ".cc-connect", "run")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("unix", filepath.Join(runDir, "api.sock"))
	if err != nil {
		t.Fatal(err)
	}
	writeOfficialFixture(t, base, "", officialPlistRunAtLoad)

	var commands []string
	disabled := false
	p := testProbe(base)
	p.LookPath = func(string) (string, error) { return "/usr/local/bin/cc-connect", nil }
	p.RunCommand = func(name string, args ...string) (string, error) {
		commands = append(commands, name+" "+strings.Join(args, " "))
		if name == "launchctl" && len(args) > 0 && args[0] == "print-disabled" {
			if disabled {
				return `"com.cc-connect.service" => disabled`, nil
			}
			return "", nil
		}
		if name == "launchctl" && len(args) > 0 && args[0] == "disable" {
			disabled = true
		}
		if strings.HasSuffix(name, "cc-connect") && len(args) == 2 && args[0] == "daemon" && args[1] == "stop" {
			_ = ln.Close() // the official CLI stop actually kills the daemon
		}
		return "", nil
	}
	st := probeOfficialInstall(p)

	if err := runOfficialSwitchover(p, st, func(string, ...any) bool { return true }); err != nil {
		t.Fatalf("switchover failed: %v\ncommands: %v", err, commands)
	}
	joined := strings.Join(commands, "\n")
	if !strings.Contains(joined, "/usr/local/bin/cc-connect daemon stop") {
		t.Fatalf("expected official CLI stop first, got:\n%s", joined)
	}
	if !strings.Contains(joined, "launchctl disable gui/501/com.cc-connect.service") {
		t.Fatalf("expected autostart disarm, got:\n%s", joined)
	}
}

func TestRunOfficialSwitchover_StopsRegisteredServiceWithoutLiveSocket(t *testing.T) {
	home := t.TempDir()
	writeOfficialFixture(t, home, "", officialPlistRunAtLoad)
	disabled := false
	var commands []string
	p := testProbe(home)
	p.LookPath = func(string) (string, error) { return "/usr/local/bin/cc-connect", nil }
	p.RunCommand = func(name string, args ...string) (string, error) {
		joined := name + " " + strings.Join(args, " ")
		commands = append(commands, joined)
		if name == "launchctl" && len(args) > 0 && args[0] == "print-disabled" {
			if disabled {
				return `"com.cc-connect.service" => disabled`, nil
			}
			return "", nil
		}
		if name == "launchctl" && len(args) > 0 && args[0] == "disable" {
			disabled = true
		}
		return "", nil
	}
	before := probeOfficialInstall(p)
	if before.Running || !before.ServiceRegistered {
		t.Fatalf("fixture state = %+v, want registered without live socket", before)
	}
	if err := runOfficialSwitchover(p, before, func(string, ...any) bool { return true }); err != nil {
		t.Fatalf("runOfficialSwitchover() error: %v", err)
	}
	if !strings.Contains(strings.Join(commands, "\n"), "/usr/local/bin/cc-connect daemon stop") {
		t.Fatalf("registered service was not stopped:\n%s", strings.Join(commands, "\n"))
	}
}

func TestExtractAppIDs(t *testing.T) {
	ids := extractAppIDs(`
app_id = "cli_a"
  app_id = "cli_b"
# app_id = "cli_commented"  (still matches by regex? no: leading # blocks ^\s*)
other = "app_id = \"cli_not_a_key\""
`)
	got := dedupeSorted(ids)
	if len(got) != 2 || got[0] != "cli_a" || got[1] != "cli_b" {
		t.Fatalf("extractAppIDs = %v, want [cli_a cli_b]", got)
	}
}

func TestExtractAppIDs_ResolvesEnvironmentPlaceholders(t *testing.T) {
	t.Setenv("FEISHU_APP_ID", "cli_from_environment")
	ids := extractAppIDs(`
app_id = "${FEISHU_APP_ID}"
app_id = "cli_literal"
`)
	got := dedupeSorted(ids)
	if len(got) != 2 || got[0] != "cli_from_environment" || got[1] != "cli_literal" {
		t.Fatalf("extractAppIDs = %v, want resolved environment ID plus literal", got)
	}
}

func TestExtractAppIDs_OmitsUnsetEnvironmentPlaceholders(t *testing.T) {
	const envName = "CC_NEXT_TEST_UNSET_OFFICIAL_APP_ID"
	t.Setenv(envName, "")
	ids := extractAppIDs(`app_id = "${` + envName + `}"`)
	if len(ids) != 0 {
		t.Fatalf("extractAppIDs = %v, want unresolved/empty ID omitted", ids)
	}
}

func TestRunOfficialSwitchover_SelectsSystemdManager(t *testing.T) {
	for _, tt := range []struct {
		name        string
		servicePath string
		managerFlag string
	}{
		{name: "user unit", servicePath: "/home/demo/.config/systemd/user/cc-connect.service", managerFlag: "--user"},
		{name: "system unit", servicePath: "/etc/systemd/system/cc-connect.service", managerFlag: "--system"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			var commands []string
			p := testProbe(root)
			p.GOOS = "linux"
			p.RunCommand = func(name string, args ...string) (string, error) {
				commands = append(commands, name+" "+strings.Join(args, " "))
				return "", nil
			}
			st := officialInstallState{
				Running:           true,
				ServiceRegistered: true,
				ServicePath:       tt.servicePath,
			}
			if err := runOfficialSwitchover(p, st, func(string, ...any) bool { return true }); err != nil {
				t.Fatalf("runOfficialSwitchover() error: %v", err)
			}
			joined := strings.Join(commands, "\n")
			for _, action := range []string{"stop", "disable"} {
				want := "systemctl " + tt.managerFlag + " " + action + " cc-connect.service"
				if !strings.Contains(joined, want) {
					t.Fatalf("commands missing %q:\n%s", want, joined)
				}
			}
		})
	}
}

func TestProbeOfficialInstall_RejectsMultipleSystemdRegistrations(t *testing.T) {
	home := t.TempDir()
	userUnit := filepath.Join(home, "user", "cc-connect.service")
	systemUnit := filepath.Join(home, "system", "cc-connect.service")
	for _, unit := range []string{userUnit, systemUnit} {
		if err := os.MkdirAll(filepath.Dir(unit), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(unit, []byte("[Service]\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p := testProbe(home)
	p.GOOS = "linux"
	p.SystemdUnitPaths = []string{userUnit, systemUnit}
	state := probeOfficialInstall(p)
	if state.ProbeErr == nil || !strings.Contains(state.ProbeErr.Error(), "multiple official systemd services") {
		t.Fatalf("state = %+v", state)
	}
}

func TestSystemdServiceEnabled_RecognizesDisabledNonzeroResult(t *testing.T) {
	p := testProbe(t.TempDir())
	p.RunCommand = func(name string, args ...string) (string, error) {
		return "disabled\n", errors.New("exit status 1")
	}
	if systemdServiceEnabled(p, "/home/demo/.config/systemd/user/cc-connect.service") {
		t.Fatal("disabled systemd service reported as enabled")
	}
}

func TestProbeOfficialInstall_WindowsScheduledTask(t *testing.T) {
	home := t.TempDir()
	writeOfficialFixture(t, home, `app_id = "cli_windows"`, "")
	p := testProbe(home)
	p.GOOS = "windows"
	p.RunCommand = func(name string, args ...string) (string, error) {
		if name != "powershell.exe" {
			return "", fmt.Errorf("unexpected command %s", name)
		}
		return "Running", nil
	}

	st := probeOfficialInstall(p)
	if !st.ServiceRegistered || !st.AutostartArmed || !st.Running {
		t.Fatalf("windows official state = %+v, want registered, armed, and running", st)
	}
}

func TestRestoreOfficialInstall_ReenablesAndStartsPreviousState(t *testing.T) {
	var commands []string
	p := testProbe(t.TempDir())
	p.RunCommand = func(name string, args ...string) (string, error) {
		commands = append(commands, name+" "+strings.Join(args, " "))
		return "", nil
	}
	before := officialInstallState{
		BinaryPath:        "/usr/local/bin/cc-connect",
		ServiceRegistered: true,
		ServicePath:       "/Users/demo/Library/LaunchAgents/com.cc-connect.service.plist",
		AutostartArmed:    true,
		Running:           true,
	}

	if err := restoreOfficialInstall(p, before, func(string, ...any) bool { return true }); err != nil {
		t.Fatalf("restoreOfficialInstall() error: %v", err)
	}
	joined := strings.Join(commands, "\n")
	for _, want := range []string{
		"launchctl enable gui/501/com.cc-connect.service",
		"/usr/local/bin/cc-connect daemon start",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("restore commands missing %q:\n%s", want, joined)
		}
	}
}

func TestRestoreOfficialInstall_NativeFallbackCanRecoverAfterCLIStartFailure(t *testing.T) {
	var commands []string
	p := testProbe(t.TempDir())
	p.RunCommand = func(name string, args ...string) (string, error) {
		commands = append(commands, name+" "+strings.Join(args, " "))
		if strings.HasSuffix(name, "cc-connect") {
			return "", errors.New("stale CLI path")
		}
		return "", nil
	}
	before := officialInstallState{
		BinaryPath:  "/stale/cc-connect",
		ServicePath: "/Users/demo/Library/LaunchAgents/com.cc-connect.service.plist",
		Running:     true,
	}
	if err := restoreOfficialInstall(p, before, func(string, ...any) bool { return true }); err != nil {
		t.Fatalf("native fallback recovered service but returned error: %v", err)
	}
	joined := strings.Join(commands, "\n")
	if !strings.Contains(joined, "launchctl bootstrap") || !strings.Contains(joined, "launchctl kickstart") {
		t.Fatalf("native fallback commands missing:\n%s", joined)
	}
}

func TestRunOfficialSwitchover_WindowsStopsAndDisables(t *testing.T) {
	state := "Running"
	var commands []string
	p := testProbe(t.TempDir())
	p.GOOS = "windows"
	p.LookPath = func(string) (string, error) { return "", errors.New("not found") }
	p.RunCommand = func(name string, args ...string) (string, error) {
		joined := name + " " + strings.Join(args, " ")
		commands = append(commands, joined)
		if strings.Contains(joined, "Get-ScheduledTask") {
			return state, nil
		}
		if strings.Contains(joined, "Stop-ScheduledTask") {
			state = "Ready"
		}
		if strings.Contains(joined, "Disable-ScheduledTask") {
			state = "Disabled"
		}
		return "", nil
	}
	before := probeOfficialInstall(p)
	if err := runOfficialSwitchover(p, before, func(string, ...any) bool { return true }); err != nil {
		t.Fatalf("runOfficialSwitchover() error: %v", err)
	}
	if state != "Disabled" {
		t.Fatalf("scheduled task state = %q, want Disabled", state)
	}
	joined := strings.Join(commands, "\n")
	for _, want := range []string{"Stop-ScheduledTask", "Disable-ScheduledTask"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("windows switchover missing %q:\n%s", want, joined)
		}
	}
}

func TestOfficialProbeForMigrationUsesCustomSourcePaths(t *testing.T) {
	base := testProbe("/home/demo")
	got := officialProbeForMigration(base, &preparedMigration{
		SourceRoot:    "/srv/official-config",
		SourceDataDir: "/var/lib/official-state",
	})
	if got.ConfigPath != "/srv/official-config/config.toml" {
		t.Fatalf("ConfigPath = %q", got.ConfigPath)
	}
	if got.SocketPath != "/var/lib/official-state/run/api.sock" {
		t.Fatalf("SocketPath = %q", got.SocketPath)
	}
}

func TestWaitForOfficialRestoredWaitsForRunningAndAutostart(t *testing.T) {
	calls := 0
	p := testProbe(t.TempDir())
	p.GOOS = "windows"
	p.RunCommand = func(name string, args ...string) (string, error) {
		calls++
		if calls < 3 {
			return "Disabled", nil
		}
		return "Running", nil
	}
	before := officialInstallState{ServiceRegistered: true, AutostartArmed: true, Running: true}
	if err := waitForOfficialRestored(p, before); err != nil {
		t.Fatalf("waitForOfficialRestored() error: %v", err)
	}
	if calls < 3 {
		t.Fatalf("probe calls = %d, want retry", calls)
	}
}

func TestRunOfficialSwitchover_PostStopProbeFailureFailsClosed(t *testing.T) {
	probeCalls := 0
	p := testProbe(t.TempDir())
	p.GOOS = "windows"
	p.LookPath = func(string) (string, error) { return "", errors.New("not found") }
	p.RunCommand = func(name string, args ...string) (string, error) {
		joined := name + " " + strings.Join(args, " ")
		if strings.Contains(joined, "Get-ScheduledTask") {
			probeCalls++
			if probeCalls == 1 {
				return "Running", nil
			}
			return "", errors.New("Task Scheduler query failed")
		}
		return "", nil
	}
	before := probeOfficialInstall(p)
	err := runOfficialSwitchover(p, before, func(string, ...any) bool { return true })
	if err == nil || !strings.Contains(err.Error(), "could not verify official daemon quiescence") || !strings.Contains(err.Error(), "Task Scheduler query failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestWaitForOfficialRestored_ProbeFailureFailsClosed(t *testing.T) {
	p := testProbe(t.TempDir())
	p.GOOS = "windows"
	p.RunCommand = func(string, ...string) (string, error) {
		return "", errors.New("Task Scheduler unavailable")
	}
	err := waitForOfficialRestored(p, officialInstallState{ServiceRegistered: true, AutostartArmed: true, Running: true})
	if err == nil || !strings.Contains(err.Error(), "Task Scheduler unavailable") {
		t.Fatalf("error = %v", err)
	}
}
