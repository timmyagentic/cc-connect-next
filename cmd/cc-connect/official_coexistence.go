package main

// Copy-only migration deliberately never modifies the official CC Connect
// installation. The explicit production cutover is different: it stops and
// disables official CC Connect, performs the final sync, starts the successor,
// and restores the original service state if the cutover fails. These probes
// give runtime startup, migrate, and doctor one shared view of that boundary.

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/timmyagentic/cc-connect-next/config"
)

const officialServiceLabel = "com.cc-connect.service"

// officialProbe carries every environment dependency of the detection so
// tests can point the probe at a sandbox.
type officialProbe struct {
	Home             string
	GOOS             string
	UID              int
	ConfigPath       string
	SocketPath       string
	LookPath         func(file string) (string, error)
	RunCommand       func(name string, args ...string) (string, error)
	DialTimeout      func(network, address string, timeout time.Duration) (net.Conn, error)
	Sleep            func(time.Duration)
	SystemdUnitPaths []string // test override; production probes user + system
}

func officialProbeForMigration(probe officialProbe, plan *preparedMigration) officialProbe {
	if plan == nil {
		return probe
	}
	if strings.TrimSpace(plan.SourceRoot) != "" {
		probe.ConfigPath = filepath.Join(plan.SourceRoot, "config.toml")
	}
	if strings.TrimSpace(plan.SourceDataDir) != "" {
		probe.SocketPath = filepath.Join(plan.SourceDataDir, "run", "api.sock")
	}
	return probe
}

func defaultOfficialProbe() officialProbe {
	home, _ := os.UserHomeDir()
	return officialProbe{
		Home:     home,
		GOOS:     runtime.GOOS,
		UID:      os.Getuid(),
		LookPath: exec.LookPath,
		RunCommand: func(name string, args ...string) (string, error) {
			out, err := exec.Command(name, args...).CombinedOutput()
			return string(out), err
		},
		DialTimeout: net.DialTimeout,
		Sleep:       time.Sleep,
	}
}

// officialInstallState is a point-in-time picture of what remains of an
// official CC Connect installation on this machine.
type officialInstallState struct {
	BinaryPath        string // "" when the official CLI is not on PATH
	ServiceRegistered bool   // an autostart artifact (plist / unit) exists
	ServicePath       string
	AutostartArmed    bool   // registered AND will start on next login/boot
	Running           bool   // an official daemon is alive right now
	RunningVia        string // human-readable evidence
	AppIDs            []string
	ProbeErr          error
}

var runAtLoadRe = regexp.MustCompile(`(?s)<key>\s*RunAtLoad\s*</key>\s*<true\s*/>`)

func plistHasRunAtLoad(plist string) bool {
	return runAtLoadRe.MatchString(plist)
}

