package main

// Migration deliberately never modifies the official CC Connect
// installation: rollback must always be "stop next, start official", and a
// trial must be able to run while the official daemon keeps serving. The
// cost of that guarantee used to be carried by documentation alone — the
// "never run both daemons against the same platform credentials" rule lived
// in migration.md and nowhere in the product. These probes give the three
// places that can actually prevent a dual-consumer incident — runtime
// startup, `migrate`, and `doctor` — eyes on the official install.

import (
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
	Home        string
	GOOS        string
	UID         int
	LookPath    func(file string) (string, error)
	RunCommand  func(name string, args ...string) (string, error)
	DialTimeout func(network, address string, timeout time.Duration) (net.Conn, error)
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
		}
	case "linux":
		for _, unit := range []string{
			filepath.Join(p.Home, ".config", "systemd", "user", "cc-connect.service"),
			"/etc/systemd/system/cc-connect.service",
		} {
			if _, err := os.Stat(unit); err == nil {
				st.ServiceRegistered = true
				st.ServicePath = unit
				st.AutostartArmed = systemdServiceEnabled(p, unit)
				break
			}
		}
	}

	// A live official daemon always serves its API socket; a connectable
	// socket is authoritative, a stale socket file refuses the dial. On
	// Windows the official daemon has no unix socket, so Running stays
	// best-effort false there and the guidance leans on the service state.
	if p.GOOS != "windows" && p.DialTimeout != nil {
		sock := filepath.Join(p.Home, ".cc-connect", "run", "api.sock")
		if conn, err := p.DialTimeout("unix", sock, 400*time.Millisecond); err == nil {
			_ = conn.Close()
			st.Running = true
			st.RunningVia = "live API socket " + sock
		}
	}

	st.AppIDs = readOfficialAppIDs(p.Home)
	return st
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
	if err != nil {
		return true // cannot tell → treat as armed; drives warnings only
	}
	return strings.HasPrefix(strings.TrimSpace(out), "enabled")
}

var appIDRe = regexp.MustCompile(`(?m)^\s*app_id\s*=\s*"([^"]+)"`)
var officialEnvPlaceholderRe = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// readOfficialAppIDs collects platform credential IDs from the official
// config. Values are compared, hashed, or redacted — never echoed whole.
func readOfficialAppIDs(home string) []string {
	data, err := os.ReadFile(filepath.Join(home, ".cc-connect", "config.toml"))
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
	default:
		return "  disable the official CC Connect scheduled task / service in your service manager"
	}
}

// officialConflictRefusal returns a non-empty startup refusal when an
// official daemon is running right now with credentials this config also
// uses. Two consumers on one credential race for the same events; refusing
// to start is strictly better than duplicating half the replies.
func officialConflictRefusal(st officialInstallState, overlap []string, goos string, uid int) string {
	if !st.Running || len(overlap) == 0 {
		return ""
	}
	return fmt.Sprintf(`Refusing to start: the official CC Connect daemon is running (%s)
and shares %d platform credential(s) with this configuration.
Two daemons consuming the same credentials race for the same events and
produce duplicate or lost replies.

Either switch over — stop and disarm the official daemon:
  cc-connect daemon stop
%s
Or keep a side-by-side trial by giving this config separate test-app credentials.

Set CC_NEXT_ALLOW_OFFICIAL_CONFLICT=1 to start anyway (not recommended).
`, st.RunningVia, len(overlap), disarmOfficialHint(goos, uid))
}

// renderOfficialCoexistenceGuidance is the post-migration report block that
// replaces the old passive "was not modified or stopped" single line
// whenever there is anything actionable to say.
func renderOfficialCoexistenceGuidance(st officialInstallState, overlap []string, goos string, uid int, targetConfig, workDir string) string {
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
	b.WriteString("  cc-connect daemon stop\n")
	b.WriteString(disarmOfficialHint(goos, uid) + "\n")
	b.WriteString("  cc-connect-next migrate --force        # final sync once the official daemon is quiet\n")
	fmt.Fprintf(&b, "  cc-connect-next daemon install --config %s --work-dir %s\n", targetConfig, workDir)
	b.WriteString("Or run `cc-connect-next migrate --switch` to perform the stop, disarm, and final sync in one command.\n")
	if len(overlap) > 0 {
		b.WriteString("To keep a side-by-side trial instead, give the migrated config separate test-app credentials.\n")
	}
	return b.String()
}

// printOfficialCoexistenceSection renders the doctor section describing the
// official install. Returns true when the state is an active failure (an
// official daemon running with shared credentials).
func printOfficialCoexistenceSection(w io.Writer, cfg *config.Config) bool {
	probe := defaultOfficialProbe()
	st := probeOfficialInstall(probe)
	if st.BinaryPath == "" && !st.ServiceRegistered && !st.Running && len(st.AppIDs) == 0 {
		return false // no official install detected; keep doctor output quiet
	}

	fmt.Fprintf(w, "\n=== official CC Connect coexistence ===\n")
	if st.BinaryPath != "" {
		fmt.Fprintf(w, "✅ binary: %s\n", st.BinaryPath)
	} else {
		fmt.Fprintf(w, "✅ binary: not on PATH\n")
	}
	switch {
	case !st.ServiceRegistered:
		fmt.Fprintf(w, "✅ autostart: not registered\n")
	case st.AutostartArmed:
		fmt.Fprintf(w, "⚠️ autostart: ARMED (%s) — next login/boot starts the official daemon\n   disarm: %s\n", st.ServicePath, strings.TrimSpace(disarmOfficialHint(probe.GOOS, probe.UID)))
	default:
		fmt.Fprintf(w, "✅ autostart: registered but disabled (%s)\n", st.ServicePath)
	}
	if st.Running {
		fmt.Fprintf(w, "⚠️ daemon: RUNNING (%s)\n", st.RunningVia)
	} else {
		fmt.Fprintf(w, "✅ daemon: not running\n")
	}

	overlap := officialCredentialOverlap(st, collectConfigAppIDs(cfg))
	if len(overlap) == 0 {
		fmt.Fprintf(w, "✅ shared credentials: none\n")
		return false
	}
	redacted := make([]string, len(overlap))
	for i, id := range overlap {
		redacted[i] = redactCredentialID(id)
	}
	if st.Running {
		fmt.Fprintf(w, "❌ shared credentials: %d (%s) — both daemons consume the same event stream; stop and disarm the official daemon or use test-app credentials\n", len(overlap), strings.Join(redacted, ", "))
		return true
	}
	fmt.Fprintf(w, "⚠️ shared credentials: %d (%s) — safe only while the official daemon stays stopped and disarmed\n", len(overlap), strings.Join(redacted, ", "))
	return false
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

	if st.Running {
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
		default:
			return fmt.Errorf("automatic switchover is not supported on %s; stop and disable the official service manually, then rerun without --switch", p.GOOS)
		}
	}

	// Fail closed: the final sync must only run against a quiet source.
	after := probeOfficialInstall(p)
	if after.Running {
		return fmt.Errorf("the official daemon is still running (%s) after the stop attempts; stop it manually, then rerun", after.RunningVia)
	}
	out("Official daemon stopped and autostart disarmed; data and binaries left untouched.\n")
	return nil
}
