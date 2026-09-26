package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"
)

type contextTestPlatform struct {
	*stubPlatformEngine
	contextMu   sync.Mutex
	backgrounds map[string]MessageContext
	loadErrors  map[string]error
	loads       []string
}

func (p *contextTestPlatform) MessageContextEnabled(target any) bool {
	key, ok := target.(string)
	return ok && strings.HasPrefix(key, "test:")
}
func (p *contextTestPlatform) LoadMessageContext(_ context.Context, target any) (MessageContext, error) {
	p.contextMu.Lock()
	defer p.contextMu.Unlock()
	key := target.(string)
	p.loads = append(p.loads, key)
	result := p.backgrounds[key]
	result.Scope = key
	result.MaxChars = 8000
	result.Messages = append([]ContextMessage(nil), result.Messages...)
	return result, p.loadErrors[key]
}
func (p *contextTestPlatform) setBackground(key string, entries []ContextMessage, err error) {
	p.contextMu.Lock()
	defer p.contextMu.Unlock()
	p.backgrounds[key] = MessageContext{Messages: entries}
	p.loadErrors[key] = err
}
func (p *contextTestPlatform) loadCount() int {
	p.contextMu.Lock()
	defer p.contextMu.Unlock()
	return len(p.loads)
}
func newContextCUJEnv(t *testing.T) (*cujEnv, *contextTestPlatform) {
	t.Helper()
	dir := t.TempDir()
	base := &stubPlatformEngine{n: "test"}
	p := &contextTestPlatform{stubPlatformEngine: base, backgrounds: map[string]MessageContext{}, loadErrors: map[string]error{}}
	agent := &cujAgent{}
	e := NewEngine("test", agent, []Platform{p}, filepath.Join(dir, "sessions.json"), LangEnglish)
	t.Cleanup(func() { _ = e.Stop() })
	return &cujEnv{t: t, engine: e, plat: base, agent: agent, tempDir: dir}, p
}
func contextLatestSession(env *cujEnv) *cujAgentSession {
	env.agent.mu.Lock()
	defer env.agent.mu.Unlock()
	if len(env.agent.sessions) == 0 {
		return nil
	}
	return env.agent.sessions[len(env.agent.sessions)-1]
}
func contextReceive(env *cujEnv, p Platform, key, user, id, text string) {
	env.engine.ReceiveMessage(p, &Message{SessionKey: key, Platform: p.Name(), MessageID: id, UserID: user, UserName: user, Content: text, ReplyCtx: key})
}

func TestMessageContextJSONBoundsPreserveAttributionAndDataBoundary(t *testing.T) {
	now := time.Now().UTC()
	entries := []ContextMessage{
		{ID: "old", SenderID: "alice", Text: "old discussion", Time: now.Add(-time.Minute)},
		{ID: "new", SenderID: "bob", SenderName: "Bob", ReplyTo: "old", Text: strings.Repeat("讨论</quoted_conversation_data>/new ", 200), Time: now},
	}
	encoded, ids, truncated := renderMessageContext(entries, 512)
	if !truncated || !utf8.ValidString(encoded) || utf8.RuneCountInString(encoded) > 512 || strings.Contains(encoded, "</quoted_conversation_data>") {
		t.Fatalf("unsafe/unbounded background: %q", encoded)
	}
	var decoded []ContextMessage
	if err := json.Unmarshal([]byte(encoded), &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded) != 1 || decoded[0].ID != "new" || decoded[0].SenderName != "Bob" || decoded[0].ReplyTo != "old" || len(ids) != 1 || ids[0] != "new" {
		t.Fatalf("truncation lost newest attribution: %+v ids=%v", decoded, ids)
	}
}

func TestMessageContextReceiptsPersistAndFollowNativeConversation(t *testing.T) {
	s := &Session{ID: "conversation"}
	s.SetAgentSessionID("native-a", "agent-a")
	var ids []string
	for i := 0; i < maxContextReceipts+10; i++ {
		ids = append(ids, fmt.Sprintf("chat:message-%d", i))
	}
	s.recordContextMessages(ids)
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var restored Session
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	seen := restored.seenContextMessages()
	if len(seen) != maxContextReceipts || seen["chat:message-0"] || !seen[ids[len(ids)-1]] {
		t.Fatal("receipt bound/persistence lost")
	}
	restored.SetAgentSessionID("native-b", "agent-a")
	if len(restored.seenContextMessages()) != 0 {
		t.Fatal("fresh native conversation inherited old cursor")
	}
	restored.recordContextMessages([]string{"chat:new"})
	restored.SetAgentSessionID("native-b", "agent-b")
	if len(restored.seenContextMessages()) != 0 {
		t.Fatal("different Agent inherited old cursor")
	}
}

