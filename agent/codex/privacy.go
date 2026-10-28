package codex

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/timmyagentic/cc-connect-next/core"
	"github.com/timmyagentic/cc-connect-next/internal/privacyguard"
	"github.com/timmyagentic/cc-connect-next/internal/processgroup"
)

const privacyProfilePrefix = "cc-connect-privacy-"

type codexPrivacyPolicy struct {
	prefix string
	args   []string
}

func (p *codexPrivacyPolicy) profile(mode string) string {
	switch normalizeMode(mode) {
	case "auto-edit", "full-auto":
		return p.prefix + "workspace"
	case "yolo":
		return p.prefix + "yolo"
	default:
		return p.prefix + "read-only"
	}
}

func localAppServer(url string) bool {
	switch strings.ToLower(strings.TrimSpace(url)) {
	case "", "stdio", "stdio://":
		return true
	default:
		return false
	}
}

func newCodexPrivacyPolicy(workDir string, extraArgs []string) (*codexPrivacyPolicy, error) {
	if !privacyguard.Active() {
		return nil, nil
	}
	if err := validatePrivacyArgs(extraArgs); err != nil {
		return nil, err
	}
	denied, allowed, err := privacyguard.Paths(workDir)
	if err != nil {
		return nil, fmt.Errorf("codex macOS privacy paths: %w", err)
	}
	return buildCodexPrivacyPolicy(workDir, denied, allowed)
}

func buildCodexPrivacyPolicy(workDir string, denied, allowed []string) (*codexPrivacyPolicy, error) {
	workDir, err := filepath.Abs(workDir)
	if err != nil {
		return nil, fmt.Errorf("codex macOS privacy work directory: %w", err)
	}
	cwdExempt := false
	for _, path := range allowed {
		if filepath.Clean(path) == workDir {
			cwdExempt = true
			break
		}
	}
	gitWritePaths, err := privacyGitWritePaths(workDir, denied, allowed)
	if err != nil {
		return nil, err
	}
	// Config layers merge tables. A unique session namespace prevents stored
	// profiles from adding narrower grants or a network override to our policy.
	p := &codexPrivacyPolicy{prefix: privacyProfilePrefix + rand.Text() + "-"}
	for _, mode := range []string{"suggest", "full-auto", "yolo"} {
		name := p.profile(mode)
		var fields []string
		fs := make(map[string]string, len(denied)+len(allowed)+1)
		if mode == "yolo" {
			fs["/"] = "write"
			// Named profiles protect .git implicitly even with root write.
			// Legacy yolo allowed normal project Git operations.
			for _, path := range gitWritePaths {
				fs[path] = "write"
			}
		} else {
			parent := ":read-only"
			if mode == "full-auto" {
				parent = ":workspace"
			}
			fields = append(fields, "extends="+tomlString(parent))
		}
		for _, path := range denied {
			fs[path] = "deny"
		}
		for _, path := range allowed {
			if !filepath.IsAbs(path) || strings.ContainsAny(path, "*?[]{}") {
				return nil, fmt.Errorf("codex macOS privacy requires literal absolute paths without glob characters")
			}
			path = filepath.Clean(path)
			// A narrower read grant would downgrade a writable project subtree.
			// Omit redundant descendants instead of granting writes that could
			// override native restrictions on .git and other protected paths.
			if mode != "yolo" && cwdExempt && strings.HasPrefix(path, workDir+string(filepath.Separator)) {
				continue
			}
			access := "read"
			// Only the selected cwd restores workspace writes. Explicit allow
			// paths restore reading; Codex still protects native subpaths.
			if mode == "yolo" || (mode == "full-auto" && filepath.Clean(path) == workDir) {
				access = "write"
			}
			fs[path] = access
		}
		keys := make([]string, 0, len(fs))
		for path := range fs {
			if !filepath.IsAbs(path) || strings.ContainsAny(path, "*?[]{}") {
				return nil, fmt.Errorf("codex macOS privacy requires literal absolute paths without glob characters")
			}
			keys = append(keys, path)
		}
		sort.Strings(keys)
		entries := make([]string, 0, len(keys))
		for _, path := range keys {
			entries = append(entries, tomlString(path)+"="+tomlString(fs[path]))
		}
		fields = append(fields, "filesystem={"+strings.Join(entries, ",")+"}")
		if mode == "yolo" {
			fields = append(fields, "network={enabled=true}")
		}
		p.args = append(p.args, "-c", "permissions."+name+"={"+strings.Join(fields, ",")+"}")
	}
	return p, nil
}

func tomlString(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func (p *codexPrivacyPolicy) launchArgs(mode, approval string) []string {
	args := append([]string(nil), p.args...)
	return append(args, "-c", "default_permissions="+tomlString(p.profile(mode)), "-c", "approval_policy="+tomlString(approval))
}

// Reject custom policy selectors rather than letting a CLI flag override our
// native profile. Parse TOML keys so quoted keys and inline tables cannot evade
// the check. Unrelated provider/model configuration remains supported.
func validatePrivacyArgs(args []string) error {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		flag, _, _ := strings.Cut(arg, "=")
		switch flag {
		case "--dangerously-bypass-approvals-and-sandbox", "--yolo", "--full-auto",
			"--sandbox", "--permissions", "--ask-for-approval", "--profile", "--add-dir", "--cd":
			return fmt.Errorf("codex macOS privacy conflicts with custom flag %s", flag)
		}
		if len(arg) >= 2 && arg[0] == '-' && strings.ContainsRune("sPaCp", rune(arg[1])) {
			return fmt.Errorf("codex macOS privacy conflicts with custom flag %s", arg[:2])
		}
		var config string
		switch {
		case arg == "-c" || arg == "--config":
			i++
			if i >= len(args) {
				return fmt.Errorf("codex macOS privacy: missing custom config value")
			}
			config = args[i]
		case strings.HasPrefix(arg, "--config="):
			config = strings.TrimPrefix(arg, "--config=")
		case strings.HasPrefix(arg, "-c") && !strings.HasPrefix(arg, "--"):
			config = strings.TrimPrefix(strings.TrimPrefix(arg, "-c"), "=")
		default:
			continue
		}
		key, _, ok := strings.Cut(config, "=")
		var decoded map[string]any
		// The CLI accepts bare string values too; only the key needs decoding.
		if !ok {
			return fmt.Errorf("codex macOS privacy: invalid custom config override")
		}
		if _, err := toml.Decode(key+"=true", &decoded); err != nil {
			return fmt.Errorf("codex macOS privacy: invalid custom config key")
		}
		for root := range decoded {
			switch root {
			case "permissions", "default_permissions", "sandbox_mode", "sandbox_workspace_write",
				"approval_policy", "approvals_reviewer", "profile", "profiles":
				return fmt.Errorf("codex macOS privacy conflicts with custom config %s", root)
			}
		}
	}
	return nil
}

