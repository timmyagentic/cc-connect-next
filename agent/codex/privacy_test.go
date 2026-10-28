package codex

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/timmyagentic/cc-connect-next/internal/privacyguard"
)

func privacyTestPolicy(t *testing.T) *codexPrivacyPolicy {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("synthetic POSIX policy paths")
	}
	p, err := buildCodexPrivacyPolicy("/synthetic/Documents/project",
		[]string{"/synthetic/Desktop", "/synthetic/Documents"},
		[]string{"/synthetic/Documents/project", "/synthetic/Desktop/reference", "/synthetic/Documents/project/explicit"})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPrivacyProfiles_PreserveNativeModeRestrictions(t *testing.T) {
	p := privacyTestPolicy(t)
	var decoded struct {
		Permissions map[string]struct {
			Extends    string
			Filesystem map[string]string
			Network    struct{ Enabled bool }
		}
	}
	var overrides []string
	for i := 1; i < len(p.args); i += 2 {
		overrides = append(overrides, p.args[i])
	}
	if _, err := toml.Decode(strings.Join(overrides, "\n"), &decoded); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		mode, parent, projectAccess, outsideAccess string
		network                                    bool
	}{
		{"suggest", ":read-only", "read", "read", false},
		{"full-auto", ":workspace", "write", "read", false},
		{"yolo", "", "write", "write", true},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			got := decoded.Permissions[p.profile(tc.mode)]
			if got.Extends != tc.parent || got.Network.Enabled != tc.network {
				t.Fatalf("mode restrictions changed: %+v", got)
			}
			want := map[string]string{
				"/synthetic/Desktop":           "deny",
				"/synthetic/Documents":         "deny",
				"/synthetic/Documents/project": tc.projectAccess,
				"/synthetic/Desktop/reference": tc.outsideAccess,
			}
			if tc.mode == "yolo" {
				want["/"] = "write"
				want["/synthetic/Documents/project/.git"] = "write"
				want["/synthetic/Documents/project/explicit"] = "write"
			}
			if !reflect.DeepEqual(got.Filesystem, want) {
				t.Fatalf("filesystem = %v, want %v", got.Filesystem, want)
			}
		})
	}
}

func TestPrivacyProfiles_ExecFreshAndResumeNeverBypass(t *testing.T) {
	for _, mode := range []string{"suggest", "auto-edit", "full-auto", "yolo"} {
		for _, resume := range []string{"", "thread-existing"} {
			t.Run(mode+"/"+resume, func(t *testing.T) {
				s := &codexSession{mode: mode, workDir: "/synthetic/project", privacy: privacyTestPolicy(t)}
				s.threadID.Store(resume)
				args := s.launchArgs("hello", nil)
				for _, forbidden := range []string{"--sandbox", "-P", "--dangerously-bypass-approvals-and-sandbox", "--full-auto"} {
					if indexOf(args, forbidden) >= 0 {
						t.Fatalf("guarded exec contains %s: %q", forbidden, args)
					}
				}
				for _, want := range [][]string{
					{"-c", "default_permissions=" + tomlString(s.privacy.profile(mode))},
					{"-c", `approval_policy="never"`},
				} {
					if !containsSequence(args, want) {
						t.Fatalf("missing %q in %q", want, args)
					}
				}
				if resume != "" && (indexOf(args, "resume") < 0 || indexOf(args, "--cd") >= 0) {
					t.Fatalf("invalid resume args: %q", args)
				}
			})
		}
	}
}

