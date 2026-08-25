package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	ccconfig "github.com/timmyagentic/cc-connect-next/config"
	"github.com/timmyagentic/cc-connect-next/core"
)

type migrationDirectStub struct {
	userID  string
	content string
}

func (*migrationDirectStub) Name() string                             { return "migration-direct-test" }
func (*migrationDirectStub) Start(core.MessageHandler) error          { return nil }
func (*migrationDirectStub) Stop() error                              { return nil }
func (*migrationDirectStub) Reply(context.Context, any, string) error { return nil }
func (*migrationDirectStub) Send(context.Context, any, string) error  { return nil }
func (p *migrationDirectStub) SendDirectUser(_ context.Context, userID, content string) error {
	p.userID, p.content = userID, content
	return nil
}

func TestResolveMigrationNotifyTargetUsesUniqueOperator(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "config.toml")
	writeMigrationFixture(t, configPath, `language = "zh"
data_dir = "`+filepath.ToSlash(root)+`"
[[projects]]
name = "alpha"
admin_from = "ou_owner"
[projects.agent]
type = "codex"
[projects.agent.options]
work_dir = "`+filepath.ToSlash(root)+`"
[[projects.platforms]]
type = "feishu"
[projects.platforms.options]
app_id = "cli_test"
app_secret = "secret"
allow_from = "ou_owner"
`)

	target, reason, err := resolveMigrationNotifyTarget(configPath, migrationNotifyHints{})
	if err != nil {
		t.Fatal(err)
	}
	if reason != "" || target == nil {
		t.Fatalf("target = %+v, reason = %q", target, reason)
	}
	if target.ProjectName != "alpha" || target.Platform.Type != "feishu" || target.UserID != "ou_owner" {
		t.Fatalf("target = %+v", target)
	}
	if target.Message != "迁移完成，cc-connect-next 已运行" {
		t.Fatalf("message = %q", target.Message)
	}
}

func TestResolveMigrationNotifyTargetDoesNotGuess(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "config.toml")
	writeMigrationFixture(t, configPath, `data_dir = "`+filepath.ToSlash(root)+`"
[[projects]]
name = "alpha"
[projects.agent]
type = "codex"
[projects.agent.options]
work_dir = "`+filepath.ToSlash(root)+`"
[[projects.platforms]]
type = "feishu"
[projects.platforms.options]
app_id = "cli_a"
app_secret = "secret"
allow_from = "ou_a"
[[projects.platforms]]
type = "lark"
[projects.platforms.options]
app_id = "cli_b"
app_secret = "secret"
allow_from = "ou_b"
`)
	target, reason, err := resolveMigrationNotifyTarget(configPath, migrationNotifyHints{})
	if err != nil {
		t.Fatal(err)
	}
	if target != nil || !strings.Contains(reason, "ambiguous") {
		t.Fatalf("target = %+v, reason = %q", target, reason)
	}
}

func TestSendMigrationCompleteUsesPlatformCapability(t *testing.T) {
	const platformName = "migration-direct-test"
	var stub *migrationDirectStub
	core.RegisterPlatform(platformName, func(map[string]any) (core.Platform, error) {
		stub = &migrationDirectStub{}
		return stub, nil
	})
	target := &migrationNotifyTarget{
		ProjectName: "alpha",
		Platform:    ccconfig.PlatformConfig{Type: platformName},
		UserID:      "ou_owner",
		Message:     "迁移完成，cc-connect-next 已运行",
	}
	if err := sendMigrationComplete(context.Background(), target, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if stub.userID != target.UserID || stub.content != target.Message {
		t.Fatalf("sent user/content = %q / %q", stub.userID, stub.content)
	}
}

func TestRunMigrateCommandRejectsAgentSwitchBeforeMutation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CC_SESSION_KEY", "feishu:chat:user")
	var stdout, stderr bytes.Buffer
	if code := runMigrateCommand([]string{"--switch"}, &stdout, &stderr); code != 2 {
		t.Fatalf("code = %d, stderr = %q", code, stderr.String())
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "external terminal") {
		t.Fatalf("stdout/stderr = %q / %q", stdout.String(), stderr.String())
	}
}