// probeOfficialInstall inspects the official binary, autostart registration,
// live daemon, and configured platform credentials. Every step is best
// effort: a probe failure degrades to "not detected" for Running (refusals
// must be certain) and to "armed" for autostart (warnings may be cautious).
func probeOfficialInstall(p officialProbe) officialInstallState {
	var st officialInstallState
	if p.LookPath != nil {
		if path, err := p.LookPath("cc-connect"); err == nil {
			st.BinaryPath = path
		}
	}
	if p.Home == "" {
		return st
	}

	switch p.GOOS {
	case "darwin":
		plist := filepath.Join(p.Home, "Library", "LaunchAgents", officialServiceLabel+".plist")
		if data, err := os.ReadFile(plist); err == nil {
			st.ServiceRegistered = true
			st.ServicePath = plist
			st.AutostartArmed = plistHasRunAtLoad(string(data)) && !launchdServiceDisabled(p)
		} else if !errors.Is(err, os.ErrNotExist) {
			st.ProbeErr = fmt.Errorf("inspect official launchd service: %w", err)
		}
	case "linux":
		units := p.SystemdUnitPaths
		if len(units) == 0 {
			units = []string{
				filepath.Join(p.Home, ".config", "systemd", "user", "cc-connect.service"),
				"/etc/systemd/system/cc-connect.service",
			}
		}
		var registered []string
		for _, unit := range units {
			if _, err := os.Stat(unit); err == nil {
				registered = append(registered, unit)
			} else if !errors.Is(err, os.ErrNotExist) {
				st.ProbeErr = errors.Join(st.ProbeErr, fmt.Errorf("inspect official systemd service %s: %w", unit, err))
			}
		}
		if len(registered) > 1 {
			st.ServiceRegistered = true
			st.ServicePath = strings.Join(registered, ", ")
			st.AutostartArmed = true
			st.ProbeErr = errors.Join(st.ProbeErr, fmt.Errorf("multiple official systemd services are registered (%s); disable and remove one before migration", st.ServicePath))
		} else if len(registered) == 1 {
			st.ServiceRegistered = true
			st.ServicePath = registered[0]
			st.AutostartArmed = systemdServiceEnabled(p, registered[0])
		}
	case "windows":
		probeOfficialWindowsTask(p, &st)
	}

	// A live official daemon always serves its API socket; a connectable
	// socket is authoritative, a stale socket file refuses the dial. On
	// Windows the official daemon has no unix socket, so Running stays
	// best-effort false there and the guidance leans on the service state.
	if p.GOOS != "windows" && p.DialTimeout != nil {
		sock := p.SocketPath
		if sock == "" {
			sock = filepath.Join(p.Home, ".cc-connect", "run", "api.sock")
		}
		if conn, err := p.DialTimeout("unix", sock, 400*time.Millisecond); err == nil {
			_ = conn.Close()
			st.Running = true
			st.RunningVia = "live API socket " + sock
		}
	}

	configPath := p.ConfigPath
	if configPath == "" {
		configPath = filepath.Join(p.Home, ".cc-connect", "config.toml")
	}
	st.AppIDs = readOfficialAppIDsFrom(configPath)
	return st
}

func probeOfficialWindowsTask(p officialProbe, st *officialInstallState) {
	if p.RunCommand == nil {
		return
	}
	out, err := p.RunCommand("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", `$ErrorActionPreference = 'Stop'
$task = Get-ScheduledTask -TaskName 'cc-connect' -ErrorAction SilentlyContinue
if ($null -eq $task) { Write-Output 'NotFound'; exit 0 }
Write-Output ([string]$task.State)`)
	if err != nil {
		st.ProbeErr = fmt.Errorf("inspect official Windows scheduled task: %w", err)
		return
	}
	state := strings.TrimSpace(out)
	if state == "" || strings.EqualFold(state, "NotFound") {
		return
	}
	st.ServiceRegistered = true
	st.ServicePath = "Task Scheduler: cc-connect"
	st.Running = strings.EqualFold(state, "Running")
	st.AutostartArmed = !strings.EqualFold(state, "Disabled")
	if st.Running {
		st.RunningVia = "Windows scheduled task cc-connect"
	}
}

func launchdServiceDisabled(p officialProbe) bool {
	if p.RunCommand == nil {
		return false
	}
	out, err := p.RunCommand("launchctl", "print-disabled", fmt.Sprintf("gui/%d", p.UID))
	if err != nil {
		return false // cannot tell → treat as armed; drives warnings only
	}
	return regexp.MustCompile(`"` + regexp.QuoteMeta(officialServiceLabel) + `"\s*=>\s*(disabled|true)`).MatchString(out)
}

func systemdServiceEnabled(p officialProbe, unit string) bool {
	if p.RunCommand == nil {
		return true
	}
	args := officialSystemctlArgs(unit, "is-enabled")
	out, err := p.RunCommand("systemctl", args...)
	state := strings.ToLower(strings.TrimSpace(out))
	switch state {
	case "disabled", "masked", "static", "indirect", "generated", "transient":
		return false
	}
	if err != nil {
		return true // cannot tell → treat as armed; drives warnings only
	}
	return strings.HasPrefix(state, "enabled")
}

var appIDRe = regexp.MustCompile(`(?m)^\s*app_id\s*=\s*"([^"]+)"`)
var officialEnvPlaceholderRe = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

func readOfficialAppIDsFrom(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return dedupeSorted(extractAppIDs(string(data)))
}

func extractAppIDs(configTOML string) []string {
	var ids []string
	for _, m := range appIDRe.FindAllStringSubmatch(configTOML, -1) {
		if resolved, ok := resolveOfficialConfigEnv(m[1]); ok {
			ids = append(ids, resolved)
		}
	}
	return ids
}