func TestPrivacyProfiles_AppServerThreadAndTurnUseNamedPolicy(t *testing.T) {
	for _, mode := range []string{"suggest", "auto-edit", "full-auto", "yolo"} {
		t.Run(mode, func(t *testing.T) {
			s := &appServerSession{mode: mode, model: "fake-model", workDir: "/synthetic/project", privacy: privacyTestPolicy(t)}
			wantApproval := "never"
			if mode == "suggest" {
				wantApproval = "on-request"
			}
			for name, params := range map[string]map[string]any{
				"start/resume": s.threadRequestParams(),
				"turn":         s.turnStartParams("thread", nil, nil),
			} {
				if params["permissions"] != s.privacy.profile(mode) || params["approvalPolicy"] != wantApproval {
					t.Fatalf("%s policy missing: %v", name, params)
				}
				for _, forbidden := range []string{"sandbox", "sandboxPolicy"} {
					if _, ok := params[forbidden]; ok {
						t.Fatalf("%s contains conflicting %s", name, forbidden)
					}
				}
			}
			if s.threadRequestParams()["cwd"] != s.workDir {
				t.Fatal("resume must retain the workspace used for exceptions")
			}
			if !containsSequence(s.launchArgs(), []string{"-c", "default_permissions=" + tomlString(s.privacy.profile(mode))}) {
				t.Fatal("app-server did not receive native profiles")
			}
			if !containsSequence(s.launchArgs(), []string{"-c", "approval_policy=" + tomlString(wantApproval)}) {
				t.Fatal("app-server launch changed interactive approval mode")
			}
			args := s.sessionTitleGenerationArgs("/synthetic/tmp")
			if indexOf(args, "-s") >= 0 || !containsSequence(args, []string{"-c", "default_permissions=" + tomlString(s.privacy.profile("suggest"))}) {
				t.Fatalf("title helper must retain read-only privacy policy: %q", args)
			}
			if !containsSequence(args, []string{"-c", `approval_policy="never"`}) {
				t.Fatal("title exec helper must not wait for interactive approvals")
			}
		})
	}
}

func TestPrivacyArgs_RejectConflictingCustomFlags(t *testing.T) {
	for _, args := range [][]string{
		{"--yolo"}, {"--dangerously-bypass-approvals-and-sandbox"}, {"--full-auto"},
		{"--sandbox=read-only"}, {"-s", "workspace-write"}, {"-sdanger-full-access"},
		{"-Punsafe"}, {"--permissions", "unsafe"}, {"-a", "on-request"},
		{"--ask-for-approval=never"}, {"--add-dir=/"},
		{"-C/"}, {"--cd", "/"}, {"--profile", "unsafe"}, {"-punsafe"},
		{"-c", `permissions."cc-connect-privacy-yolo".filesystem."/"="write"`},
		{"--config", `"sandbox_mode"="danger-full-access"`},
		{"--config='default_permissions'=':danger-full-access'"},
		{"-capproval_policy=on-request"}, {"-c=profiles.x.sandbox_mode=unsafe"},
		{"-c", "sandbox_workspace_write.network_access=true"},
		{"-c"}, {"--config=malformed"},
	} {
		if err := validatePrivacyArgs(args); err == nil {
			t.Errorf("accepted conflicting args %q", args)
		}
	}
	for _, args := range [][]string{
		nil,
		{"-c", `model="example"`, "--config", "model_reasoning_effort=high"},
		{"--config=service_tier=fast", "-cmodel_provider=example"},
	} {
		if err := validatePrivacyArgs(args); err != nil {
			t.Errorf("rejected unrelated args %q: %v", args, err)
		}
	}
}

func TestPrivacyVersion_RejectOldUnknownAndPrereleaseRuntime(t *testing.T) {
	for _, version := range []string{"codex-cli 0.163.0", "codex-cli 0.164.1\n", "codex-cli 1.0.0"} {
		if err := validatePrivacyVersion(version); err != nil {
			t.Errorf("%q: %v", version, err)
		}
	}
	for _, version := range []string{"", "codex-cli 0.162.9", "0.163.0", "codex-cli 0.163.0-alpha.1", "other 0.163.0"} {
		if err := validatePrivacyVersion(version); err == nil {
			t.Errorf("accepted unsupported runtime %q", version)
		}
	}
}

