package main

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type fakeLarkCLICall struct {
	name        string
	args        []string
	stdin       string
	hasDeadline bool
}

type fakeLarkCLIResult struct {
	stdout string
	stderr string
	err    error
}

type fakeLarkCLIProcess struct {
	lookPaths []string
	lookErrs  []error
	results   []fakeLarkCLIResult
	calls     []fakeLarkCLICall
}

func (f *fakeLarkCLIProcess) LookPath(string) (string, error) {
	if len(f.lookPaths) == 0 {
		return "", errors.New("unexpected LookPath")
	}
	path, err := f.lookPaths[0], f.lookErrs[0]
	f.lookPaths, f.lookErrs = f.lookPaths[1:], f.lookErrs[1:]
	return path, err
}

func (f *fakeLarkCLIProcess) Run(ctx context.Context, name string, args []string, stdin string, _ ...string) (string, string, error) {
	_, hasDeadline := ctx.Deadline()
	f.calls = append(f.calls, fakeLarkCLICall{name: name, args: slices.Clone(args), stdin: stdin, hasDeadline: hasDeadline})
	if len(f.results) == 0 {
		return "", "", errors.New("unexpected command")
	}
	result := f.results[0]
	f.results = f.results[1:]
	return result.stdout, result.stderr, result.err
}

func TestFilterLarkCLIChildEnvRemovesSecretBearingValues(t *testing.T) {
	const secret = "secret-value"
	got := filterLarkCLIChildEnv([]string{
		"PATH=/usr/bin:/bin",
		"HOME=/Users/demo",
		"FEISHU_APP_SECRET=" + secret,
		"CUSTOM_URL=https://example.test/?token=" + secret,
		"HTTPS_PROXY=http://127.0.0.1:7890",
	}, secret)
	joined := strings.Join(got, "\n")
	if strings.Contains(joined, secret) || strings.Contains(joined, "FEISHU_APP_SECRET") || strings.Contains(joined, "CUSTOM_URL") {
		t.Fatalf("secret-bearing environment survived: %q", joined)
	}
	for _, expected := range []string{"PATH=/usr/bin:/bin", "HOME=/Users/demo", "HTTPS_PROXY=http://127.0.0.1:7890"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("safe environment entry %q was removed: %q", expected, joined)
		}
	}
}

func TestVerifyLarkCLIBotIdentityAcceptsSuccessEnvelope(t *testing.T) {
	err := verifyLarkCLIBotIdentity(
		`{"ok":true,"identity":"bot","data":{"profile":"next","appId":"cli_next","identity":"bot","available":true,"tokenStatus":"ready"}}`,
		"next",
		"cli_next",
	)
	if err != nil {
		t.Fatal(err)
	}
}

func TestResolveLarkCLITargetRequiresProjectForDifferentBots(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "config.toml")
	writeMigrationFixture(t, configPath, `[[projects]]
name = "alpha"
[projects.agent]
type = "codex"
[[projects.platforms]]
type = "feishu"
[projects.platforms.options]
app_id = "cli_alpha"
app_secret = "secret-alpha"

[[projects]]
name = "beta"
[projects.agent]
type = "codex"
[[projects.platforms]]
type = "lark"
[projects.platforms.options]
app_id = "cli_beta"
app_secret = "secret-beta"
`)

	_, err := resolveLarkCLITarget(configPath, "", 0)
	if err == nil || !strings.Contains(err.Error(), "--project") {
		t.Fatalf("resolveLarkCLITarget() error = %v, want explicit project guidance", err)
	}
	_, err = resolveLarkCLITarget(configPath, "", 1)
	if err == nil || !strings.Contains(err.Error(), "--project") {
		t.Fatalf("platform index bypassed project ambiguity: %v", err)
	}

	target, err := resolveLarkCLITarget(configPath, "beta", 0)
	if err != nil {
		t.Fatal(err)
	}
	if target.ProjectName != "beta" || target.PlatformType != "lark" || target.AppID != "cli_beta" || target.AppSecret != "secret-beta" {
		t.Fatalf("target = %+v", target)
	}
}