func resolveOfficialConfigEnv(raw string) (string, bool) {
	missing := false
	resolved := officialEnvPlaceholderRe.ReplaceAllStringFunc(raw, func(token string) string {
		match := officialEnvPlaceholderRe.FindStringSubmatch(token)
		value, ok := os.LookupEnv(match[1])
		if !ok || value == "" {
			missing = true
			return ""
		}
		return value
	})
	resolved = strings.TrimSpace(resolved)
	return resolved, !missing && resolved != ""
}

func officialSystemctlArgs(servicePath, action string) []string {
	manager := "--system"
	if strings.Contains(servicePath, "/.config/systemd/user/") {
		manager = "--user"
	}
	return []string{manager, action, "cc-connect.service"}
}

// extractAppIDsFromConfigFile reads credential IDs from a config file on
// disk (used post-migration, where the migrated config is not loaded).
func extractAppIDsFromConfigFile(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return dedupeSorted(extractAppIDs(string(data)))
}

// collectConfigAppIDs collects credential IDs from the loaded next config.
func collectConfigAppIDs(cfg *config.Config) []string {
	var ids []string
	for _, proj := range cfg.Projects {
		for _, pc := range proj.Platforms {
			if id, ok := pc.Options["app_id"].(string); ok && id != "" {
				ids = append(ids, id)
			}
		}
	}
	return dedupeSorted(ids)
}