var privacyVersionPattern = regexp.MustCompile(`^codex-cli ([0-9]+)\.([0-9]+)\.([0-9]+)$`)

func validatePrivacyVersion(output string) error {
	matches := privacyVersionPattern.FindStringSubmatch(strings.TrimSpace(output))
	if len(matches) == 4 {
		major, e1 := strconv.Atoi(matches[1])
		minor, e2 := strconv.Atoi(matches[2])
		_, e3 := strconv.Atoi(matches[3])
		if e1 == nil && e2 == nil && e3 == nil && (major > 0 || minor >= 163) {
			return nil
		}
	}
	return fmt.Errorf("codex macOS privacy requires a supported stable codex-cli >= 0.163.0; installed version is unsupported or unrecognized")
}

// --version does not authenticate or launch a model. A conservative floor
// prevents older CLIs silently ignoring unknown permissions config keys.
func checkPrivacyRuntime(ctx context.Context, bin, workDir string, extraEnv []string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "--version")
	cmd.Dir = workDir
	cmd.Env = core.MergeEnv(os.Environ(), extraEnv)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("codex macOS privacy: cannot verify CLI version: %w", err)
	}
	return validatePrivacyVersion(string(output))
}

func privacyWorkspaceMode(mode string) bool {
	mode = normalizeMode(mode)
	return mode == "auto-edit" || mode == "full-auto"
}

// :workspace is a built-in baseline, not a migration of sandbox_workspace_write.
// In particular it can restore TMPDIR writes that a legacy policy excluded.
// Until those custom policies can be preserved exactly, refuse them explicitly.
func checkPrivacyWorkspaceConfig(request func(string, any, any) error, workDir string) error {
	cwd, err := filepath.Abs(workDir)
	if err != nil {
		return fmt.Errorf("codex macOS privacy resolve config cwd: %w", err)
	}
	var response struct {
		Config map[string]json.RawMessage `json:"config"`
	}
	if err := request("config/read", map[string]any{"includeLayers": false, "cwd": cwd}, &response); err != nil {
		return fmt.Errorf("codex macOS privacy cannot verify effective workspace policy: %w", err)
	}
	if response.Config == nil {
		return fmt.Errorf("codex macOS privacy cannot verify effective workspace policy: missing config")
	}
	raw, ok := response.Config["sandbox_workspace_write"]
	if !ok {
		return nil
	}
	var policy map[string]json.RawMessage
	if err := json.Unmarshal(raw, &policy); err != nil {
		return fmt.Errorf("codex macOS privacy cannot decode effective workspace policy")
	}
	for key, value := range policy {
		supported := false
		switch key {
		case "writable_roots":
			var roots []string
			supported = json.Unmarshal(value, &roots) == nil && len(roots) == 0
		case "network_access", "exclude_tmpdir_env_var", "exclude_slash_tmp":
			var enabled bool
			supported = json.Unmarshal(value, &enabled) == nil && !enabled
		}
		if !supported {
			return fmt.Errorf("codex macOS privacy: custom sandbox_workspace_write.%s is not supported by the native workspace profile; refusing to change the existing policy", key)
		}
	}
	return nil
}

// Exec reloads config for every process, so inspect its effective layers before
// each guarded workspace turn. This probe sends only initialize and config/read.
func checkExecPrivacyWorkspaceConfig(ctx context.Context, bin string, extraArgs []string, workDir string, extraEnv []string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	args := append(append([]string(nil), extraArgs...), "app-server")
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = workDir
	cmd.Env = core.MergeEnv(os.Environ(), extraEnv)
	processgroup.Prepare(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("codex macOS privacy config stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return fmt.Errorf("codex macOS privacy config stdout: %w", err)
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return fmt.Errorf("codex macOS privacy config probe: %w", err)
	}
	defer func() {
		_ = stdin.Close()
		_ = processgroup.Kill(cmd)
		_ = cmd.Wait()
	}()
	reader := bufio.NewReader(stdout)
	if err := rpcRequestOverIO(stdin, reader, 1, "initialize", map[string]any{
		"clientInfo": map[string]any{"name": "cc-connect-next-privacy-config", "version": "0.1.0"},
	}, nil); err != nil {
		return fmt.Errorf("codex macOS privacy config initialize: %w", err)
	}
	if err := rpcNotifyOverIO(stdin, "initialized", nil); err != nil {
		return err
	}
	return checkPrivacyWorkspaceConfig(func(method string, params, out any) error {
		return rpcRequestOverIO(stdin, reader, 2, method, params, out)
	}, workDir)
}