func TestSetupLarkCLICompanionCreatesDefaultBotProfileWithoutSecretInArgs(t *testing.T) {
	const secret = "super-secret-that-must-stay-on-stdin"
	process := &fakeLarkCLIProcess{
		lookPaths: []string{"/usr/local/bin/lark-cli"},
		lookErrs:  []error{nil},
		results: []fakeLarkCLIResult{
			{stdout: `[{"name":"personal","appId":"cli_personal","brand":"feishu","active":true,"effective":true}]`},
			{},
			{},
			{stdout: `{"profile":"cc-connect-next-alpha","appId":"cli_alpha","identity":"bot","available":true,"tokenStatus":"ready"}`},
			{},
		},
	}

	result, err := setupLarkCLICompanion(context.Background(), larkCLITarget{
		ProjectName: "alpha", PlatformType: "feishu", AppID: "cli_alpha", AppSecret: secret,
	}, larkCLISetupOptions{InstallIfMissing: true}, process)
	if err != nil {
		t.Fatal(err)
	}
	if result.ProfileName != "cc-connect-next-alpha" || result.PreviousProfile != "personal" || result.Reused {
		t.Fatalf("result = %+v", result)
	}
	if len(process.calls) != 5 {
		t.Fatalf("calls = %#v", process.calls)
	}
	for _, call := range process.calls {
		if !call.hasDeadline {
			t.Fatalf("lark-cli call had no deadline: %#v", call)
		}
		if strings.Contains(strings.Join(call.args, " "), secret) || strings.Contains(call.name, secret) {
			t.Fatalf("secret leaked into argv: %#v", call)
		}
	}
	if got := process.calls[1]; got.stdin != secret+"\n" || !slices.Contains(got.args, "--app-secret-stdin") || slices.Contains(got.args, "--use") {
		t.Fatalf("profile add call = %#v", got)
	}
	if got := process.calls[2]; !slices.Equal(got.args, []string{"--profile", "cc-connect-next-alpha", "config", "default-as", "bot"}) {
		t.Fatalf("default identity call = %#v", got)
	}
	if got := process.calls[3]; !slices.Equal(got.args, []string{"--profile", "cc-connect-next-alpha", "whoami", "--as", "bot"}) {
		t.Fatalf("verification call = %#v", got)
	}
	if got := process.calls[4]; !slices.Equal(got.args, []string{"profile", "use", "cc-connect-next-alpha"}) {
		t.Fatalf("profile use call = %#v", got)
	}
}

func TestSetupLarkCLICompanionReusesSameAppWithoutRewritingSecret(t *testing.T) {
	process := &fakeLarkCLIProcess{
		lookPaths: []string{"/usr/local/bin/lark-cli"},
		lookErrs:  []error{nil},
		results: []fakeLarkCLIResult{
			{stdout: `[{"name":"shared-bot","appId":"cli_alpha","brand":"feishu","active":true,"effective":true}]`},
			{},
			{stdout: `{"profile":"shared-bot","appId":"cli_alpha","identity":"bot","available":true,"tokenStatus":"ready"}`},
		},
	}

	result, err := setupLarkCLICompanion(context.Background(), larkCLITarget{
		ProjectName: "alpha", PlatformType: "feishu", AppID: "cli_alpha", AppSecret: "must-not-be-used",
	}, larkCLISetupOptions{InstallIfMissing: true}, process)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Reused || result.ProfileName != "shared-bot" {
		t.Fatalf("result = %+v", result)
	}
	for _, call := range process.calls {
		if call.stdin != "" || slices.Contains(call.args, "profile") && slices.Contains(call.args, "add") {
			t.Fatalf("existing profile was rewritten: %#v", call)
		}
	}
}

func TestSetupLarkCLICompanionMakesExistingSameAppProfileDefault(t *testing.T) {
	process := &fakeLarkCLIProcess{
		lookPaths: []string{"/usr/local/bin/lark-cli"},
		lookErrs:  []error{nil},
		results: []fakeLarkCLIResult{
			{stdout: `[{"name":"personal","appId":"cli_personal","active":true,"effective":true},{"name":"next-bot","appId":"cli_alpha"}]`},
			{},
			{stdout: `{"profile":"next-bot","appId":"cli_alpha","identity":"bot","available":true,"tokenStatus":"ready"}`},
			{},
		},
	}

	result, err := setupLarkCLICompanion(context.Background(), larkCLITarget{
		ProjectName: "alpha", PlatformType: "feishu", AppID: "cli_alpha", AppSecret: "unused",
	}, larkCLISetupOptions{InstallIfMissing: true}, process)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Reused || result.ProfileName != "next-bot" || result.PreviousProfile != "personal" {
		t.Fatalf("result = %+v", result)
	}
	if got := process.calls[3]; !slices.Equal(got.args, []string{"profile", "use", "next-bot"}) {
		t.Fatalf("profile use call = %#v", got)
	}
}

