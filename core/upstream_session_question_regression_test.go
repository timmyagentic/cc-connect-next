package core

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// failOnceCodexLikeSession models a backend that creates a resumable thread,
// then fails its first turn without including the thread ID on EventError.
type failOnceCodexLikeSession struct {
	threadID string
	events   chan Event
	alive    atomic.Bool
	sends    atomic.Int32
}

func (s *failOnceCodexLikeSession) Send(_ string, _ []ImageAttachment, _ []FileAttachment) error {
	if s.sends.Add(1) == 1 {
		s.events <- Event{Type: EventError, Error: errors.New("Selected model is at capacity")}
	} else {
		s.events <- Event{Type: EventResult, SessionID: s.threadID, Content: "recovered", Done: true}
	}
	return nil
}
func (s *failOnceCodexLikeSession) RespondPermission(_ string, _ PermissionResult) error {
	return nil
}
func (s *failOnceCodexLikeSession) Events() <-chan Event { return s.events }
func (s *failOnceCodexLikeSession) CurrentSessionID() string {
	if s.sends.Load() > 0 {
		return s.threadID
	}
	return ""
}
func (s *failOnceCodexLikeSession) Alive() bool  { return s.alive.Load() }
func (s *failOnceCodexLikeSession) Close() error { s.alive.Store(false); close(s.events); return nil }

func TestEventErrorPersistsLateSessionIDForRetry(t *testing.T) {
	sess := &failOnceCodexLikeSession{
		threadID: "codex-thread-capacity",
		events:   make(chan Event, 4),
	}
	sess.alive.Store(true)
	starts := atomic.Int32{}
	agent := &controllableAgent{startSessionFn: func(_ context.Context, _ string) (AgentSession, error) {
		starts.Add(1)
		return sess, nil
	}}
	p := &stubPlatformEngine{n: "plain"}
	e := NewEngine("test", agent, []Platform{p}, "", LangEnglish)
	key := "test:capacity-retry"
	defer e.cleanupInteractiveState(key)

	e.ReceiveMessage(p, &Message{SessionKey: key, Content: "full alert context", ReplyCtx: "ctx1"})
	waitForPersistedLateSessionID(t, e.sessions.GetOrCreateActive(key), "codex-thread-capacity")
	e.ReceiveMessage(p, &Message{SessionKey: key, Content: "retry", ReplyCtx: "ctx2"})

	deadline := time.Now().Add(time.Second)
	for (sess.sends.Load() < 2 || e.sessions.GetOrCreateActive(key).Busy() || !strings.Contains(strings.Join(p.getSent(), "\n"), "recovered")) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := starts.Load(); got != 1 {
		t.Fatalf("StartSession calls = %d, want 1; retry recycled the live context", got)
	}
	if got := sess.sends.Load(); got != 2 {
		t.Fatalf("Send calls = %d, want 2", got)
	}
}

