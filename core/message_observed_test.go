package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

type observingTestPlatform struct {
	stubPlatformEngine
	observer func(HookEvent) error
}

func (p *observingTestPlatform) SetMessageObserver(observer func(HookEvent) error) {
	p.observer = observer
}

func TestSetHooks_ObservationOptInAndFailurePropagation(t *testing.T) {
	p := &observingTestPlatform{stubPlatformEngine: stubPlatformEngine{n: "observer"}}
	e := &Engine{platforms: []Platform{p}}
	e.SetHooks(NewHookManager("project", []HookConfig{{Event: "message.received", Type: "command", Command: "exit 0"}}, "sh", "-c", ""))
	if p.observer != nil {
		t.Fatal("unrelated hooks enabled observation")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var event HookEvent
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			t.Error(err)
		}
		if event.Project != "project" || event.Event != HookEventMessageObserved || event.Content != "" {
			t.Errorf("unexpected observer event: %#v", event)
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	e.SetHooks(NewHookManager("project", []HookConfig{{Event: "message.observed", Type: "http", URL: server.URL}}, "sh", "-c", ""))
	if p.observer == nil {
		t.Fatal("configured observation hook is not wired")
	}
	if err := p.observer(HookEvent{Event: HookEventMessageObserved}); err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("expected synchronous HTTP failure, got %v", err)
	}
	e.SetHooks(nil)
	if p.observer != nil {
		t.Fatal("removing hooks did not remove the observer")
	}
}

func TestEmitSync_WaitsEvenWhenHookIsConfiguredAsync(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-release
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	releaseHook := sync.OnceFunc(func() { close(release) })
	defer releaseHook()
	hm := NewHookManager("project", []HookConfig{{Event: "message.observed", Type: "http", URL: server.URL, Async: boolPtr(true)}}, "sh", "-c", "")
	done := make(chan error, 1)
	go func() { done <- hm.EmitSync(HookEvent{Event: HookEventMessageObserved}) }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("hook did not start")
	}
	select {
	case err := <-done:
		t.Fatalf("synchronous observer acknowledged before persistence finished: %v", err)
	default:
	}
	releaseHook()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("hook did not finish after persistence completed")
	}
}

func TestEmitSync_CommandMetadataAndFailures(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix shell command")
	}
	output := filepath.Join(t.TempDir(), "route.json")
	hm := NewHookManager("project", []HookConfig{
		{Event: "message.observed", Type: "command", Command: "exit 7"},
		{Event: "*", Type: "command", Command: `printf '%s' "$CC_HOOK_EXTRA" > ` + output},
	}, "sh", "-c", "")
	metadata := map[string]any{"message_id": "om_source", "chat_id": "oc_source", "mentioned_user_ids": []string{"ou_target"}}
	if err := hm.EmitSync(HookEvent{Event: HookEventMessageObserved, Extra: metadata}); err == nil {
		t.Fatal("command failure was acknowledged")
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("remaining matching hook was skipped after failure: %v", err)
	}
	var got, want any
	encoded, _ := json.Marshal(metadata)
	if json.Unmarshal(data, &got) != nil || json.Unmarshal(encoded, &want) != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("command did not receive exact metadata: %q", data)
	}
}

func TestEmitSync_NilAndUnmatchedHooksAreNoOps(t *testing.T) {
	var hm *HookManager
	if err := hm.EmitSync(HookEvent{Event: HookEventMessageObserved}); err != nil {
		t.Fatal(err)
	}
	hm = NewHookManager("project", []HookConfig{{Event: "message.received", Type: "http", URL: "http://127.0.0.1:1"}}, "sh", "-c", "")
	if err := hm.EmitSync(HookEvent{Event: HookEventMessageObserved}); err != nil {
		t.Fatal(err)
	}
}

func TestMessageObservationManifest_RequiresConfiguredHookAndKeepsBodiesPrivate(t *testing.T) {
	p := &observingTestPlatform{stubPlatformEngine: stubPlatformEngine{n: "observer"}}
	e := NewEngine("project", &stubAgent{}, []Platform{p}, "", LangEnglish)
	e.platformLifecycleMu.Lock()
	e.platformReady[p] = true
	e.platformLifecycleMu.Unlock()
	feature := func() RuntimeFeatureCapability {
		manifest := e.QueryAgentCapabilityManifest("", "", false)
		adapter := findRuntimeAdapter(t, manifest.Runtime, "platform", "observer")
		return findRuntimeFeature(t, adapter.Capabilities, "message_observation")
	}
	if got := feature(); got.Availability.State != CapabilityUnavailable {
		t.Fatalf("unconfigured observer advertised as enabled: %#v", got)
	}
	e.SetHooks(NewHookManager("project", []HookConfig{{Event: "message.observed", Type: "command", Command: "DO_NOT_LEAK_HOOK_COMMAND"}}, "sh", "-c", ""))
	got := feature()
	if got.Availability.State != CapabilityConditional || got.Permission != CapabilityPermissionConditional ||
		got.ReadOnly == nil || *got.ReadOnly || !hasManifestEffect(got.SideEffects, "shell_execution") {
		t.Fatalf("observer capability omitted its access or side effect boundary: %#v", got)
	}
	encoded, err := json.Marshal(e.QueryAgentCapabilityManifest("", "", false))
	if err != nil || strings.Contains(string(encoded), "DO_NOT_LEAK_HOOK_COMMAND") {
		t.Fatal("capability manifest exposed configured hook contents")
	}
}