func TestSetupLarkCLICompanionDoesNotOverwriteSameNameForDifferentApp(t *testing.T) {
	process := &fakeLarkCLIProcess{
		lookPaths: []string{"/usr/local/bin/lark-cli"},
		lookErrs:  []error{nil},
		results: []fakeLarkCLIResult{
			{stdout: `[{"name":"cc-connect-next-alpha","appId":"cli_other","active":true,"effective":true}]`},
			{},
			{},
			{stdout: `{"profile":"cc-connect-next-alpha-cli-alpha","appId":"cli_alpha","identity":"bot","available":true,"tokenStatus":"ready"}`},
			{},
		},
	}

	result, err := setupLarkCLICompanion(context.Background(), larkCLITarget{
		ProjectName: "alpha", PlatformType: "feishu", AppID: "cli_alpha", AppSecret: "secret",
	}, larkCLISetupOptions{InstallIfMissing: true}, process)
	if err != nil {
		t.Fatal(err)
	}
	if result.ProfileName == "cc-connect-next-alpha" || !strings.HasPrefix(result.ProfileName, "cc-connect-next-alpha-") {
		t.Fatalf("collision was not isolated: %+v", result)
	}
	if got := process.calls[1]; !slices.Contains(got.args, result.ProfileName) {
		t.Fatalf("profile add did not use isolated name: %#v", got)
	}
}

func TestSetupLarkCLICompanionInstallsOnlyWhenExplicitlyAllowed(t *testing.T) {
	missing := errors.New("not found")
	process := &fakeLarkCLIProcess{
		lookPaths: []string{"", "/opt/bin/lark-cli"},
		lookErrs:  []error{missing, nil},
		results: []fakeLarkCLIResult{
			{},
			{stdout: `[]`},
			{},
			{},
			{stdout: `{"profile":"cc-connect-next-alpha","appId":"cli_alpha","identity":"bot","available":true,"tokenStatus":"ready"}`},
			{},
		},
	}

	result, err := setupLarkCLICompanion(context.Background(), larkCLITarget{
		ProjectName: "alpha", PlatformType: "feishu", AppID: "cli_alpha", AppSecret: "secret",
	}, larkCLISetupOptions{InstallIfMissing: true}, process)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Installed || process.calls[0].name != "npx" || !slices.Equal(process.calls[0].args, []string{"@larksuite/cli@latest", "install"}) {
		t.Fatalf("install result/call = %+v / %#v", result, process.calls[0])
	}
}

func TestSetupLarkCLICompanionRedactsSecretFromCommandFailure(t *testing.T) {
	const secret = "redact-this-secret"
	process := &fakeLarkCLIProcess{
		lookPaths: []string{"/usr/local/bin/lark-cli"},
		lookErrs:  []error{nil},
		results: []fakeLarkCLIResult{
			{stdout: `[]`},
			{stderr: "rejected " + secret, err: errors.New("exit status 1")},
		},
	}

	_, err := setupLarkCLICompanion(context.Background(), larkCLITarget{
		ProjectName: "alpha", PlatformType: "feishu", AppID: "cli_alpha", AppSecret: secret,
	}, larkCLISetupOptions{InstallIfMissing: true}, process)
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("error = %v, want redacted failure", err)
	}
}

func TestSetupLarkCLICompanionDoesNotSwitchBeforeBotVerification(t *testing.T) {
	process := &fakeLarkCLIProcess{
		lookPaths: []string{"/usr/local/bin/lark-cli"},
		lookErrs:  []error{nil},
		results: []fakeLarkCLIResult{
			{stdout: `[{"name":"personal","appId":"cli_personal","active":true,"effective":true}]`},
			{},
			{},
			{stdout: `{"profile":"cc-connect-next-alpha","appId":"cli_alpha","identity":"bot","available":false,"tokenStatus":"missing"}`},
		},
	}

	_, err := setupLarkCLICompanion(context.Background(), larkCLITarget{
		ProjectName: "alpha", PlatformType: "feishu", AppID: "cli_alpha", AppSecret: "secret",
	}, larkCLISetupOptions{InstallIfMissing: true}, process)
	if err == nil {
		t.Fatal("verification failure was accepted")
	}
	for _, call := range process.calls {
		if slices.Contains(call.args, "--use") || slices.Equal(call.args, []string{"profile", "use", "cc-connect-next-alpha"}) {
			t.Fatalf("profile switched before successful verification: %#v", call)
		}
	}
}

