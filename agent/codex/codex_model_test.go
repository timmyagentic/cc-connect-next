package codex

import (
	"strings"
	"testing"

	"github.com/timmyagentic/cc-connect-next/core"
)

func TestConfiguredModels_BoundaryConditions(t *testing.T) {
	a := &Agent{}
	a.SetProviders([]core.ProviderConfig{
		{Name: "first", Models: []core.ModelOption{{Name: "first"}}},
		{Name: "second", Models: []core.ModelOption{{Name: "second"}}},
	})
	if got := a.configuredModels(); got != nil {
		t.Fatalf("configuredModels() without active provider = %v, want nil", got)
	}
	if !a.SetActiveProvider("second") {
		t.Fatal("SetActiveProvider(second) = false")
	}
	if got := a.configuredModels(); len(got) != 1 || got[0].Name != "second" {
		t.Fatalf("configuredModels() = %v, want second", got)
	}
}

func TestGetModel_PrefersActiveProviderModel(t *testing.T) {
	a := &Agent{model: "gpt-4.1-mini"}
	a.SetProviders([]core.ProviderConfig{{Name: "openai", Model: "gpt-5.4"}})
	a.SetActiveProvider("openai")

	if got := a.GetModel(); got != "gpt-5.4" {
		t.Fatalf("GetModel() = %q, want gpt-5.4", got)
	}
}

func TestNormalizeAppServerURL_StdIOIsExplicit(t *testing.T) {
	for _, raw := range []string{"stdio", " stdio "} {
		if got := normalizeAppServerURL(raw); got != "stdio://" {
			t.Fatalf("normalizeAppServerURL(%q) = %q, want stdio://", raw, got)
		}
	}
}

func TestNormalizeBackend_EmptyDefaultsToNativeAppServer(t *testing.T) {
	if got := normalizeBackend(""); got != "app_server" {
		t.Fatalf("normalizeBackend(empty) = %q, want app_server", got)
	}
}

func TestNormalizeBackend_ExplicitExecRemainsAvailable(t *testing.T) {
	for _, raw := range []string{"exec", " EXEC "} {
		if got := normalizeBackend(raw); got != "exec" {
			t.Fatalf("normalizeBackend(%q) = %q, want exec", raw, got)
		}
	}
}

func TestNormalizeAppServerURL_EmptyDefaultsToStdIO(t *testing.T) {
	if got := normalizeAppServerURL(""); got != "stdio://" {
		t.Fatalf("normalizeAppServerURL(empty) = %q, want stdio://", got)
	}
}

func TestNativeSteerStatus_ReflectsConfiguredBackend(t *testing.T) {
	var _ core.NativeSteerDoctorInfo = (*Agent)(nil)

	appServer := &Agent{backend: "app_server"}
	if available, detail := appServer.NativeSteerStatus(); !available || detail == "" {
		t.Fatalf("app-server NativeSteerStatus() = %v, %q; want available with detail", available, detail)
	}

	execBackend := &Agent{backend: "exec"}
	available, detail := execBackend.NativeSteerStatus()
	if available {
		t.Fatal("exec NativeSteerStatus() = available, want unavailable")
	}
	for _, want := range []string{"fall back to FIFO", `backend = "app_server"`, `busy_message_mode = "queue"`} {
		if !strings.Contains(detail, want) {
			t.Fatalf("exec NativeSteerStatus() detail = %q, want %q", detail, want)
		}
	}
}

func TestWorkspaceAgentOptions_PreservesStdIOAppServerURL(t *testing.T) {
	a := &Agent{
		backend:      "app_server",
		appServerURL: normalizeAppServerURL("stdio"),
	}

	opts := a.WorkspaceAgentOptions()
	if got := opts["app_server_url"]; got != "stdio://" {
		t.Fatalf("WorkspaceAgentOptions()[app_server_url] = %#v, want stdio://", got)
	}
}

func TestIsCodexChatModel(t *testing.T) {
	accept := []string{
		"gpt-5.3-codex", "gpt-5.4", "gpt-6", "gpt-4.1-mini", "gpt-4o",
		"chatgpt-4o-latest", "codex-mini-latest", "o1-mini", "o3-mini",
		"o4-mini", "o5-preview", "o1", "o3", "o4", "o5", "GPT-5.3-Codex",
	}
	for _, id := range accept {
		if !isCodexChatModel(id) {
			t.Errorf("isCodexChatModel(%q) = false, want true", id)
		}
	}
	reject := []string{
		"", "text-embedding-3-small", "whisper-1", "tts-1-hd", "dall-e-3",
		"gpt-image-1", "gpt-4o-realtime-preview", "gpt-4o-transcribe",
		"gpt-4o-search-preview", "gpt-4o-audio-preview", "omni-moderation-latest",
		"claude-sonnet-4", "davinci-002", "babbage-002",
	}
	for _, id := range reject {
		if isCodexChatModel(id) {
			t.Errorf("isCodexChatModel(%q) = true, want false", id)
		}
	}
}
