package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"unicode"

	ccconfig "github.com/timmyagentic/cc-connect-next/config"
)

const larkCLIProfilePrefix = "cc-connect-next-"

type larkCLITarget struct {
	ProjectName   string
	PlatformType  string
	PlatformIndex int
	AppID         string
	AppSecret     string
}

type larkCLISetupOptions struct {
	InstallIfMissing bool
	ProfileName      string
}

type larkCLISetupResult struct {
	ProfileName     string
	PreviousProfile string
	Installed       bool
	Reused          bool
}

type larkCLIProfile struct {
	Name      string `json:"name"`
	AppID     string `json:"appId"`
	Brand     string `json:"brand"`
	Active    bool   `json:"active"`
	Effective bool   `json:"effective"`
}

type larkCLIWhoAmI struct {
	Profile     string `json:"profile"`
	AppID       string `json:"appId"`
	Identity    string `json:"identity"`
	Available   bool   `json:"available"`
	TokenStatus string `json:"tokenStatus"`
}

type larkCLIProcess interface {
	LookPath(string) (string, error)
	Run(context.Context, string, []string, string, ...string) (stdout, stderr string, err error)
}

type realLarkCLIProcess struct{}

func (realLarkCLIProcess) LookPath(name string) (string, error) {
	return exec.LookPath(name)
}