func TestPrivacyPaths_QuoteLiteralPaths(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("synthetic POSIX policy paths")
	}
	path := "/synthetic/quote\" and slash\\dir"
	p, err := buildCodexPrivacyPolicy("/synthetic/project", []string{path}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if _, err := toml.Decode(p.args[1], &parsed); err != nil {
		t.Fatalf("invalid TOML quoting: %v", err)
	}
	for _, path := range []string{"relative", "/synthetic/*", "/synthetic/[abc]"} {
		if _, err := buildCodexPrivacyPolicy("/synthetic/project", []string{path}, nil); err == nil {
			t.Fatalf("accepted non-literal path %q", path)
		}
	}
}

func TestPrivacyRuntime_OldCLIStopsBeforeSessionStart(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS activation only")
	}
	if err := privacyguard.Configure(privacyguard.Settings{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = privacyguard.Configure(privacyguard.Settings{}) })
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "codex")
	marker := filepath.Join(dir, "launched")
	script := "#!/bin/sh\nif [ \"$1\" = --version ]; then echo 'codex-cli 0.162.0'; exit 0; fi\n: > " + tomlString(marker) + "\nexit 1\n"
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, backend := range []string{"exec", "app_server"} {
		var err error
		if backend == "exec" {
			_, err = newCodexSession(context.Background(), codexSessionParams{cliBin: bin, workDir: dir})
		} else {
			_, err = newAppServerSession(context.Background(), appServerSessionParams{cliBin: bin, workDir: dir})
		}
		if err == nil || !strings.Contains(err.Error(), "0.163.0") {
			t.Fatalf("%s did not fail closed: %v", backend, err)
		}
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("unsupported CLI launched a session: %v", err)
	}
}

func TestPrivacyActivation_DisabledAndRemote(t *testing.T) {
	if p, err := newCodexPrivacyPolicy("/synthetic", []string{"--yolo"}); err != nil || p != nil {
		t.Fatalf("disabled guard altered legacy behavior: %v, %v", p, err)
	}
	for _, url := range []string{"", "stdio", "stdio://", " STDIO:// "} {
		if !localAppServer(url) {
			t.Errorf("local transport excluded: %q", url)
		}
	}
	for _, url := range []string{"ws://remote:1234", "wss://remote", "http://remote"} {
		if localAppServer(url) {
			t.Errorf("remote transport included: %q", url)
		}
	}
}