func dedupeSorted(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	var out []string
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// officialCredentialOverlap returns the credential IDs present in both the
// official install and the given next config.
func officialCredentialOverlap(st officialInstallState, ourIDs []string) []string {
	official := make(map[string]struct{}, len(st.AppIDs))
	for _, id := range st.AppIDs {
		official[id] = struct{}{}
	}
	var overlap []string
	for _, id := range ourIDs {
		if _, ok := official[id]; ok {
			overlap = append(overlap, id)
		}
	}
	sort.Strings(overlap)
	return overlap
}

func redactCredentialID(id string) string {
	if len(id) <= 8 {
		return id[:min(len(id), 4)] + "…"
	}
	return id[:8] + "…"
}

func disarmOfficialHint(goos string, uid int) string {
	switch goos {
	case "darwin":
		return fmt.Sprintf("  launchctl disable gui/%d/%s", uid, officialServiceLabel)
	case "linux":
		return "  systemctl --user disable cc-connect.service   # or: sudo systemctl disable cc-connect.service"
	case "windows":
		return "  powershell.exe -NoProfile -Command \"Disable-ScheduledTask -TaskName 'cc-connect'\""
	default:
		return "  disable the official CC Connect scheduled task / service in your service manager"
	}
}

// officialConflictRefusal returns a non-empty startup refusal when an
// official daemon is running right now with credentials this config also
// uses. Two consumers on one credential race for the same events; refusing
// to start is strictly better than duplicating half the replies.
func officialConflictRefusal(st officialInstallState, overlap []string, _ string, _ int) string {
	if len(overlap) == 0 {
		return ""
	}
	if st.ProbeErr != nil {
		return fmt.Sprintf(`Refusing to start: cc-connect-next could not verify the official CC Connect service state (%v).
The configurations share %d platform credential(s), so starting could create two consumers.

Resolve the service probe, then run the direct production cutover:
  cc-connect-next migrate --switch

Set CC_NEXT_ALLOW_OFFICIAL_CONFLICT=1 to start anyway (not recommended).
`, st.ProbeErr, len(overlap))
	}
	if !st.Running {
		return ""
	}
	return fmt.Sprintf(`Refusing to start: the official CC Connect daemon is running (%s)
and shares %d platform credential(s) with this configuration.
Two daemons consuming the same credentials race for the same events and
produce duplicate or lost replies.

Run the direct production cutover, which stops and disables official CC Connect,
performs the final sync, and starts cc-connect-next:
  cc-connect-next migrate --switch

Advanced side-by-side trials still require separate app credentials.

Set CC_NEXT_ALLOW_OFFICIAL_CONFLICT=1 to start anyway (not recommended).
`, st.RunningVia, len(overlap))
}

// renderOfficialCoexistenceGuidance is the post-migration report block that
// replaces the old passive "was not modified or stopped" single line
// whenever there is anything actionable to say.
func renderOfficialCoexistenceGuidance(st officialInstallState, overlap []string) string {
	var b strings.Builder
	b.WriteString("The official CC Connect installation was not modified or stopped.\n")

	if !st.Running && !st.AutostartArmed {
		if st.ServiceRegistered {
			fmt.Fprintf(&b, "Official daemon: not running; autostart already disabled (%s).\n", st.ServicePath)
		}
		return b.String()
	}

	if st.Running {
		fmt.Fprintf(&b, "⚠ The official daemon is RUNNING right now (%s).\n", st.RunningVia)
	}
	if st.AutostartArmed {
		fmt.Fprintf(&b, "⚠ The official daemon is armed to autostart on next login/boot (%s).\n", st.ServicePath)
	}
	if len(overlap) > 0 {
		fmt.Fprintf(&b, "⚠ %d platform credential(s) are shared between the official config and the migrated one — running both daemons duplicates message handling.\n", len(overlap))
	}

	b.WriteString("\nTo switch production traffic to cc-connect-next:\n")
	b.WriteString("  cc-connect-next migrate --switch\n")
	b.WriteString("This stops any existing successor, stops and disables official CC Connect, performs the final sync, then installs and starts cc-connect-next.\n")
	if len(overlap) > 0 {
		b.WriteString("Advanced side-by-side trials remain possible with separate app credentials.\n")
	}
	return b.String()
}

// printOfficialCoexistenceSection renders the doctor section describing the
// official install. Returns true when the state is an active failure (an
// official daemon running with shared credentials).
func printOfficialCoexistenceSection(w io.Writer, cfg *config.Config) bool {
	probe := defaultOfficialProbe()
	st := probeOfficialInstall(probe)
	if st.BinaryPath == "" && !st.ServiceRegistered && !st.Running && len(st.AppIDs) == 0 && st.ProbeErr == nil {
		return false // no official install detected; keep doctor output quiet
	}

	_, _ = fmt.Fprintf(w, "\n=== official CC Connect coexistence ===\n")
	probeFailed := st.ProbeErr != nil
	if probeFailed {
		_, _ = fmt.Fprintf(w, "❌ service probe: %v — cannot prove direct cutover or coexistence safety\n", st.ProbeErr)
	}
	if st.BinaryPath != "" {
		_, _ = fmt.Fprintf(w, "✅ binary: %s\n", st.BinaryPath)
	} else {
		_, _ = fmt.Fprintf(w, "✅ binary: not on PATH\n")
	}
	switch {
	case !st.ServiceRegistered:
		_, _ = fmt.Fprintf(w, "✅ autostart: not registered\n")
	case st.AutostartArmed:
		_, _ = fmt.Fprintf(w, "⚠️ autostart: ARMED (%s) — next login/boot starts the official daemon\n   disarm: %s\n", st.ServicePath, strings.TrimSpace(disarmOfficialHint(probe.GOOS, probe.UID)))
	default:
		_, _ = fmt.Fprintf(w, "✅ autostart: registered but disabled (%s)\n", st.ServicePath)
	}
	if st.Running {
		_, _ = fmt.Fprintf(w, "⚠️ daemon: RUNNING (%s)\n", st.RunningVia)
	} else {
		_, _ = fmt.Fprintf(w, "✅ daemon: not running\n")
	}

	overlap := officialCredentialOverlap(st, collectConfigAppIDs(cfg))
	if len(overlap) == 0 {
		_, _ = fmt.Fprintf(w, "✅ shared credentials: none\n")
		return probeFailed
	}
	redacted := make([]string, len(overlap))
	for i, id := range overlap {
		redacted[i] = redactCredentialID(id)
	}
	if st.Running {
		_, _ = fmt.Fprintf(w, "❌ shared credentials: %d (%s) — both daemons consume the same event stream; run `cc-connect-next migrate --switch` for direct cutover (separate credentials are only for an advanced parallel trial)\n", len(overlap), strings.Join(redacted, ", "))
		return true
	}
	_, _ = fmt.Fprintf(w, "⚠️ shared credentials: %d (%s) — safe only while the official daemon stays stopped and disarmed\n", len(overlap), strings.Join(redacted, ", "))
	return probeFailed
}

// runOfficialSwitchover stops the running official daemon and disarms its
// autostart. It never deletes binaries or data — rollback stays
// "cc-connect-next daemon stop; re-enable and start the official service".
// It fails closed: if the official daemon still answers after the stop
// attempts, the switchover (and therefore the final sync) does not proceed.
func runOfficialSwitchover(p officialProbe, st officialInstallState, out func(format string, args ...any) bool) error {
	if !st.Running && !st.AutostartArmed && !st.ServiceRegistered {
		out("Official CC Connect daemon: nothing to stop or disarm.\n")
		return nil
	}

	// Stop a registered service even when its API socket is not live yet. A
	// launchd/systemd restart loop can be between attempts and still begin
	// writing the source after a socket-only probe said "not running".
	if st.Running || st.ServiceRegistered {
		stopped := false
		if st.BinaryPath != "" {
			out("Stopping the official daemon via %s daemon stop…\n", st.BinaryPath)
			if _, err := p.RunCommand(st.BinaryPath, "daemon", "stop"); err == nil {
				stopped = true
			}
		}
		if !stopped {
			switch p.GOOS {
			case "darwin":
				out("Stopping the official daemon via launchctl bootout…\n")
				_, _ = p.RunCommand("launchctl", "bootout", fmt.Sprintf("gui/%d/%s", p.UID, officialServiceLabel))
			case "linux":
				out("Stopping the official daemon via systemctl…\n")
				_, _ = p.RunCommand("systemctl", officialSystemctlArgs(st.ServicePath, "stop")...)
			case "windows":
				out("Stopping the official daemon via Task Scheduler…\n")
				_, _ = runOfficialWindowsTaskCommand(p, "Stop-ScheduledTask -TaskName 'cc-connect' -ErrorAction SilentlyContinue")
			}
		}
	}

	if st.ServiceRegistered {
		switch p.GOOS {
		case "darwin":
			out("Disabling the official service autostart (launchctl disable)…\n")
			if _, err := p.RunCommand("launchctl", "disable", fmt.Sprintf("gui/%d/%s", p.UID, officialServiceLabel)); err != nil {
				return fmt.Errorf("disable official launchd service: %w (disable it manually, then rerun)", err)
			}
		case "linux":
			out("Disabling the official service autostart (systemctl disable)…\n")
			if _, err := p.RunCommand("systemctl", officialSystemctlArgs(st.ServicePath, "disable")...); err != nil {
				return fmt.Errorf("disable official systemd service: %w (disable it manually, then rerun)", err)
			}
		case "windows":
			out("Disabling the official scheduled task autostart…\n")
			if _, err := runOfficialWindowsTaskCommand(p, "Disable-ScheduledTask -TaskName 'cc-connect' | Out-Null"); err != nil {
				return fmt.Errorf("disable official scheduled task: %w (disable it manually, then rerun)", err)
			}
		default:
			return fmt.Errorf("automatic switchover is not supported on %s; stop and disable the official service manually, then rerun without --switch", p.GOOS)
		}
	}

	// Fail closed: the final sync must only run against a quiet source.
	if err := waitForOfficialQuiescence(p, st); err != nil {
		return err
	}
	out("Official daemon stopped and autostart disarmed; data and binaries left untouched.\n")
	return nil
}

func waitForOfficialQuiescence(p officialProbe, before officialInstallState) error {
	var after officialInstallState
	var lastProbeErr error
	for attempt := 0; attempt < 50; attempt++ {
		after = probeOfficialInstall(p)
		if after.ProbeErr != nil {
			lastProbeErr = after.ProbeErr
			if p.Sleep != nil {
				p.Sleep(100 * time.Millisecond)
			}
			continue
		}
		lastProbeErr = nil
		runningStopped := !after.Running
		autostartDisabled := !before.ServiceRegistered || !before.AutostartArmed || !after.AutostartArmed
		if runningStopped && autostartDisabled {
			return nil
		}
		if p.Sleep != nil {
			p.Sleep(100 * time.Millisecond)
		}
	}
	if lastProbeErr != nil {
		return fmt.Errorf("could not verify official daemon quiescence after stop/disable: %w", lastProbeErr)
	}
	if after.Running {
		return fmt.Errorf("the official daemon is still running (%s) after the stop attempts; stop it manually, then rerun", after.RunningVia)
	}
	return fmt.Errorf("the official daemon autostart is still armed after the disable command; disable it manually, then rerun")
}

func runOfficialWindowsTaskCommand(p officialProbe, command string) (string, error) {
	if p.RunCommand == nil {
		return "", fmt.Errorf("command runner is unavailable")
	}
	return p.RunCommand("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "$ErrorActionPreference = 'Stop'\n"+command)
}

// restoreOfficialInstall returns official CC Connect to the running/autostart
// state observed before a failed direct cutover. It is deliberately best-effort
// and returns every recovery error to the caller instead of hiding downtime.
func restoreOfficialInstall(p officialProbe, before officialInstallState, out func(format string, args ...any) bool) error {
	if !before.ServiceRegistered && !before.Running {
		return nil
	}
	if p.RunCommand == nil {
		return fmt.Errorf("command runner is unavailable")
	}
	var recoveryErrors []error
	if before.ServiceRegistered && before.AutostartArmed {
		out("Re-enabling official CC Connect autostart…\n")
		switch p.GOOS {
		case "darwin":
			if _, err := p.RunCommand("launchctl", "enable", fmt.Sprintf("gui/%d/%s", p.UID, officialServiceLabel)); err != nil {
				recoveryErrors = append(recoveryErrors, fmt.Errorf("enable official launchd service: %w", err))
			}
		case "linux":
			if _, err := p.RunCommand("systemctl", officialSystemctlArgs(before.ServicePath, "enable")...); err != nil {
				recoveryErrors = append(recoveryErrors, fmt.Errorf("enable official systemd service: %w", err))
			}
		case "windows":
			if _, err := runOfficialWindowsTaskCommand(p, "Enable-ScheduledTask -TaskName 'cc-connect' | Out-Null"); err != nil {
				recoveryErrors = append(recoveryErrors, fmt.Errorf("enable official scheduled task: %w", err))
			}
		default:
			recoveryErrors = append(recoveryErrors, fmt.Errorf("automatic official-service recovery is not supported on %s", p.GOOS))
		}
	}

	if before.Running {
		out("Restarting official CC Connect after the failed cutover…\n")
		started := false
		var cliStartErr error
		if before.BinaryPath != "" {
			if _, err := p.RunCommand(before.BinaryPath, "daemon", "start"); err == nil {
				started = true
			} else {
				cliStartErr = fmt.Errorf("start official daemon via CLI: %w", err)
			}
		}
		if !started {
			var err error
			switch p.GOOS {
			case "darwin":
				_, err = p.RunCommand("launchctl", "bootstrap", fmt.Sprintf("gui/%d", p.UID), before.ServicePath)
				if err == nil {
					_, err = p.RunCommand("launchctl", "kickstart", "-kp", fmt.Sprintf("gui/%d/%s", p.UID, officialServiceLabel))
				}
			case "linux":
				_, err = p.RunCommand("systemctl", officialSystemctlArgs(before.ServicePath, "start")...)
			case "windows":
				_, err = runOfficialWindowsTaskCommand(p, "Start-ScheduledTask -TaskName 'cc-connect'")
			default:
				err = fmt.Errorf("automatic official daemon recovery is not supported on %s", p.GOOS)
			}
			if err != nil {
				recoveryErrors = append(recoveryErrors, errors.Join(cliStartErr, err))
			}
		}
	}
	return errors.Join(recoveryErrors...)
}

func waitForOfficialRestored(p officialProbe, before officialInstallState) error {
	var after officialInstallState
	var lastProbeErr error
	for attempt := 0; attempt < 100; attempt++ {
		after = probeOfficialInstall(p)
		if after.ProbeErr != nil {
			lastProbeErr = after.ProbeErr
			if p.Sleep != nil {
				p.Sleep(100 * time.Millisecond)
			}
			continue
		}
		lastProbeErr = nil
		runningRestored := !before.Running || after.Running
		autostartRestored := !before.ServiceRegistered || !before.AutostartArmed || after.AutostartArmed
		if runningRestored && autostartRestored {
			return nil
		}
		if p.Sleep != nil {
			p.Sleep(100 * time.Millisecond)
		}
	}
	if lastProbeErr != nil {
		return fmt.Errorf("could not verify official CC Connect recovery: %w", lastProbeErr)
	}
	return fmt.Errorf("official CC Connect recovery was not observable (wanted running=%t autostart=%t; got running=%t autostart=%t)", before.Running, before.AutostartArmed, after.Running, after.AutostartArmed)
}
