package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func namedCardEvent(text string) HookEvent {
	return HookEvent{
		Event: HookEventCardAction, Project: "untrusted-project", Platform: "fixture", UserID: "operator",
		Extra: map[string]any{
			"hook": "confirm_reply", "action": "confirmed", "message_id": "message-1", "chat_id": "chat-1",
			"data": map[string]any{"request_id": "request-1"}, "form_values": map[string]any{"reply_text": text},
		},
	}
}

func TestNamedHook_SelectsProjectAndHandlerWithoutWildcardDisclosure(t *testing.T) {
	received := make(chan HookEvent, 8)
	paths := make(chan string, 8)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var event HookEvent
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			t.Error(err)
		}
		received <- event
		paths <- r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	hooks := []HookConfig{
		{Name: "confirm_reply", Project: "first", Event: "card.action", Type: "http", URL: srv.URL + "/first"},
		{Name: "confirm_reply", Project: "second", Event: "card.action", Type: "http", URL: srv.URL + "/second"},
		{Name: "archive", Project: "first", Event: "card.action", Type: "http", URL: srv.URL + "/archive"},
		{Event: "*", Type: "http", URL: srv.URL + "/wildcard", Async: boolPtr(false)},
	}
	hm := NewHookManager("first", hooks, "sh", "-c", "")
	event := namedCardEvent("edited message")
	hm.Emit(event)
	if err := hm.EmitSync(event); err != nil {
		t.Fatal(err)
	}
	if err := hm.DispatchNamed("missing", event); err == nil {
		t.Fatal("missing handler accepted")
	}
	if err := NewHookManager("third", hooks, "sh", "-c", "").DispatchNamed("confirm_reply", event); err == nil {
		t.Fatal("another project's handler accepted")
	}
	for _, name := range []string{"confirm_reply", "archive"} {
		if err := hm.DispatchNamed(name, event); err != nil {
			t.Fatal(err)
		}
	}
	gotPaths := make(map[string]bool)
	for range 2 {
		select {
		case got := <-received:
			if got.Project != "first" || got.Timestamp.IsZero() || got.Extra["form_values"].(map[string]any)["reply_text"] != "edited message" {
				t.Fatalf("unexpected event %#v", got)
			}
			gotPaths[<-paths] = true
		case <-time.After(time.Second):
			t.Fatal("named handler did not receive the event")
		}
	}
	if !gotPaths["/first"] || !gotPaths["/archive"] {
		t.Fatalf("handler paths = %v", gotPaths)
	}
	select {
	case got := <-paths:
		t.Fatalf("unexpected handler %q", got)
	case <-time.After(20 * time.Millisecond):
	}
}

func TestNamedHook_DoesNotWaitForHandlerAndCapturesCallbackData(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	received := make(chan HookEvent, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		var event HookEvent
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			t.Error(err)
		}
		received <- event
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	defer close(release)
	hm := NewHookManager("project", []HookConfig{{
		Name: "confirm_reply", Project: "project", Event: "card.action", Type: "http", URL: srv.URL, Async: boolPtr(false),
	}}, "sh", "-c", "")
	event := namedCardEvent("original")
	returned := make(chan error, 1)
	go func() { returned <- hm.DispatchNamed("confirm_reply", event) }()
	select {
	case err := <-returned:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("callback waited for the handler")
	}
	event.Extra["form_values"].(map[string]any)["reply_text"] = "changed after return"
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("handler was not started")
	}
	// Allow the handler to read after the caller has reused its callback data.
	release <- struct{}{}
	select {
	case got := <-received:
		if got.Extra["form_values"].(map[string]any)["reply_text"] != "original" {
			t.Fatalf("callback data was not captured: %#v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("handler did not finish")
	}
}

func TestNamedCommandHook_JSONStdinPreservesLiteralUserInput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX command fixture")
	}
	output := filepath.Join(t.TempDir(), "event.json")
	t.Setenv("HOOK_OUTPUT", output)
	text := "edited\n`touch injected` $(touch injected) ' \" " + strings.Repeat("x", 180000)
	hm := NewHookManager("project", []HookConfig{{
		Name: "confirm_reply", Project: "project", Event: "card.action", Type: "command", Input: "json",
		Command: `cat > "$HOOK_OUTPUT.tmp" && mv "$HOOK_OUTPUT.tmp" "$HOOK_OUTPUT"`,
	}}, "sh", "-c", "")
	if err := hm.DispatchNamed("confirm_reply", namedCardEvent(text)); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		body, err := os.ReadFile(output)
		if err == nil {
			var got HookEvent
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatal(err)
			}
			if got.Extra["form_values"].(map[string]any)["reply_text"] != text || got.Project != "project" {
				t.Fatal("stdin changed the submitted value or project")
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("command output missing: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestLifecycleHook_ProjectFilter(t *testing.T) {
	called := make(chan struct{}, 2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called <- struct{}{}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	hm := NewHookManager("first", []HookConfig{
		{Project: "second", Event: "message.sent", Type: "http", URL: srv.URL, Async: boolPtr(false)},
		{Project: "first", Event: "message.sent", Type: "http", URL: srv.URL, Async: boolPtr(false)},
	}, "sh", "-c", "")
	hm.Emit(HookEvent{Event: HookEventMessageSent})
	if len(called) != 1 {
		t.Fatalf("got %d deliveries, want one", len(called))
	}
}

func TestNamedHook_RejectsDuplicateBindingsAndNonCardEvents(t *testing.T) {
	hook := HookConfig{Name: "confirm_reply", Project: "project", Event: "card.action", Type: "command", Input: "json", Command: "exit 0"}
	hm := NewHookManager("project", []HookConfig{hook, hook}, "sh", "-c", "")
	if err := hm.DispatchNamed("confirm_reply", namedCardEvent("value")); err == nil {
		t.Fatal("duplicate named binding accepted")
	}
	hm = NewHookManager("project", []HookConfig{hook}, "sh", "-c", "")
	if err := hm.DispatchNamed("confirm_reply", HookEvent{Event: HookEventMessageSent}); err == nil {
		t.Fatal("lifecycle event used named card dispatch")
	}
	if err := hm.DispatchNamed("confirm_reply", HookEvent{Event: HookEventCardAction, Extra: map[string]any{"invalid": make(chan int)}}); err == nil {
		t.Fatal("non-JSON payload accepted")
	}
}

func TestNamedHook_CapabilityRequiresExactNamedSubscription(t *testing.T) {
	for _, hooks := range [][]HookConfig{
		{{Event: "*", Type: "command", Command: "exit 0"}},
		{{Name: "confirm_reply", Project: "other", Event: "card.action", Type: "command", Input: "json", Command: "exit 0"}},
	} {
		if NewHookManager("project", hooks, "sh", "-c", "").handlesEvent(HookEventCardAction) {
			t.Fatal("wildcard or another project's hook enabled card actions")
		}
	}
	hooks := []HookConfig{{Name: "confirm_reply", Project: "project", Event: "card.action", Type: "command", Input: "json", Command: "exit 0"}}
	if !NewHookManager("project", hooks, "sh", "-c", "").handlesEvent(HookEventCardAction) {
		t.Fatal("registered named card hook was unavailable")
	}
}