func TestPrivacyRuntime_ExecAndTitleRecheckReplacedBinary(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS activation only")
	}
	if err := privacyguard.Configure(privacyguard.Settings{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = privacyguard.Configure(privacyguard.Settings{}) })
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "codex")
	marker := filepath.Join(dir, "launched")
	script := "#!/bin/sh\nif [ \"$1\" = --version ]; then echo 'codex-cli 0.163.0'; exit 0; fi\n: > " + tomlString(marker) + "\nexit 1\n"
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	cs, err := newCodexSession(context.Background(), codexSessionParams{
		cliBin: bin, workDir: dir, mode: "yolo", resumeID: "retained-thread",
	})
	if err != nil {
		t.Fatal(err)
	}
	cleanupCodexTestSession(t, cs)
	if err := os.WriteFile(bin, []byte(strings.ReplaceAll(script, "0.163.0", "0.162.0")), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := cs.Send("synthetic", nil, nil); err == nil || !strings.Contains(err.Error(), "0.163.0") {
		t.Fatalf("exec accepted downgraded CLI: %v", err)
	}
	s := &appServerSession{cliBin: bin, privacy: cs.privacy}
	if _, err := s.generateSessionTitleWithCodex(context.Background(), "synthetic"); err == nil || !strings.Contains(err.Error(), "0.163.0") {
		t.Fatalf("title helper accepted downgraded CLI: %v", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("downgraded CLI launched a model process: %v", err)
	}
}

func TestPrivacySessionInitialization_LocalNativePolicyAndRemoteExclusion(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS activation only")
	}
	if err := privacyguard.Configure(privacyguard.Settings{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = privacyguard.Configure(privacyguard.Settings{}) })
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "codex")
	script := `#!/bin/sh
if [ "$1" = "--version" ]; then
  echo 'codex-cli 0.163.0'
  exit 0
fi
if [ -n "$PRIVACY_ARGS" ]; then printf '%s\n' "$@" > "$PRIVACY_ARGS"; fi
while IFS= read -r line; do
  if [ -n "$PRIVACY_RPC" ]; then printf '%s\n' "$line" >> "$PRIVACY_RPC"; fi
  id=$(printf '%s' "$line" | sed -n 's/.*"id":[[:space:]]*\([0-9][0-9]*\).*/\1/p')
  case "$line" in
    *'"method":"initialize"'*) printf '{"id":%s,"result":{"userAgent":"codex_cli_rs/0.163.0"}}\n' "$id" ;;
    *'"method":"config/read"'*)
      if [ "$PRIVACY_UNSUPPORTED" = "1" ]; then
        printf '{"id":%s,"result":{"config":{"sandbox_workspace_write":{"exclude_tmpdir_env_var":true}}}}\n' "$id"
      else
        printf '{"id":%s,"result":{"config":{}}}\n' "$id"
      fi ;;
    *'"method":"thread/start"'*|*'"method":"thread/resume"'*) printf '{"id":%s,"result":{"thread":{"id":"synthetic-thread"},"model":"fake","reasoningEffort":"high"}}\n' "$id" ;;
    *'"method":"turn/start"'*) printf '{"id":%s,"result":{"turn":{"id":"synthetic-turn"}}}\n' "$id" ;;
  esac
done
`
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"suggest", "full-auto", "yolo"} {
		for _, resume := range []string{"", "synthetic-thread"} {
			t.Run(mode+"/"+resume, func(t *testing.T) {
				wantApproval := "never"
				if mode == "suggest" {
					wantApproval = "on-request"
				}
				cs, err := newCodexSession(context.Background(), codexSessionParams{
					cliBin: bin, workDir: dir, mode: mode, resumeID: resume,
				})
				if err != nil {
					t.Fatal(err)
				}
				if cs.privacy == nil {
					t.Fatal("exec constructor omitted privacy policy")
				}
				cleanupCodexTestSession(t, cs)
				rpcFile := filepath.Join(t.TempDir(), "rpc")
				argsFile := filepath.Join(t.TempDir(), "args")
				s, err := newAppServerSession(context.Background(), appServerSessionParams{
					cliBin: bin, url: "stdio://", workDir: dir, mode: mode, resumeID: resume,
					model: "fake", effort: "high",
					extraEnv: []string{"PRIVACY_ARGS=" + argsFile, "PRIVACY_RPC=" + rpcFile},
				})
				if err != nil {
					t.Fatal(err)
				}
				cleanupCodexTestSession(t, s)
				if err := s.Send("synthetic input", nil, nil); err != nil {
					t.Fatal(err)
				}
				raw, err := os.ReadFile(rpcFile)
				if err != nil {
					t.Fatal(err)
				}
				threadMethod := "thread/start"
				if resume != "" {
					threadMethod = "thread/resume"
				}
				seen := make(map[string]bool)
				for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
					var req struct {
						Method string
						Params map[string]any
					}
					if err := json.Unmarshal([]byte(line), &req); err != nil {
						t.Fatal(err)
					}
					if req.Method != threadMethod && req.Method != "turn/start" {
						continue
					}
					seen[req.Method] = true
					if req.Params["permissions"] != s.privacy.profile(mode) || req.Params["approvalPolicy"] != wantApproval {
						t.Fatalf("unguarded RPC: %s", line)
					}
					if _, ok := req.Params["sandbox"]; ok {
						t.Fatalf("conflicting sandbox: %s", line)
					}
				}
				if !seen[threadMethod] || !seen["turn/start"] {
					t.Fatalf("missing RPCs: %s", raw)
				}
				args := waitForArgsFile(t, argsFile)
				if !containsSequence(args, []string{"-c", "default_permissions=" + tomlString(s.privacy.profile(mode))}) {
					t.Fatalf("app-server subprocess missing native profile: %q", args)
				}
			})
		}
	}
	// Host policy must not reject flags or inject paths for a remote server.
	s, err := newAppServerSession(context.Background(), appServerSessionParams{
		cliBin: bin, url: "ws://remote.invalid:1234", workDir: dir,
		cliExtraArgs: []string{"--yolo"}, model: "fake", effort: "high",
		extraEnv: []string{"PRIVACY_ARGS=" + filepath.Join(dir, "remote-args"), "PRIVACY_RPC=" + filepath.Join(dir, "remote-rpc")},
	})
	if err != nil {
		t.Fatal(err)
	}
	cleanupCodexTestSession(t, s)
	if s.privacy != nil {
		t.Fatal("host privacy policy applied to remote server")
	}
	for _, backend := range []string{"exec", "app_server"} {
		rpcFile := filepath.Join(t.TempDir(), "blocked-rpc")
		env := []string{"PRIVACY_UNSUPPORTED=1", "PRIVACY_RPC=" + rpcFile}
		var err error
		if backend == "exec" {
			_, err = newCodexSession(context.Background(), codexSessionParams{
				cliBin: bin, workDir: dir, mode: "full-auto", extraEnv: env,
			})
		} else {
			_, err = newAppServerSession(context.Background(), appServerSessionParams{
				cliBin: bin, workDir: dir, mode: "full-auto", extraEnv: env,
			})
		}
		if err == nil || !strings.Contains(err.Error(), "exclude_tmpdir_env_var") {
			t.Fatalf("%s continued with unsupported workspace restrictions: %v", backend, err)
		}
		raw, readErr := os.ReadFile(rpcFile)
		if readErr != nil {
			t.Fatal(readErr)
		}
		for _, method := range []string{"thread/start", "thread/resume", "turn/start", "model/list"} {
			if strings.Contains(string(raw), method) {
				t.Fatalf("%s reached %s after unsupported workspace policy: %s", backend, method, raw)
			}
		}
	}
}