func TestMaybeSetupLarkCLICompanionNonInteractiveNeedsExplicitFlag(t *testing.T) {
	called := false
	var out bytes.Buffer
	err := maybeSetupLarkCLICompanion(
		&out,
		strings.NewReader(""),
		false,
		false,
		false,
		larkCLITarget{ProjectName: "alpha"},
		func(larkCLITarget) error { called = true; return nil },
	)
	if err != nil {
		t.Fatal(err)
	}
	if called || !strings.Contains(out.String(), "--lark-cli") {
		t.Fatalf("called=%v output=%q", called, out.String())
	}
}

func TestMaybeSetupLarkCLICompanionExplicitFlagRunsNonInteractive(t *testing.T) {
	called := false
	var out bytes.Buffer
	err := maybeSetupLarkCLICompanion(
		&out,
		strings.NewReader(""),
		true,
		false,
		false,
		larkCLITarget{ProjectName: "alpha"},
		func(target larkCLITarget) error {
			called = target.ProjectName == "alpha"
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("explicit --lark-cli did not run setup")
	}
}

func TestResolveLarkCLICompanionFlagsRejectsConflict(t *testing.T) {
	if _, _, err := resolveLarkCLICompanionFlags(true, true); err == nil {
		t.Fatal("conflicting --lark-cli/--no-lark-cli flags were accepted")
	}
}

func TestPrintUsageListsLarkCLICompanion(t *testing.T) {
	out := captureStderr(t, printUsage)
	if !strings.Contains(out, "lark-cli") || !strings.Contains(out, "default bot profile") {
		t.Fatalf("printUsage() output missing lark-cli companion:\n%s", out)
	}
}

func TestMaybeSetupMigratedLarkCLIDryRunNeverWrites(t *testing.T) {
	called := false
	var out bytes.Buffer
	err := maybeSetupMigratedLarkCLI(
		&out,
		strings.NewReader(""),
		true,
		false,
		false,
		true,
		"/tmp/config.toml",
		"alpha",
		0,
		func(larkCLITarget) error { called = true; return nil },
	)
	if err != nil {
		t.Fatal(err)
	}
	if called || !strings.Contains(out.String(), "Would configure") {
		t.Fatalf("called=%v output=%q", called, out.String())
	}
}

func TestMigrateHelpListsLarkCLICompanionFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := runMigrateCommand([]string{"--help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("code = %d", code)
	}
	for _, flag := range []string{"--lark-cli", "--no-lark-cli", "--lark-cli-project", "--lark-cli-platform-index"} {
		if !strings.Contains(stderr.String(), flag) {
			t.Fatalf("migrate help missing %s:\n%s", flag, stderr.String())
		}
	}
}

func TestRunLarkCLICommandBindsConfiguredBot(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "config.toml")
	writeMigrationFixture(t, configPath, `[[projects]]
name = "alpha"
[projects.agent]
type = "codex"
[projects.agent.options]
work_dir = "`+filepath.ToSlash(root)+`"
[[projects.platforms]]
type = "feishu"
[projects.platforms.options]
app_id = "cli_alpha"
app_secret = "secret-alpha"
`)
	process := &fakeLarkCLIProcess{
		lookPaths: []string{"/usr/local/bin/lark-cli"},
		lookErrs:  []error{nil},
		results: []fakeLarkCLIResult{
			{stdout: `[]`},
			{},
			{},
			{stdout: `{"profile":"cc-connect-next-alpha","appId":"cli_alpha","identity":"bot","available":true,"tokenStatus":"ready"}`},
			{},
		},
	}
	var stdout, stderr bytes.Buffer
	code := runLarkCLICommand([]string{"setup", "--config", configPath}, &stdout, &stderr, process)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "设为默认") || !strings.Contains(stdout.String(), "不要用这个 profile") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunLarkCLISetupRejectsUnexpectedPositionalArgument(t *testing.T) {
	process := &fakeLarkCLIProcess{}
	var stdout, stderr bytes.Buffer
	if code := runLarkCLICommand([]string{"setup", "unexpected"}, &stdout, &stderr, process); code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if len(process.calls) != 0 || !strings.Contains(stderr.String(), "unexpected argument") {
		t.Fatalf("calls=%#v stderr=%q", process.calls, stderr.String())
	}
}

func TestRunLarkCLISetupHelpUsesCompanionUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := runLarkCLICommand([]string{"setup", "--help"}, &stdout, &stderr, nil); code != 0 {
		t.Fatalf("code = %d", code)
	}
	if !strings.Contains(stderr.String(), "Usage: cc-connect-next lark-cli setup") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