func (realLarkCLIProcess) Run(ctx context.Context, name string, args []string, stdin string, secrets ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(filterLarkCLIChildEnv(os.Environ(), secrets...),
		"LARKSUITE_CLI_NO_UPDATE_NOTIFIER=1",
		"LARKSUITE_CLI_NO_SKILLS_NOTIFIER=1",
	)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func filterLarkCLIChildEnv(base []string, secrets ...string) []string {
	filtered := make([]string, 0, len(base))
	for _, entry := range base {
		_, value, _ := strings.Cut(entry, "=")
		sensitive := false
		for _, secret := range secrets {
			secret = strings.TrimSpace(secret)
			if secret != "" && strings.Contains(value, secret) {
				sensitive = true
				break
			}
		}
		if !sensitive {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func resolveLarkCLITarget(configPath, projectName string, platformIndex int) (larkCLITarget, error) {
	if platformIndex < 0 {
		return larkCLITarget{}, fmt.Errorf("--platform-index must be >= 0")
	}
	cfg, err := ccconfig.Load(configPath)
	if err != nil {
		return larkCLITarget{}, fmt.Errorf("load config %s: %w", configPath, err)
	}
	projectName = strings.TrimSpace(projectName)
	candidates, projectFound := collectLarkCLITargetCandidates(cfg, projectName)
	if projectName != "" && !projectFound {
		return larkCLITarget{}, fmt.Errorf("project %q not found", projectName)
	}
	if len(candidates) == 0 {
		if projectName != "" {
			return larkCLITarget{}, fmt.Errorf("project %q has no Feishu/Lark platform", projectName)
		}
		return larkCLITarget{}, errors.New("no Feishu/Lark platform is configured")
	}

	var target larkCLITarget
	if projectName == "" && platformIndex > 0 {
		return larkCLITarget{}, errors.New("--platform-index is scoped to one project; use --project as well")
	}
	if projectName == "" {
		uniqueAppIDs := make(map[string]struct{})
		for _, candidate := range candidates {
			uniqueAppIDs[candidate.AppID] = struct{}{}
		}
		if len(uniqueAppIDs) != 1 {
			projects := make([]string, 0, len(candidates))
			for _, candidate := range candidates {
				if !slices.Contains(projects, candidate.ProjectName) {
					projects = append(projects, candidate.ProjectName)
				}
			}
			return larkCLITarget{}, fmt.Errorf("multiple Feishu/Lark bots are configured for projects %s; use --project", strings.Join(projects, ", "))
		}
		target = candidates[0]
	} else if platformIndex > 0 {
		for _, candidate := range candidates {
			if candidate.PlatformIndex == platformIndex {
				target = candidate
				break
			}
		}
		if target.ProjectName == "" {
			return larkCLITarget{}, fmt.Errorf("--platform-index %d is out of range for project %q", platformIndex, projectName)
		}
	} else {
		if len(candidates) != 1 {
			return larkCLITarget{}, fmt.Errorf("project %q has %d Feishu/Lark platforms; use --platform-index", projectName, len(candidates))
		}
		target = candidates[0]
	}

	if target.AppID == "" || target.AppSecret == "" {
		return larkCLITarget{}, fmt.Errorf("project %q Feishu/Lark platform %d is missing app_id/app_secret; run cc-connect-next feishu setup first", target.ProjectName, target.PlatformIndex)
	}
	return target, nil
}

func collectLarkCLITargetCandidates(cfg *ccconfig.Config, projectName string) ([]larkCLITarget, bool) {
	var candidates []larkCLITarget
	projectFound := projectName == ""
	for _, project := range cfg.Projects {
		if projectName != "" && project.Name != projectName {
			continue
		}
		projectFound = true
		familyIndex := 0
		for _, platform := range project.Platforms {
			if platform.Type != "feishu" && platform.Type != "lark" {
				continue
			}
			familyIndex++
			appID, _ := platform.Options["app_id"].(string)
			appSecret, _ := platform.Options["app_secret"].(string)
			candidates = append(candidates, larkCLITarget{
				ProjectName:   project.Name,
				PlatformType:  platform.Type,
				PlatformIndex: familyIndex,
				AppID:         strings.TrimSpace(appID),
				AppSecret:     strings.TrimSpace(appSecret),
			})
		}
	}
	return candidates, projectFound
}

func setupLarkCLICompanion(ctx context.Context, target larkCLITarget, opts larkCLISetupOptions, process larkCLIProcess) (larkCLISetupResult, error) {
	if process == nil {
		return larkCLISetupResult{}, errors.New("lark-cli process runner is unavailable")
	}
	if strings.TrimSpace(target.AppID) == "" || strings.TrimSpace(target.AppSecret) == "" {
		return larkCLISetupResult{}, errors.New("app_id/app_secret are required")
	}

	binary, err := process.LookPath("lark-cli")
	installed := false
	if err != nil {
		if !opts.InstallIfMissing {
			return larkCLISetupResult{}, errors.New("lark-cli is not installed; rerun with installation enabled")
		}
		_, stderr, installErr := process.Run(ctx, "npx", []string{"@larksuite/cli@latest", "install"}, "", target.AppSecret)
		if installErr != nil {
			return larkCLISetupResult{}, larkCLICommandError("install", stderr, installErr, target.AppSecret)
		}
		binary, err = process.LookPath("lark-cli")
		if err != nil {
			return larkCLISetupResult{}, errors.New("official lark-cli installer completed, but lark-cli is still not on PATH; open a new terminal and rerun cc-connect-next lark-cli setup")
		}
		installed = true
	}

	stdout, stderr, err := process.Run(ctx, binary, []string{"profile", "list"}, "", target.AppSecret)
	if err != nil {
		return larkCLISetupResult{}, larkCLICommandError("profile list", stderr, err, target.AppSecret)
	}
	profiles, err := decodeLarkCLIProfiles(stdout)
	if err != nil {
		return larkCLISetupResult{}, fmt.Errorf("decode lark-cli profiles: %w", err)
	}

	profileName, reused := chooseLarkCLIProfileName(profiles, target, opts.ProfileName)
	previousProfile := activeLarkCLIProfileName(profiles)
	if previousProfile == profileName {
		previousProfile = ""
	}

	if !reused {
		args := []string{
			"profile", "add",
			"--name", profileName,
			"--app-id", target.AppID,
			"--app-secret-stdin",
			"--brand", target.PlatformType,
			"--use",
		}
		_, stderr, err = process.Run(ctx, binary, args, target.AppSecret+"\n", target.AppSecret)
		if err != nil {
			return larkCLISetupResult{}, larkCLICommandError("profile add", stderr, err, target.AppSecret)
		}
	} else if !larkCLIProfileIsActive(profiles, profileName) {
		_, stderr, err = process.Run(ctx, binary, []string{"profile", "use", profileName}, "", target.AppSecret)
		if err != nil {
			return larkCLISetupResult{}, larkCLICommandError("profile use", stderr, err, target.AppSecret)
		}
	}

	_, stderr, err = process.Run(ctx, binary, []string{"--profile", profileName, "config", "default-as", "bot"}, "", target.AppSecret)
	if err != nil {
		return larkCLISetupResult{}, larkCLICommandError("set default identity", stderr, err, target.AppSecret)
	}

	stdout, stderr, err = process.Run(ctx, binary, []string{"--profile", profileName, "whoami", "--as", "bot"}, "", target.AppSecret)
	if err != nil {
		return larkCLISetupResult{}, larkCLICommandError("verify bot identity", stderr, err, target.AppSecret)
	}
	if err := verifyLarkCLIBotIdentity(stdout, profileName, target.AppID); err != nil {
		return larkCLISetupResult{}, err
	}

	return larkCLISetupResult{
		ProfileName:     profileName,
		PreviousProfile: previousProfile,
		Installed:       installed,
		Reused:          reused,
	}, nil
}

func decodeLarkCLIProfiles(raw string) ([]larkCLIProfile, error) {
	raw = strings.TrimSpace(raw)
	var profiles []larkCLIProfile
	if err := json.Unmarshal([]byte(raw), &profiles); err == nil {
		return profiles, nil
	}

	var envelope struct {
		OK   bool            `json:"ok"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil || !envelope.OK {
		return nil, errors.New("unexpected JSON output")
	}
	if err := json.Unmarshal(envelope.Data, &profiles); err == nil {
		return profiles, nil
	}
	var data struct {
		Profiles []larkCLIProfile `json:"profiles"`
	}
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		return nil, errors.New("success output did not contain profiles")
	}
	return data.Profiles, nil
}

func chooseLarkCLIProfileName(profiles []larkCLIProfile, target larkCLITarget, requested string) (string, bool) {
	for _, profile := range profiles {
		if profile.AppID == target.AppID && (profile.Active || profile.Effective) {
			return profile.Name, true
		}
	}
	for _, profile := range profiles {
		if profile.AppID == target.AppID {
			return profile.Name, true
		}
	}

	name := strings.TrimSpace(requested)
	if name == "" {
		projectSlug := larkCLIProfileSlug(target.ProjectName)
		if projectSlug == "" {
			projectSlug = "bot"
		}
		name = larkCLIProfilePrefix + projectSlug
		if target.PlatformIndex > 1 {
			name += "-" + strconv.Itoa(target.PlatformIndex)
		}
	}
	if !larkCLIProfileNameExists(profiles, name) {
		return name, false
	}

	suffix := larkCLIProfileSlug(target.AppID)
	if suffix == "" {
		suffix = "bot"
	}
	candidate := name + "-" + suffix
	for i := 2; larkCLIProfileNameExists(profiles, candidate); i++ {
		candidate = name + "-" + suffix + "-" + strconv.Itoa(i)
	}
	return candidate, false
}

func larkCLIProfileSlug(raw string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.TrimSpace(raw) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
			lastDash = false
		case !lastDash && b.Len() > 0:
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func larkCLIProfileNameExists(profiles []larkCLIProfile, name string) bool {
	for _, profile := range profiles {
		if profile.Name == name {
			return true
		}
	}
	return false
}

func activeLarkCLIProfileName(profiles []larkCLIProfile) string {
	for _, profile := range profiles {
		if profile.Effective {
			return profile.Name
		}
	}
	for _, profile := range profiles {
		if profile.Active {
			return profile.Name
		}
	}
	return ""
}

func larkCLIProfileIsActive(profiles []larkCLIProfile, name string) bool {
	for _, profile := range profiles {
		if profile.Name == name {
			return profile.Active || profile.Effective
		}
	}
	return false
}

func verifyLarkCLIBotIdentity(raw, profileName, appID string) error {
	var identity larkCLIWhoAmI
	trimmed := []byte(strings.TrimSpace(raw))
	if err := json.Unmarshal(trimmed, &identity); err != nil {
		return errors.New("lark-cli whoami returned unexpected JSON")
	}
	if identity.Profile == "" && identity.AppID == "" {
		var envelope struct {
			OK   bool            `json:"ok"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(trimmed, &envelope); err != nil || !envelope.OK {
			return errors.New("lark-cli whoami returned unexpected JSON")
		}
		if err := json.Unmarshal(envelope.Data, &identity); err != nil {
			return errors.New("lark-cli whoami success output did not contain identity")
		}
	}
	if identity.Profile != profileName || identity.AppID != appID || identity.Identity != "bot" || !identity.Available {
		return fmt.Errorf("lark-cli bot verification mismatch: profile=%q app_id=%q identity=%q available=%t", identity.Profile, identity.AppID, identity.Identity, identity.Available)
	}
	return nil
}

func larkCLICommandError(operation, stderr string, err error, secrets ...string) error {
	detail := strings.TrimSpace(stderr)
	if detail == "" && err != nil {
		detail = err.Error()
	}
	for _, secret := range secrets {
		if secret != "" {
			detail = strings.ReplaceAll(detail, secret, "<redacted>")
		}
	}
	if detail == "" {
		detail = "command failed"
	}
	return fmt.Errorf("lark-cli %s: %s", operation, detail)
}

func resolveLarkCLICompanionFlags(yes, no bool) (decided, apply bool, err error) {
	if yes && no {
		return false, false, errors.New("--lark-cli and --no-lark-cli cannot be combined")
	}
	if yes {
		return true, true, nil
	}
	if no {
		return true, false, nil
	}
	return false, false, nil
}

func maybeSetupLarkCLICompanion(out io.Writer, in io.Reader, yes, no, interactive bool, target larkCLITarget, setup func(larkCLITarget) error) error {
	decided, apply, err := resolveLarkCLICompanionFlags(yes, no)
	if err != nil {
		return err
	}
	if decided && !apply {
		return nil
	}
	if !decided {
		if !interactive {
			_, _ = fmt.Fprintln(out, "ℹ️  未安装或绑定 lark-cli（stdin 非交互）。加 --lark-cli 启用，或稍后运行 `cc-connect-next lark-cli setup`。")
			return nil
		}
		_, _ = fmt.Fprintln(out, "可安装官方 lark-cli，并复用这个机器人创建独立 profile；它会成为默认 profile 与默认 bot 身份，原 profile 始终保留。")
		if !promptYesNo(in, out, "是否现在安装并默认绑定 lark-cli？", true) {
			_, _ = fmt.Fprintln(out, "已跳过。稍后可运行 `cc-connect-next lark-cli setup`。")
			return nil
		}
	}
	if setup == nil {
		return errors.New("lark-cli setup function is unavailable")
	}
	return setup(target)
}

func maybeSetupMigratedLarkCLI(
	out io.Writer,
	in io.Reader,
	yes, no, interactive, dryRun bool,
	configPath, projectName string,
	platformIndex int,
	setup func(larkCLITarget) error,
) error {
	decided, apply, err := resolveLarkCLICompanionFlags(yes, no)
	if err != nil {
		return err
	}
	if dryRun {
		if decided && apply {
			_, _ = fmt.Fprintln(out, "Would configure the official lark-cli companion after a real migration; dry-run made no lark-cli changes.")
		}
		return nil
	}
	if decided && !apply {
		return nil
	}
	if !decided {
		if !interactive {
			_, _ = fmt.Fprintln(out, "ℹ️  未安装或绑定 lark-cli（stdin 非交互）。加 --lark-cli 启用，或迁移后运行 `cc-connect-next lark-cli setup`。")
			return nil
		}
		_, _ = fmt.Fprintln(out, "迁移后的飞书机器人可以同时作为官方 lark-cli 的默认 bot profile；原 profile 始终保留。")
		if !promptYesNo(in, out, "是否现在安装并默认绑定 lark-cli？", true) {
			_, _ = fmt.Fprintln(out, "已跳过。稍后可运行 `cc-connect-next lark-cli setup`。")
			return nil
		}
	}

	target, err := resolveLarkCLITarget(configPath, projectName, platformIndex)
	if err != nil {
		return err
	}
	if setup == nil {
		return errors.New("lark-cli setup function is unavailable")
	}
	return setup(target)
}

func runLarkCLI(args []string) int {
	return runLarkCLICommand(args, os.Stdout, os.Stderr, realLarkCLIProcess{})
}

func runLarkCLICommand(args []string, stdout, stderr io.Writer, process larkCLIProcess) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printLarkCLIUsage(stdout)
		return 0
	}
	if args[0] != "setup" {
		_, _ = fmt.Fprintf(stderr, "unknown lark-cli subcommand %q\n\n", args[0])
		printLarkCLIUsage(stderr)
		return 2
	}

	fs := flag.NewFlagSet("lark-cli setup", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configFile := fs.String("config", "", "path to cc-connect-next config file")
	project := fs.String("project", "", "project whose Feishu/Lark bot should be bound")
	platformIndex := fs.Int("platform-index", 0, "1-based index among Feishu/Lark platforms in the project")
	profileName := fs.String("profile", "", "lark-cli profile name (default: cc-connect-next-<project>)")
	fs.Usage = func() { printLarkCLIUsage(stderr) }
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	target, err := resolveLarkCLITarget(resolveConfigPath(*configFile), *project, *platformIndex)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "lark-cli setup: %v\n", err)
		return 1
	}
	result, err := setupLarkCLICompanion(context.Background(), target, larkCLISetupOptions{
		InstallIfMissing: true,
		ProfileName:      *profileName,
	}, process)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "lark-cli setup: %v\n", err)
		return 1
	}
	reportLarkCLISetupResult(stdout, result)
	return 0
}

