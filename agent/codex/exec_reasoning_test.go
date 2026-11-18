package codex

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/timmyagentic/cc-connect-next/core"
)

func reasoningRollout(t *testing.T, home, id string) string {
	t.Helper()
	dir := filepath.Join(home, "sessions", time.Now().Format("2006/01/02"))
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "rollout-fixture-"+id+".jsonl")
	appendRollout(t, path, fmt.Sprintf(`{"type":"session_meta","payload":{"id":%q}}`+"\n", id))
	return path
}

func appendRollout(t *testing.T, path, text string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if _, err := io.WriteString(f, text); err != nil {
		t.Fatal(err)
	}
}

func completedReasoning(thread, turn, id string) string {
	return fmt.Sprintf(`{"type":"event_msg","payload":{"type":"item_completed","thread_id":%q,"turn_id":%q,"item":{"type":"Reasoning","id":%q,"summary_text":[""],"raw_content":[]}}}`+"\n", thread, turn, id)
}

func TestExecCountsOmittedReasoningWhileStdoutIsSilent(t *testing.T) {
	home := t.TempDir()
	path := reasoningRollout(t, home, "thread")
	appendRollout(t, path, `{"type":"event_msg","payload":{"type":"task_started","turn_id":"turn"}}`+"\n"+
		completedReasoning("thread", "turn", "empty-1")+completedReasoning("thread", "turn", "empty-2"))

	cs := &codexSession{ctx: context.Background(), events: make(chan core.Event, 16)}
	reader, writer := io.Pipe()
	done := make(chan error, 1)
	go func() { done <- cs.readExecEvents(reader, newExecReasoning(home, "")) }()
	t.Cleanup(func() { _ = writer.Close() })
	if _, err := fmt.Fprintln(writer, `{"type":"thread.started","thread_id":"thread"}`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		select {
		case e := <-cs.events:
			if e.Type != core.EventThinking || e.ThinkingKind != core.ThinkingItem || e.Content != "" {
				t.Fatalf("unexpected event %#v", e)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("hidden reasoning never reached the running card")
		}
	}

	// The CLI renumbers the visible item. Its mirror, a replayed completion
	// and commentary preceding a tool must not add extra reasoning events.
	appendRollout(t, path, completedReasoning("thread", "turn", "visible")+
		completedReasoning("thread", "turn", "visible")+
		`{"type":"response_item","payload":{"type":"reasoning","id":"visible","summary":[{"text":"summary"}]}}`+"\n")
	if _, err := io.WriteString(writer, strings.Join([]string{
		`{"type":"item.completed","item":{"type":"reasoning","id":"item_0","text":"summary"}}`,
		`{"type":"item.completed","item":{"type":"agent_message","id":"item_1","text":"commentary"}}`,
		`{"type":"item.started","item":{"type":"command_execution","id":"item_2","command":"echo test"}}`,
		`{"type":"turn.completed"}`,
	}, "\n")+"\n"); err != nil {
		t.Fatal(err)
	}
	_ = writer.Close()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	var counted, tools int
	var texts []string
	var last core.EventType
	for len(cs.events) > 0 {
		e := <-cs.events
		last = e.Type
		if e.Type == core.EventToolUse {
			tools++
		}
		if e.Type == core.EventThinking {
			switch e.ThinkingKind {
			case core.ThinkingItem:
				counted++
			case core.ThinkingText:
				texts = append(texts, e.Content)
			}
		}
	}
	if counted != 1 || tools != 1 || strings.Join(texts, ",") != "summary,commentary" || last != core.EventResult {
		t.Fatalf("later events: counted %d, tools %d, text %q, last %s", counted, tools, texts, last)
	}
}

func TestExecReasoningScopesResumesAndPartialRecords(t *testing.T) {
	home := t.TempDir()
	path := reasoningRollout(t, home, "thread")
	appendRollout(t, path, `{"type":"turn_context","payload":{"turn_id":"old"}}`+"\n"+
		completedReasoning("thread", "old", "old-id"))
	r := newExecReasoning(home, "thread")
	defer r.close()
	r.observe(map[string]any{"type": "thread.started", "thread_id": "thread"})
	appendRollout(t, path, `{"type":"event_msg","payload":{"type":"task_started","turn_id":"new"}}`+"\n"+
		completedReasoning("thread", "old", "foreign-turn")+completedReasoning("other", "new", "foreign-thread")+
		`{"type":"response_item","payload":{"type":"reasoning","id":"unfinished","status":"in_progress"}}`+"\n"+
		`{"type":"response_item","payload":{"type":"reasoning","id":"interrupted","status":"incomplete"}}`+"\n"+
		strings.TrimSuffix(completedReasoning("thread", "new", "current"), "\n"))
	r.poll(false)
	if len(r.rolloutIDs) != 0 {
		t.Fatalf("counted a partial or foreign record: %v", r.rolloutIDs)
	}
	appendRollout(t, path, "\n"+`{"type":"turn_context","payload":{"turn_id":"later"}}`+"\n"+
		`{"type":"response_item","payload":{"type":"reasoning","id":"later-id"}}`+"\n")
	r.poll(true)
	if _, ok := r.rolloutIDs["current"]; !ok || len(r.rolloutIDs) != 1 {
		t.Fatalf("resumed count: %v", r.rolloutIDs)
	}
}

func TestExecReasoningWithoutRolloutCountsVisibleItemIDsOnce(t *testing.T) {
	cs := &codexSession{ctx: context.Background(), events: make(chan core.Event, 8)}
	raw := strings.Join([]string{
		`{"type":"thread.started","thread_id":"missing"}`,
		`{"type":"item.completed","item":{"type":"reasoning","id":"item_0","text":"visible"}}`,
		`{"type":"item.completed","item":{"type":"reasoning","id":"item_0","text":"visible"}}`,
		`{"type":"turn.completed"}`,
	}, "\n")
	if err := cs.readExecEvents(strings.NewReader(raw), newExecReasoning(t.TempDir(), "")); err != nil {
		t.Fatal(err)
	}
	var count int
	for len(cs.events) > 0 {
		if e := <-cs.events; e.ThinkingKind == core.ThinkingItem {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("got %d reasoning items, want 1", count)
	}
}

func TestExecReasoningRejectsUnverifiableResume(t *testing.T) {
	for _, kind := range []string{"missing at launch", "removed", "replaced", "wrong metadata", "backup suffix"} {
		t.Run(kind, func(t *testing.T) {
			home := t.TempDir()
			path := reasoningRollout(t, home, "thread")
			if kind == "missing at launch" {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			} else if kind == "wrong metadata" {
				if err := os.WriteFile(path, []byte(`{"type":"session_meta","payload":{"id":"other"}}`+"\n"), 0600); err != nil {
					t.Fatal(err)
				}
			} else if kind == "backup suffix" {
				if err := os.Rename(path, path+".bak"); err != nil {
					t.Fatal(err)
				}
				path += ".bak"
			}
			r := newExecReasoning(home, "thread")
			defer r.close()
			switch kind {
			case "removed", "replaced":
				if err := os.Rename(path, path+".old"); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "missing at launch" || kind == "replaced" {
				path = reasoningRollout(t, home, "thread")
			}
			if kind != "removed" {
				appendRollout(t, path, `{"type":"turn_context","payload":{"turn_id":"foreign"}}`+"\n"+
					completedReasoning("thread", "foreign", "foreign-id"))
			}
			r.observe(map[string]any{"type": "thread.started", "thread_id": "thread"})
			r.poll(true)
			if len(r.rolloutIDs) != 0 {
				t.Fatalf("counted unbound items %v", r.rolloutIDs)
			}
		})
	}
}

func TestExecReasoningLiveLookupAcrossMidnightRequiresUniqueRollout(t *testing.T) {
	home := t.TempDir()
	path := reasoningRollout(t, home, "thread")
	appendRollout(t, path, `{"type":"turn_context","payload":{"turn_id":"turn"}}`+"\n"+
		completedReasoning("thread", "turn", "one"))
	observe := func() int {
		r := newExecReasoning(home, "")
		defer r.close()
		r.started = r.started.AddDate(0, 0, -1)
		r.observe(map[string]any{"type": "thread.started", "thread_id": "thread"})
		r.poll(false)
		return len(r.rolloutIDs)
	}
	if got := observe(); got != 1 {
		t.Fatalf("live adjacent-day count = %d", got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(path), "rollout-copy-thread.jsonl"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if got := observe(); got != 0 {
		t.Fatalf("counted %d items from ambiguous rollouts", got)
	}
}