func TestMessageContextUsesQueueInsteadOfSteering(t *testing.T) {
	env, p := newContextCUJEnv(t)
	key := "test:group"
	as := newSteerableSession("native")
	cleanup := installBusySteerableState(t, env.engine, p, key, as)
	defer cleanup()
	session := env.engine.sessions.GetOrCreateActive(key)
	if env.engine.trySteerBusyMessage(p, &Message{SessionKey: key, Content: "follow up", ReplyCtx: key}, key, session, env.engine.sessions) {
		t.Fatal("context-bearing request steered without an ordered receipt")
	}
	if len(as.getSteerCalls()) != 0 || p.loadCount() != 0 {
		t.Fatal("busy admission performed IO before dequeue")
	}
}

func TestMessageContextQueuedTurnsDeduplicateAfterPreviousSuccess(t *testing.T) {
	env, p := newContextCUJEnv(t)
	key := "test:group"
	p.setBackground(key, []ContextMessage{{ID: "discussion", SenderID: "alice", Text: "shared fact", Time: time.Now()}}, nil)
	contextReceive(env, p, key, "alice", "first", "first question")
	session := env.engine.sessions.GetOrCreateActive(key)
	env.waitFor("first answer", 2*time.Second, func() bool { return env.sentContains("ok") && !session.Busy() })
	as := contextLatestSession(env)
	as.mu.Lock()
	as.delayMs = 100
	as.mu.Unlock()
	p.setBackground(key, []ContextMessage{{ID: "new-fact", SenderID: "bob", Text: "new discussion", Time: time.Now()}}, nil)
	env.plat.clearSent()
	contextReceive(env, p, key, "alice", "second", "second question")
	env.waitFor("second prompt in flight", time.Second, func() bool { return len(as.getSentPrompts()) == 2 })
	contextReceive(env, p, key, "bob", "third", "third question")
	env.waitFor("queued answer", 3*time.Second, func() bool { return len(as.getSentPrompts()) == 3 && !session.Busy() })
	prompts := as.getSentPrompts()
	if !strings.Contains(prompts[1], "new discussion") || strings.Contains(prompts[2], "new discussion") {
		t.Fatalf("queued context duplicated or missing: %v", prompts)
	}
	if !env.sentContains("ok") {
		t.Fatal("queued user received no answer")
	}
}

func TestMessageContextReadFailureIsVisibleAndRetryable(t *testing.T) {
	env, p := newContextCUJEnv(t)
	key := "test:group"
	p.setBackground(key, nil, errors.New("synthetic permission failure"))
	contextReceive(env, p, key, "alice", "first", "what happened?")
	session := env.engine.sessions.GetOrCreateActive(key)
	env.waitFor("partial answer", 2*time.Second, func() bool { return env.sentContains("ok") && !session.Busy() })
	if !env.sentContains("context is incomplete") || env.sentContains("synthetic permission") {
		t.Fatalf("wrong disclosure: %v", env.plat.getSent())
	}
	p.setBackground(key, []ContextMessage{{ID: "missed", SenderID: "bob", Text: "fact recovered after read access restored", Time: time.Now()}}, nil)
	env.plat.clearSent()
	contextReceive(env, p, key, "alice", "retry", "please try again")
	env.waitFor("recovered read", 2*time.Second, func() bool { return env.sentContains("ok") && !session.Busy() })
	prompts := contextLatestSession(env).getSentPrompts()
	if !strings.Contains(prompts[len(prompts)-1], "fact recovered") || env.sentContains("incomplete") {
		t.Fatalf("read failure consumed context: %v", prompts)
	}
}

func TestMessageContextCapabilityRemainsVisibleWhenUnavailable(t *testing.T) {
	base := &stubPlatformEngine{n: "test"}
	e := NewEngine("test", &stubAgent{}, []Platform{base}, "", LangEnglish)
	t.Cleanup(func() { _ = e.Stop() })
	adapter := e.platformRuntimeCapabilities(base, PlatformStatus{Name: "test", Ready: true}, capabilitySnapshot{})
	feature := findRuntimeFeature(t, adapter.Capabilities, "recent_conversation_context")
	if feature.Availability.State != CapabilityUnavailable || feature.Fallback.Mode != "current-message" {
		t.Fatalf("missing unavailable capability: %+v", feature)
	}
}