func reportLarkCLISetupResult(stdout io.Writer, result larkCLISetupResult) {
	if result.Installed {
		_, _ = fmt.Fprintln(stdout, "✅ 官方 lark-cli 已安装。")
	}
	if result.Reused {
		_, _ = fmt.Fprintf(stdout, "✅ 已复用同一飞书 App 的 lark-cli profile %q，并设为默认。\n", result.ProfileName)
	} else {
		_, _ = fmt.Fprintf(stdout, "✅ 已创建 lark-cli profile %q，并设为默认。\n", result.ProfileName)
	}
	_, _ = fmt.Fprintln(stdout, "   默认身份：bot；旧 profile 未删除。")
	if result.PreviousProfile != "" {
		_, _ = fmt.Fprintf(stdout, "   上一个默认 profile：%q（可用 `lark-cli profile use -` 切回）。\n", result.PreviousProfile)
	}
	_, _ = fmt.Fprintln(stdout, "⚠️  cc-connect-next 运行时，不要用这个 profile 执行 `lark-cli event consume`；第二条事件长连接会与机器人争抢事件。")
}

func printLarkCLIUsage(out io.Writer) {
	_, _ = fmt.Fprintln(out, `Usage: cc-connect-next lark-cli setup [options]

Install the official lark-cli when missing, reuse one configured Feishu/Lark
bot, create or reuse an isolated profile, make it the default profile, set its
default identity to bot, and verify that identity. Existing profiles and user
OAuth logins are preserved.

Options:
  --config <path>          Path to cc-connect-next config
  --project <name>         Required when different Feishu/Lark bots exist
  --platform-index <n>     Select one of multiple Feishu/Lark platforms
  --profile <name>         Override the generated profile name

This command never runs auth login, sends a real message, or starts event
consumption. Do not run lark-cli event consume with the same App while
cc-connect-next is running.`)
}
