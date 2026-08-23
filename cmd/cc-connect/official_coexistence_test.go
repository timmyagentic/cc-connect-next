package main

import (
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
		Home: home,
		GOOS: "darwin",
		UID:  501,
		LookPath: func(string) (string, error) { return "", fmt.Errorf("not found") },
		RunCommand: func(name string, args ...string) (string, error) {
			return "", fmt.Errorf("no command runner in test")
		},
		DialTimeout: func(network, address string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout(network, address, timeout)
		},
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
	for _, want := range []string{"Refusing to start", "cc-connect daemon stop", "launchctl disable gui/501/com.cc-connect.service", "CC_NEXT_ALLOW_OFFICIAL_CONFLICT"} {
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

func TestRenderOfficialCoexistenceGuidance(t *testing.T) {
	quiet := officialInstallState{}
	out := renderOfficialCoexistenceGuidance(quiet, nil, "darwin", 501, "/t/config.toml", "/w")
	if !strings.Contains(out, "was not modified or stopped") || strings.Contains(out, "⚠") {
		t.Fatalf("quiet state should keep the single passive line, got:\n%s", out)
	}

	hot := officialInstallState{Running: true, RunningVia: "live API socket /x", AutostartArmed: true, ServicePath: "/p"}
	out = renderOfficialCoexistenceGuidance(hot, []string{"cli_a"}, "darwin", 501, "/t/config.toml", "/w")
	for _, want := range []string{
		"RUNNING right now",
		"armed to autostart",
		"credential(s) are shared",
		"cc-connect daemon stop",
		"migrate --switch",
		"daemon install --config /t/config.toml --work-dir /w",
		"test-app credentials",
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
	p := testProbe(base)
	p.LookPath = func(string) (string, error) { return "/usr/local/bin/cc-connect", nil }
	p.RunCommand = func(name string, args ...string) (string, error) {
		commands = append(commands, name+" "+strings.Join(args, " "))
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