func TestGetOrCreateWorkspaceAgent_InvalidatesStaleSessionIDs(t *testing.T) {
	baseDir := t.TempDir()
	dataDir := t.TempDir()
	sessionPath := filepath.Join(dataDir, "sessions.json")
	bindingPath := filepath.Join(dataDir, "bindings.json")
	agentName := "test-multiworkspace-invalidate-agent"

	RegisterAgent(agentName, func(opts map[string]any) (Agent, error) {
		return &namedTestAgent{name: agentName}, nil
	})

	e := NewEngine("test", &namedTestAgent{name: agentName}, nil, sessionPath, LangEnglish)
	e.SetMultiWorkspace(baseDir, bindingPath)

	workspaceDir := normalizeWorkspacePath(filepath.Join(baseDir, "workspace"))
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	h := sha256.Sum256([]byte(workspaceDir))
	workspaceSessionPath := filepath.Join(dataDir, fmt.Sprintf("%s_ws_%x.json", e.name, h[:4]))
	t.Cleanup(func() {
		_ = os.Remove(workspaceSessionPath)
	})

	store := NewSessionManager(workspaceSessionPath)
	stale := store.GetOrCreateActive("feishu:channel1:user1")
	stale.SetAgentSessionID("old-claudecode-session", "claudecode")
	keep := store.NewSession("feishu:channel1:user1", "keep")
	keep.SetAgentSessionID("keep-opencode-session", agentName)
	store.Save()

	agent, sessions, err := e.getOrCreateWorkspaceAgent(workspaceDir)
	if err != nil {
		t.Fatalf("getOrCreateWorkspaceAgent: %v", err)
	}
	if agent == nil || sessions == nil {
		t.Fatalf("expected workspace agent and session manager, got agent=%v sessions=%v", agent, sessions)
	}

	loadedStale := sessions.FindByID(stale.ID)
	if loadedStale == nil {
		t.Fatalf("stale session %q not loaded", stale.ID)
	}
	if got := loadedStale.AgentSessionID; got != "" {
		t.Fatalf("stale session id = %q, want cleared", got)
	}
	if got := loadedStale.AgentType; got != agentName {
		t.Fatalf("stale session agent type = %q, want %q", got, agentName)
	}

	loadedKeep := sessions.FindByID(keep.ID)
	if loadedKeep == nil {
		t.Fatalf("kept session %q not loaded", keep.ID)
	}
	if got := loadedKeep.AgentSessionID; got != "keep-opencode-session" {
		t.Fatalf("kept session id = %q, want preserved", got)
	}
	if got := loadedKeep.AgentType; got != agentName {
		t.Fatalf("kept session agent type = %q, want %q", got, agentName)
	}

	reloaded := NewSessionManager(workspaceSessionPath)
	reloadedStale := reloaded.FindByID(stale.ID)
	if reloadedStale == nil {
		t.Fatalf("stale session %q not persisted", stale.ID)
	}
	if got := reloadedStale.AgentSessionID; got != "" {
		t.Fatalf("persisted stale session id = %q, want cleared", got)
	}
	if got := reloadedStale.AgentType; got != agentName {
		t.Fatalf("persisted stale session agent type = %q, want %q", got, agentName)
	}
}

// TestSendAskQuestionPrompt_CardPlatform_SingleSelectUsesActionRows is a
// regression for issue #1658 (Feishu pi ask_user_question card: button clicks
// never dispatched on mobile/desktop). The old layout rendered each option
// as a `column_set` row with the button nested inside a column; Feishu mobile
// clients dropped those click events, and even on desktop the layout was
// reported as unreliable. The fix renders each option as a markdown line +
// a flat action row (tag:"action") whose single button carries the askq:
// value and the askq_label/askq_question extras — the same shape that
// permission cards use for their cmd: clicks, which the issue reporter
// confirmed dispatch reliably.
func TestSendAskQuestionPrompt_CardPlatform_SingleSelectUsesActionRows(t *testing.T) {
	e := newTestEngine()
	p := &stubCardPlatform{stubPlatformEngine: stubPlatformEngine{n: "feishu"}}
	e.sendAskQuestionPrompt(p, "ctx", testQuestions(), 0)

	if len(p.sentCards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(p.sentCards))
	}
	card := p.sentCards[0]

	// Count askq action rows. We expect exactly 3 options, each rendered as
	// its own CardActions (one button per row) — not as nested CardListItem.
	var actionRows int
	var askqButtons int
	var sawCardListItem bool
	for _, elem := range card.Elements {
		switch e := elem.(type) {
		case CardActions:
			for _, btn := range e.Buttons {
				if strings.HasPrefix(btn.Value, "askq:") {
					askqButtons++
					// Each button must carry the askq_label + askq_question
					// extras so the Feishu callback handler can render the
					// post-answer card without re-fetching the question.
					if btn.Extra["askq_label"] == "" {
						t.Errorf("askq button %q missing askq_label extra", btn.Text)
					}
					if btn.Extra["askq_question"] == "" {
						t.Errorf("askq button %q missing askq_question extra", btn.Text)
					}
				}
			}
			actionRows++
		case CardListItem:
			sawCardListItem = true
		}
	}
	if actionRows != 3 {
		t.Errorf("expected 3 CardActions rows (one per option), got %d", actionRows)
	}
	if askqButtons != 3 {
		t.Errorf("expected 3 askq buttons in action rows, got %d", askqButtons)
	}
	if sawCardListItem {
		t.Errorf("CardListItem must not be used for single-select AskUserQuestion on Feishu (issue #1658)")
	}
}

func waitForPersistedLateSessionID(t *testing.T, session *Session, want string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if session.GetAgentSessionID() == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("session ID = %q, want %q", session.GetAgentSessionID(), want)
}