// Opt-in OS enforcement check: no thread/model/auth request, only codex sandbox
// against synthetic paths and a fresh CODEX_HOME.
func TestPrivacySandbox_SyntheticNativeEnforcement(t *testing.T) {
	if runtime.GOOS != "darwin" || os.Getenv("CC_CODEX_PRIVACY_SANDBOX_TEST") != "1" {
		t.Skip("set CC_CODEX_PRIVACY_SANDBOX_TEST=1 on macOS")
	}
	bin, err := exec.LookPath("codex")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	desktop := filepath.Join(root, "Desktop")
	project := filepath.Join(desktop, "project")
	explicit := filepath.Join(project, "explicit")
	gitChild := filepath.Join(project, ".git", "explicit")
	allowed := filepath.Join(desktop, "reference")
	home := filepath.Join(root, "codex-home")
	for _, dir := range []string{project, explicit, gitChild, allowed, home} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{filepath.Join(desktop, "private"), filepath.Join(allowed, "file")} {
		if err := os.WriteFile(path, []byte("synthetic"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// CLI-selected profiles must win over legacy config and reserved-name
	// collisions without retaining a colliding network grant or broad parent.
	config := "sandbox_mode=\"danger-full-access\"\ndefault_permissions=\":danger-full-access\"\n" +
		"[permissions.cc-connect-privacy-read-only]\nextends=\":workspace\"\nnetwork.enabled=true\nfilesystem.\"/\"=\"write\"\n" +
		"[permissions.cc-connect-privacy-workspace]\nnetwork.enabled=true\nfilesystem.\"/\"=\"write\"\n"
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	// Deliberately put descendants before cwd so handling cannot depend on
	// upstream deduplication or exception ordering.
	p, err := buildCodexPrivacyPolicy(project, []string{desktop}, []string{explicit, gitChild, project, allowed})
	if err != nil {
		t.Fatal(err)
	}
	other, err := buildCodexPrivacyPolicy(project, []string{desktop}, []string{project, allowed})
	if err != nil {
		t.Fatal(err)
	}
	if p.profile("suggest") == other.profile("suggest") {
		t.Fatal("sessions must not reuse config-layer profile names")
	}
	env := []string{"CODEX_HOME=" + home, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_AUTHOR_NAME=Synthetic Privacy Test", "GIT_AUTHOR_EMAIL=synthetic@example.invalid",
		"GIT_COMMITTER_NAME=Synthetic Privacy Test", "GIT_COMMITTER_EMAIL=synthetic@example.invalid"}
	var inheritedEnv []string
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GIT_") {
			inheritedEnv = append(inheritedEnv, entry)
		}
	}
	if err := checkPrivacyRuntime(context.Background(), bin, project, env); err != nil {
		t.Fatal(err)
	}
	runIn := func(policy *codexPrivacyPolicy, cwd, mode string, success bool, command ...string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		args := append([]string{"sandbox"}, policy.launchArgs(mode, "never")...)
		args = append(args, "--")
		args = append(args, command...)
		cmd := exec.CommandContext(ctx, bin, args...)
		cmd.Dir = cwd
		cmd.Env = append(append([]string(nil), inheritedEnv...), env...)
		out, err := cmd.CombinedOutput()
		if ctx.Err() != nil || (err == nil) != success {
			t.Fatalf("%s %q: err=%v output=%s", mode, command, err, out)
		}
		if !success && !strings.Contains(string(out), "Operation not permitted") && !strings.Contains(string(out), "Permission denied") {
			t.Fatalf("expected OS permission denial, got %v: %s", err, out)
		}
	}
	run := func(mode string, success bool, command ...string) {
		t.Helper()
		runIn(p, project, mode, success, command...)
	}
	for _, mode := range []string{"suggest", "full-auto", "yolo"} {
		run(mode, false, "/bin/cat", filepath.Join(desktop, "private"))
		run(mode, true, "/bin/cat", filepath.Join(allowed, "file"))
		run(mode, mode != "suggest", "/usr/bin/touch", filepath.Join(project, "workspace-"+mode))
		run(mode, mode != "suggest", "/usr/bin/touch", filepath.Join(explicit, "descendant-"+mode))
		run(mode, mode == "yolo", "/usr/bin/touch", filepath.Join(allowed, "outside-"+mode))
		run(mode, mode == "yolo", "/usr/bin/touch", filepath.Join(project, ".git", "config-"+mode))
		run(mode, mode == "yolo", "/usr/bin/touch", filepath.Join(gitChild, "metadata-"+mode))
	}
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	gitProject := filepath.Join(root, "git-project")
	if err := os.MkdirAll(filepath.Join(gitProject, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	gitPolicy, err := buildCodexPrivacyPolicy(gitProject, []string{desktop}, []string{allowed})
	if err != nil {
		t.Fatal(err)
	}
	runIn(gitPolicy, gitProject, "yolo", true, git, "init", "--template=", "--initial-branch=synthetic")
	if err := os.WriteFile(filepath.Join(gitProject, "synthetic.txt"), []byte("synthetic content\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runIn(gitPolicy, gitProject, "yolo", true, git, "add", "synthetic.txt")
	runIn(gitPolicy, gitProject, "yolo", true, git, "-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null", "commit", "-m", "Synthetic privacy regression")
	runIn(gitPolicy, gitProject, "yolo", true, git, "rev-parse", "--verify", "HEAD")
	runIn(gitPolicy, gitProject, "yolo", false, "/usr/bin/find", desktop, "-maxdepth", "4", "-type", "f")
	runIn(gitPolicy, gitProject, "yolo", false, "/bin/sh", "-c", `/bin/cat "$1"`, "sh", filepath.Join(desktop, "private"))
	caseVariant := filepath.Join(root, "desktop", "private")
	if _, err := os.Stat(caseVariant); err == nil {
		runIn(gitPolicy, gitProject, "yolo", false, "/bin/cat", caseVariant)
	}
	scopedGit := filepath.Join(desktop, "scoped-git")
	if err := os.MkdirAll(scopedGit, 0700); err != nil {
		t.Fatal(err)
	}
	// Native Git inspects parent metadata. The runtime therefore exempts the
	// workspace's category, while unrelated categories remain denied.
	unrelated := filepath.Join(root, "Documents")
	if err := os.MkdirAll(unrelated, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(unrelated, "private"), []byte("synthetic"), 0600); err != nil {
		t.Fatal(err)
	}
	scopedPolicy, err := buildCodexPrivacyPolicy(scopedGit, []string{desktop, unrelated}, []string{desktop})
	if err != nil {
		t.Fatal(err)
	}
	runIn(scopedPolicy, scopedGit, "yolo", true, git, "init", "--template=", "--initial-branch=synthetic")
	if err := os.WriteFile(filepath.Join(scopedGit, "scoped.txt"), []byte("synthetic\n"), 0600); err != nil {
		t.Fatal(err)
	}
	runIn(scopedPolicy, scopedGit, "yolo", true, git, "add", "scoped.txt")
	runIn(scopedPolicy, scopedGit, "yolo", true, git, "-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null", "commit", "-m", "Synthetic selected project")
	runIn(scopedPolicy, scopedGit, "yolo", false, "/bin/cat", filepath.Join(unrelated, "private"))
	runIn(scopedPolicy, scopedGit, "full-auto", true, "/usr/bin/touch", filepath.Join(scopedGit, "workspace-write"))
	runIn(scopedPolicy, scopedGit, "full-auto", false, "/usr/bin/touch", filepath.Join(desktop, "sibling-write"))
	subdir := filepath.Join(gitProject, "src")
	if err := os.MkdirAll(subdir, 0o700); err != nil {
		t.Fatal(err)
	}
	subdirPolicy, err := buildCodexPrivacyPolicy(subdir, []string{desktop}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "subdir.txt"), []byte("synthetic subdir\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runIn(subdirPolicy, subdir, "yolo", true, git, "add", "subdir.txt")
	runIn(subdirPolicy, subdir, "yolo", true, git, "-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null", "commit", "-m", "Synthetic subdir regression")
	linked := filepath.Join(root, "linked-project")
	runIn(gitPolicy, gitProject, "yolo", true, git, "-c", "core.hooksPath=/dev/null", "worktree", "add", "-b", "synthetic-linked", linked)
	linkedPolicy, err := buildCodexPrivacyPolicy(linked, []string{desktop}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(linked, "linked.txt"), []byte("synthetic linked worktree\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runIn(linkedPolicy, linked, "yolo", true, git, "add", "linked.txt")
	runIn(linkedPolicy, linked, "yolo", true, git, "-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null", "commit", "-m", "Synthetic linked worktree regression")

	protectedGit := filepath.Join(desktop, "protected-git")
	if err := os.MkdirAll(protectedGit, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(protectedGit, "secret"), []byte("synthetic secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"symlink", "pointer"} {
		unsafeProject := filepath.Join(root, "unsafe-"+kind)
		if err := os.MkdirAll(unsafeProject, 0o700); err != nil {
			t.Fatal(err)
		}
		if kind == "symlink" {
			err = os.Symlink(protectedGit, filepath.Join(unsafeProject, ".git"))
		} else {
			err = os.WriteFile(filepath.Join(unsafeProject, ".git"), []byte("gitdir: "+protectedGit+"\n"), 0o600)
		}
		if err != nil {
			t.Fatal(err)
		}
		unsafePolicy, err := buildCodexPrivacyPolicy(unsafeProject, []string{desktop}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runIn(unsafePolicy, unsafeProject, "yolo", false, "/bin/cat", filepath.Join(protectedGit, "secret"))
		runIn(unsafePolicy, unsafeProject, "yolo", false, "/usr/bin/touch", filepath.Join(protectedGit, "must-not-write"))
		if kind == "symlink" {
			runIn(unsafePolicy, unsafeProject, "yolo", false, "/bin/cat", filepath.Join(unsafeProject, ".git", "secret"))
		}
	}
	// Only a synthetic loopback server is contacted; no auth or model API.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("synthetic-network"))
	}))
	defer server.Close()
	for _, mode := range []string{"suggest", "full-auto", "yolo"} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		args := append([]string{"sandbox"}, p.launchArgs(mode, "never")...)
		args = append(args, "--", "/usr/bin/curl", "--noproxy", "*", "--max-time", "2", "--silent", "--show-error", server.URL)
		cmd := exec.CommandContext(ctx, bin, args...)
		cmd.Dir = project
		cmd.Env = append(append([]string(nil), inheritedEnv...), env...)
		output, err := cmd.CombinedOutput()
		cancel()
		if (err == nil) != (mode == "yolo") || (mode == "yolo" && !strings.Contains(string(output), "synthetic-network")) {
			t.Fatalf("%s native network policy: %v, %s", mode, err, output)
		}
	}
	if err := checkExecPrivacyWorkspaceConfig(context.Background(), bin, nil, gitProject, env); err != nil {
		t.Fatalf("ordinary effective workspace config rejected: %v", err)
	}
	for _, override := range []string{
		"exclude_tmpdir_env_var=true", "exclude_slash_tmp=true", "network_access=true",
		"writable_roots=[" + tomlString(allowed) + "]",
	} {
		if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(config+"\n[sandbox_workspace_write]\n"+override+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		err := checkExecPrivacyWorkspaceConfig(context.Background(), bin, nil, gitProject, env)
		key, _, _ := strings.Cut(override, "=")
		if err == nil || !strings.Contains(err.Error(), "sandbox_workspace_write."+key) {
			t.Fatalf("effective %s not refused: %v", override, err)
		}
	}
}

func TestPrivacyWorkspaceConfig_RejectsUnsupportedEffectiveOverrides(t *testing.T) {
	for _, tc := range []struct {
		config string
		field  string
	}{
		{`{}`, ""},
		{`{"sandbox_workspace_write":{"writable_roots":[],"network_access":false,"exclude_tmpdir_env_var":false,"exclude_slash_tmp":false}}`, ""},
		{`{"sandbox_workspace_write":{"exclude_tmpdir_env_var":true}}`, "exclude_tmpdir_env_var"},
		{`{"sandbox_workspace_write":{"exclude_slash_tmp":true}}`, "exclude_slash_tmp"},
		{`{"sandbox_workspace_write":{"network_access":true}}`, "network_access"},
		{`{"sandbox_workspace_write":{"writable_roots":["/synthetic/extra"]}}`, "writable_roots"},
		{`{"sandbox_workspace_write":{"future_restriction":true}}`, "future_restriction"},
	} {
		err := checkPrivacyWorkspaceConfig(func(method string, params, out any) error {
			if method != "config/read" || params.(map[string]any)["cwd"] == "" {
				t.Fatalf("must resolve effective cwd config, got %s %v", method, params)
			}
			return json.Unmarshal([]byte(`{"config":`+tc.config+`}`), out)
		}, t.TempDir())
		if tc.field == "" {
			if err != nil {
				t.Fatalf("default config refused: %v", err)
			}
		} else if err == nil || !strings.Contains(err.Error(), tc.field) {
			t.Fatalf("unsupported override accepted: %s, %v", tc.config, err)
		}
	}
	err := checkPrivacyWorkspaceConfig(func(string, any, any) error {
		return fmt.Errorf("synthetic config read failure")
	}, t.TempDir())
	if err == nil {
		t.Fatal("failed effective config lookup must not continue unguarded")
	}
}
