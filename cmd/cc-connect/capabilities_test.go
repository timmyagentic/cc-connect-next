package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/timmyagentic/cc-connect-next/config"
	"github.com/timmyagentic/cc-connect-next/core"
)

type capabilityManifestTestAgent struct{}

func (*capabilityManifestTestAgent) Name() string { return "codex" }
func (*capabilityManifestTestAgent) StartSession(context.Context, string) (core.AgentSession, error) {
	return nil, nil
}
func (*capabilityManifestTestAgent) ListSessions(context.Context) ([]core.AgentSessionInfo, error) {
	return nil, nil
}
func (*capabilityManifestTestAgent) Stop() error { return nil }

type capabilityRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn capabilityRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestRunCapabilitiesCommandUsesSessionEnvironmentAndReturnsJSON(t *testing.T) {
	t.Setenv("CC_PROJECT", "demo")
	t.Setenv("CC_SESSION_KEY", "feishu:chat:user")
	var requestURL string
	factory := func(string) *http.Client {
		return &http.Client{Transport: capabilityRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			requestURL = req.URL.String()
			body := `{"schema":"cc-connect-next.agent-capabilities/v1","version":"v-test","project":"demo","read_only":true,"security_note":"safe","query":"模型","configuration":{"version":"v-test","capabilities":[],"options":[]},"tools":[],"commands":[{"id":"model","invocation":"/model","source":"builtin","category":"agent","usage":"/model","description":"Switch model","description_zh":"切换模型","permission":"member","read_only":false,"fallback":{"mode":"reject","description":"reject","description_zh":"拒绝"},"availability":{"state":"available","reason":"supported","reason_zh":"支持"}}],"skills":[],"runtime":[]}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		})}
	}
	var stdout, stderr bytes.Buffer
	code := runCapabilitiesCommand([]string{"--all", "--search", "模型", "--format", "json"}, &stdout, &stderr, factory)
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	for _, want := range []string{"all=true", "project=demo", "session_key=feishu%3Achat%3Auser", "search=%E6%A8%A1%E5%9E%8B"} {
		if !strings.Contains(requestURL, want) {
			t.Errorf("request URL %q missing %q", requestURL, want)
		}
	}
	if !strings.Contains(stdout.String(), core.AgentCapabilityManifestSchema) || !strings.Contains(stdout.String(), `"model"`) {
		t.Fatalf("stdout = %s", stdout.String())
	}
}

func TestRunCapabilitiesCommandHonestNoMatch(t *testing.T) {
	factory := func(string) *http.Client {
		return &http.Client{Transport: capabilityRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{"schema":"cc-connect-next.agent-capabilities/v1","version":"v-test","project":"demo","read_only":true,"security_note":"safe","query":"卡片主题颜色","configuration":{"version":"v-test","capabilities":[],"options":[]},"tools":[],"commands":[],"skills":[],"runtime":[]}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		})}
	}
	var stdout, stderr bytes.Buffer
	code := runCapabilitiesCommand([]string{"--search", "卡片主题颜色", "--lang", "zh"}, &stdout, &stderr, factory)
	if code != 0 || !strings.Contains(stdout.String(), "没有声明") || !strings.Contains(stdout.String(), "/feedback") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunCapabilitiesCommandRejectsUnsupportedFormat(t *testing.T) {
	factory := func(string) *http.Client {
		return &http.Client{Transport: capabilityRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{"schema":"cc-connect-next.agent-capabilities/v1","version":"v-test","project":"demo","read_only":true,"security_note":"safe","configuration":{"version":"v-test","capabilities":[],"options":[]},"tools":[],"commands":[],"skills":[],"runtime":[]}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		})}
	}
	var stdout, stderr bytes.Buffer
	if code := runCapabilitiesCommand([]string{"--format", "yaml"}, &stdout, &stderr, factory); code != 2 || !strings.Contains(stderr.String(), "unsupported format") {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
}

func TestUnifiedManifestUsesFullConfigurationIntentExpansion(t *testing.T) {
	engine := core.NewEngine("demo", &capabilityManifestTestAgent{}, nil, "", core.LangChinese)
	engine.SetConfigCatalog(config.CapabilityCatalog("v-test"))
	manifest := engine.QueryAgentCapabilityManifest("", "消息忙的时候直接追加给当前回答", false)
	for _, option := range manifest.Configuration.Options {
		if option.Path == "queue.busy_message_mode" {
			return
		}
	}
	t.Fatalf("unified Manifest did not reuse the configuration intent search; first options=%v", manifest.Configuration.Options)
}

func TestUnifiedManifestAllIncludesEveryCompiledAdapterContract(t *testing.T) {
	catalog := config.CapabilityCatalog("v-test")
	engine := core.NewEngine("demo", &capabilityManifestTestAgent{}, nil, "", core.LangEnglish)
	engine.SetConfigCatalog(catalog)
	manifest := engine.QueryAgentCapabilityManifest("", "", true)
	if len(manifest.CompiledAgents) != len(catalog.Agents) || len(manifest.CompiledPlatforms) != len(catalog.Platforms) {
		t.Fatalf("compiled inventory = %d/%d, want %d/%d", len(manifest.CompiledAgents), len(manifest.CompiledPlatforms), len(catalog.Agents), len(catalog.Platforms))
	}
	if len(manifest.Configuration.Options) != len(catalog.Options) || len(manifest.Configuration.Options) < 500 {
		t.Fatalf("all-adapter configuration options = %d, catalog = %d", len(manifest.Configuration.Options), len(catalog.Options))
	}
	foundInactivePlatform := false
	for _, adapter := range manifest.Runtime {
		if adapter.Kind == "platform" && adapter.State == core.CapabilityUnavailable && len(adapter.Capabilities) == 1 && adapter.Capabilities[0].ID == "activation" {
			foundInactivePlatform = true
			break
		}
	}
	if !foundInactivePlatform {
		t.Fatal("all-adapter Manifest has no explicit compiled-but-inactive Platform entry")
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) > 4<<20 {
		t.Fatalf("full Manifest is unexpectedly large: %d bytes", len(encoded))
	}
}

func TestCapabilitiesCLIToLocalRuntimeAPIEndToEnd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix socket API")
	}
	dataDir, err := os.MkdirTemp("/tmp", "ccm-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dataDir) })
	api, err := core.NewAPIServer(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	engine := core.NewEngine("demo", &capabilityManifestTestAgent{}, nil, "", core.LangChinese)
	engine.SetConfigCatalog(config.CapabilityCatalog("v-test"))
	api.RegisterEngine("demo", engine)
	api.Start()
	t.Cleanup(api.Stop)
	if info, err := os.Stat(api.SocketPath()); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("capability API socket mode: info=%v err=%v", info, err)
	}

	var stdout, stderr bytes.Buffer
	code := runCapabilitiesCommand([]string{
		"--data-dir", dataDir, "--project", "demo", "--search", "消息忙的时候直接追加给当前回答", "--lang", "zh",
	}, &stdout, &stderr, newLocalAPIClient)
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	for _, want := range []string{"Agent 能力清单", "queue.busy_message_mode", "权限", "退化行为"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("E2E output missing %q:\n%s", want, stdout.String())
		}
	}
}
