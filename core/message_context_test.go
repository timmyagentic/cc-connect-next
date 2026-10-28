package core

import (
	"context"
	"encoding/json"
	"errors"
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
	positions   []time.Time
	onLoad      func(int)
}

func (p *contextTestPlatform) MessageContextEnabled(target any) bool {
	key, ok := target.(string)
	return ok && strings.HasPrefix(key, "test:")
}
func (p *contextTestPlatform) MessageContextScope(target any) string { return target.(string) }
func (p *contextTestPlatform) LoadMessageContext(_ context.Context, target any, after time.Time) (MessageContext, error) {
	p.contextMu.Lock()
	key := target.(string)
	p.loads = append(p.loads, key)
	p.positions = append(p.positions, after)
	loadCount, onLoad := len(p.loads), p.onLoad
	result := p.backgrounds[key]
	result.Scope, result.Through, result.MaxChars = key, time.Now().UTC(), 8000
	result.Messages = append([]ContextMessage(nil), result.Messages...)
	err := p.loadErrors[key]
	p.contextMu.Unlock()
	if onLoad != nil {
		onLoad(loadCount)
	}
	return result, err
}
func (p *contextTestPlatform) setBackground(key string, entries []ContextMessage, err error) {
	p.contextMu.Lock()
	defer p.contextMu.Unlock()
	for i := range entries {
		if entries[i].Time.IsZero() {
			entries[i].Time = time.Now().UTC()
		}
	}
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

func TestMessageContextPositionPersistsAndFollowsNativeConversation(t *testing.T) {
	s := &Session{}
	at := time.Now().UTC()
	s.advanceMessageContext("chat", at)
	s.SetAgentSessionID("native-a", "agent")
	if !s.messageContextPosition("chat").Equal(at) {
		t.Fatal("first native ID lost delivered input")
	}
	data, _ := json.Marshal(s)
	var restored Session
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	restored.advanceMessageContext("chat", at.Add(-time.Minute))
	if !restored.messageContextPosition("chat").Equal(at) || !restored.messageContextPosition("other-topic").IsZero() {
		t.Fatal("position regressed, leaked or failed to persist")
	}
	restored.SetAgentSessionID("native-b", "agent")
	if !restored.messageContextPosition("chat").IsZero() {
		t.Fatal("new conversation inherited old position")
	}
	restored.advanceMessageContext("chat", at)
	restored.SetAgentSessionID("native-b", "other-agent")
	if !restored.messageContextPosition("chat").IsZero() {
		t.Fatal("different agent inherited old position")
	}
}

func newContextSteerEnv(t *testing.T, key string) (*cujEnv, *contextTestPlatform, *steerableAgentSession, *Session) {
	t.Helper()
	env, p := newContextCUJEnv(t)
	as := newSteerableSession("native")
	t.Cleanup(installBusySteerableState(t, env.engine, p, key, as))
	session := env.engine.sessions.GetOrCreateActive(key)
	session.SetAgentSessionID("native", "agent")
	return env, p, as, session
}

func TestMessageContextConsecutiveSteersOnlyDeliverNewDiscussion(t *testing.T) {
	key := "test:group"
	env, p, as, session := newContextSteerEnv(t, key)
	p.setBackground(key, []ContextMessage{{ID: "old", Text: "initial discussion"}}, nil)
	for i, sender := range []string{"alice", "alice", "bob"} {
		if i == 2 {
			p.setBackground(key, []ContextMessage{{ID: "new", Text: "new shared decision"}}, nil)
		}
		if !env.engine.trySteerBusyMessage(p, &Message{SessionKey: key, UserID: sender, Content: "continue", ReplyCtx: key}, key, session, env.engine.sessions) {
			t.Fatal("follow-up was not steered")
		}
	}
	calls := as.getSteerCalls()
	if len(calls) != 3 || !strings.Contains(calls[0], "initial discussion") || strings.Contains(calls[1], "quoted_conversation_data") || !strings.Contains(calls[2], "new shared decision") {
		t.Fatalf("wrong incremental prompts: %v", calls)
	}
	if session.messageContextPosition(key).IsZero() || p.positions[1].IsZero() || !p.positions[2].After(p.positions[1]) {
		t.Fatal("steers did not advance position before turn completion")
	}
}

func TestMessageContextRejectedSteerDoesNotAdvancePosition(t *testing.T) {
	for _, err := range []error{ErrSteerTurnMismatch, ErrSteerOutcomeUnknown} {
		t.Run(err.Error(), func(t *testing.T) {
			key := "test:group"
			env, p, as, session := newContextSteerEnv(t, key)
			p.setBackground(key, []ContextMessage{{ID: "fact", Text: "new fact"}}, nil)
			as.steerErr = err
			env.engine.trySteerBusyMessage(p, &Message{SessionKey: key, UserID: "bob", Content: "continue", ReplyCtx: key}, key, session, env.engine.sessions)
			if !session.messageContextPosition(key).IsZero() {
				t.Fatal("unconfirmed delivery advanced position")
			}
		})
	}
}

func TestMessageContextQueuedTurnsOnlyDeliverNewDiscussion(t *testing.T) {
	env, p := newContextCUJEnv(t)
	key := "test:group"
	p.setBackground(key, []ContextMessage{{ID: "discussion", Text: "shared fact"}}, nil)
	contextReceive(env, p, key, "alice", "first", "first question")
	session := env.engine.sessions.GetOrCreateActive(key)
	env.waitFor("first answer", 2*time.Second, func() bool { return env.sentContains("ok") && !session.Busy() })
	as := contextLatestSession(env)
	as.mu.Lock()
	as.delayMs = 100
	as.mu.Unlock()
	p.setBackground(key, []ContextMessage{{ID: "new-fact", Text: "new discussion"}}, nil)
	contextReceive(env, p, key, "alice", "second", "second question")
	env.waitFor("second prompt in flight", time.Second, func() bool { return len(as.getSentPrompts()) == 2 })
	contextReceive(env, p, key, "bob", "third", "third question")
	env.waitFor("queued answer", 3*time.Second, func() bool { return len(as.getSentPrompts()) == 3 && !session.Busy() })
	prompts := as.getSentPrompts()
	if !strings.Contains(prompts[1], "new discussion") || strings.Contains(prompts[2], "new discussion") {
		t.Fatalf("queued context duplicated or missing: %v", prompts)
	}
}

func TestMessageContextFailedResultDoesNotReplayDeliveredBackground(t *testing.T) {
	env, p := newContextCUJEnv(t)
	key := "test:group"
	contextReceive(env, p, key, "alice", "warmup", "first question")
	session := env.engine.sessions.GetOrCreateActive(key)
	env.waitFor("warmup", 2*time.Second, func() bool { return env.sentContains("ok") && !session.Busy() })
	as := contextLatestSession(env)
	p.setBackground(key, []ContextMessage{{ID: "fact", Text: "delivered discussion"}}, nil)
	as.mu.Lock()
	as.nextEventOverride = &Event{Type: EventResult, Done: true, Content: "failed result", Error: errors.New("failed result")}
	as.mu.Unlock()
	env.plat.clearSent()
	contextReceive(env, p, key, "alice", "failed", "next question")
	env.waitFor("failed result", 2*time.Second, func() bool { return env.sentContains("failed result") && !session.Busy() })
	env.plat.clearSent()
	contextReceive(env, p, key, "alice", "retry", "please retry")
	env.waitFor("retry", 2*time.Second, func() bool { return env.sentContains("ok") && !session.Busy() })
	prompts := as.getSentPrompts()
	if len(prompts) != 3 || !strings.Contains(prompts[1], "delivered discussion") || strings.Contains(prompts[2], "delivered discussion") {
		t.Fatalf("delivered background replayed after failed work: %v", prompts)
	}
}

func TestMessageContextReadFailureDoesNotAdvancePosition(t *testing.T) {
	env, p := newContextCUJEnv(t)
	key := "test:group"
	p.setBackground(key, nil, errors.New("synthetic permission failure"))
	contextReceive(env, p, key, "alice", "first", "what happened?")
	session := env.engine.sessions.GetOrCreateActive(key)
	env.waitFor("partial answer", 2*time.Second, func() bool { return env.sentContains("ok") && !session.Busy() })
	if !env.sentContains("context is incomplete") || env.sentContains("synthetic permission") || !session.messageContextPosition(key).IsZero() {
		t.Fatalf("wrong disclosure or position: %v", env.plat.getSent())
	}
	p.setBackground(key, []ContextMessage{{ID: "missed", Text: "recovered fact"}}, nil)
	env.plat.clearSent()
	contextReceive(env, p, key, "alice", "retry", "please try again")
	env.waitFor("recovered read", 2*time.Second, func() bool { return env.sentContains("ok") && !session.Busy() })
	if !strings.Contains(contextLatestSession(env).getSentPrompts()[1], "recovered fact") {
		t.Fatal("failed history read consumed background")
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

func TestMessageContextDrainsEventsThatArriveDuringRead(t *testing.T) {
	for _, queued := range []bool{false, true} {
		name := "foreground"
		if queued {
			name = "queued"
		}
		t.Run(name, func(t *testing.T) {
			env, p := newContextCUJEnv(t)
			key := "test:group"
			contextReceive(env, p, key, "alice", "warmup", "first question")
			session := env.engine.sessions.GetOrCreateActive(key)
			env.waitFor("warmup", 2*time.Second, func() bool { return env.sentContains("ok") && !session.Busy() })
			as := contextLatestSession(env)
			as.mu.Lock()
			as.delayMs, as.reply = 100, "fresh answer"
			as.mu.Unlock()
			injectOnLoad := 2
			if queued {
				injectOnLoad = 3
			} else {
				state := env.engine.interactiveStates[key]
				state.mu.Lock()
				state.eventsNeedResync = true
				state.mu.Unlock()
			}
			p.contextMu.Lock()
			p.onLoad = func(count int) {
				if count == injectOnLoad {
					as.events <- Event{Type: EventError, Error: errors.New("stale previous turn event")}
				}
			}
			p.contextMu.Unlock()
			env.plat.clearSent()
			contextReceive(env, p, key, "alice", "next", "next question")
			if queued {
				env.waitFor("foreground send", time.Second, func() bool { return len(as.getSentPrompts()) == 2 })
				contextReceive(env, p, key, "bob", "queued", "queued question")
			}
			env.waitFor("new turns finished", 3*time.Second, func() bool { return len(as.getSentPrompts()) == injectOnLoad && !session.Busy() })
			if env.sentContains("stale previous turn event") {
				t.Fatalf("old event failed the new turn: %v", env.plat.getSent())
			}
		})
	}
}

type mediaContextPlatform struct {
	*contextTestPlatform
	hydrated []string
	fail     bool
}

func (p *mediaContextPlatform) HydrateMessageContext(_ context.Context, _ any, entries []ContextMessage) ([]ContextMessage, error) {
	p.contextMu.Lock()
	defer p.contextMu.Unlock()
	for i := range entries {
		if len(entries[i].Resources) == 0 {
			continue
		}
		p.hydrated = append(p.hydrated, entries[i].ID)
		if p.fail {
			entries[i].Incomplete = true
			entries[i].Text += " [image unavailable]"
			continue
		}
		entries[i].Images = []ImageAttachment{{MimeType: "image/png", Data: []byte("image bytes"), FileName: entries[i].ID + ".png"}}
		entries[i].Files = []FileAttachment{{MimeType: "text/plain", Data: []byte("file bytes"), FileName: "notes.txt", MessageID: entries[i].ID}}
	}
	return entries, nil
}

func TestMessageContextDeliveredMediaIsNotDownloadedAgain(t *testing.T) {
	env, base := newContextCUJEnv(t)
	p := &mediaContextPlatform{contextTestPlatform: base}
	key := "test:group"
	s, state := &Session{}, &interactiveState{}
	p.setBackground(key, []ContextMessage{{ID: "photo", Text: "shared image", Resources: []ContextResource{{Kind: "image", Key: "opaque"}}}}, nil)
	first := env.engine.prepareMessageContext(p, key, s, state, "look", "first")
	if len(first.Images) != 1 || len(first.Files) != 1 || !strings.Contains(first.Text, `"history_image_indexes":[1]`) || strings.Contains(first.Text, "opaque") {
		t.Fatalf("missing or unsafe media delivery: %+v", first)
	}
	acceptMessageContext(s, env.engine.sessions, first)
	second := env.engine.prepareMessageContext(p, key, s, state, "again", "second")
	if len(second.Images) != 0 || len(p.hydrated) != 1 || strings.Contains(second.Text, "shared image") {
		t.Fatalf("old media downloaded or injected again: %+v calls=%v", second, p.hydrated)
	}
}

type mediaContextAgent struct {
	stubAgent
	session *mediaContextAgentSession
}

func (a *mediaContextAgent) StartSession(context.Context, string) (AgentSession, error) {
	return a.session, nil
}

type mediaContextAgentSession struct{ *cujAgentSession }

func (s *mediaContextAgentSession) Send(prompt string, images []ImageAttachment, files []FileAttachment) error {
	var names []string
	for _, image := range images {
		names = append(names, image.FileName)
	}
	for _, file := range files {
		names = append(names, file.MessageID+"/"+file.FileName)
	}
	s.mu.Lock()
	s.reply = "attachments=" + strings.Join(names, ",")
	s.mu.Unlock()
	return s.cujAgentSession.Send(prompt, images, files)
}
