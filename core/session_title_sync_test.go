package core

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type titleSyncAgentSession struct {
	*controllableAgentSession
	err           error
	threadID      string
	title         string
	initialErr    error
	initialPrompt string
	initialCalls  int
}

func (s *titleSyncAgentSession) SetSessionTitle(threadID, title string) error {
	s.threadID = threadID
	s.title = title
	return s.err
}

func (s *titleSyncAgentSession) SetInitialSessionTitle(prompt string) error {
	s.initialPrompt = prompt
	s.initialCalls++
	return s.initialErr
}

type titleInitializingResultSession struct {
	AgentSession
	initialPrompt string
	initialCalls  int
}

type orderedTitleSession struct {
	*controllableAgentSession
	mu          sync.Mutex
	order       []string
	titlePrompt string
}

func (s *orderedTitleSession) record(step string) {
	s.mu.Lock()
	s.order = append(s.order, step)
	s.mu.Unlock()
}

func (s *orderedTitleSession) SetInitialSessionTitle(prompt string) error {
	s.mu.Lock()
	s.order = append(s.order, "title")
	s.titlePrompt = prompt
	s.mu.Unlock()
	return nil
}

func (s *orderedTitleSession) Send(string, []ImageAttachment, []FileAttachment) error {
	s.record("send")
	s.events <- Event{Type: EventResult, Content: "done", Done: true}
	return nil
}

func (s *orderedTitleSession) snapshot() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.order...)
}

func (s *orderedTitleSession) prompt() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.titlePrompt
}

type blockingTitleSession struct {
	*controllableAgentSession
	started chan struct{}
	release chan struct{}
}

func (s *blockingTitleSession) SetInitialSessionTitle(string) error {
	close(s.started)
	<-s.release
	return nil
}

func (s *titleInitializingResultSession) SetInitialSessionTitle(prompt string) error {
	s.initialPrompt = prompt
	s.initialCalls++
	return nil
}

func TestGetOrCreateInteractiveStateWith_TitlesFreshSessionAtCreation(t *testing.T) {
	for _, tt := range []struct {
		name    string
		initErr error
	}{
		{name: "success"},
		{name: "title failure is fail-open", initErr: errors.New("title unavailable")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			live := &titleSyncAgentSession{
				controllableAgentSession: newControllableSession("thread-fresh"),
				initialErr:               tt.initErr,
			}
			agent := &controllableAgent{nextSession: live}
			p := &stubPlatformEngine{n: "test"}
			e := NewEngine("test", agent, []Platform{p}, "", LangEnglish)
			managed := &Session{}

			state := e.getOrCreateInteractiveStateWith(
				"test:user", p, "ctx", managed, e.sessions, nil, "", "首个真实问题",
			)

			if state.agentSession != live {
				t.Fatal("fresh session was not returned after title initialization")
			}
			if live.initialCalls != 1 || live.initialPrompt != "首个真实问题" {
				t.Fatalf("initial title calls = %d, prompt = %q", live.initialCalls, live.initialPrompt)
			}
		})
	}
}

func TestProcessInteractiveMessage_CreatesThenTitlesBeforeFirstSend(t *testing.T) {
	live := &orderedTitleSession{controllableAgentSession: newControllableSession("thread-fresh")}
	agent := &controllableAgent{
		startSessionFn: func(context.Context, string) (AgentSession, error) {
			live.record("start")
			return live, nil
		},
	}
	p := &stubPlatformEngine{n: "test"}
	e := NewEngine("test", agent, []Platform{p}, "", LangEnglish)
	key := "test:user"
	managed := e.sessions.GetOrCreateActive(key)
	if !managed.TryLock() {
		t.Fatal("failed to lock managed session")
	}

	e.processInteractiveMessageWith(
		p,
		&Message{
			SessionKey:   key,
			Platform:     "test",
			MessageID:    "msg-1",
			Content:      "[Reply to Alice]: 不应进入标题\n首个真实问题",
			ExtraContent: "[Reply to Alice]: 不应进入标题",
			ReplyCtx:     "ctx",
		},
		managed,
		agent,
		e.sessions,
		key,
		"",
		key,
		"首个真实问题",
	)

	got := live.snapshot()
	want := []string{"start", "title", "send"}
	if len(got) != len(want) {
		t.Fatalf("lifecycle order = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("lifecycle order = %v, want %v", got, want)
		}
	}
	if got := live.prompt(); got != "首个真实问题" {
		t.Fatalf("title prompt = %q, want platform-neutral user content", got)
	}
}

func TestGetOrCreateInteractiveStateWith_TitleGenerationReleasesGlobalLock(t *testing.T) {
	live := &blockingTitleSession{
		controllableAgentSession: newControllableSession("thread-fresh"),
		started:                  make(chan struct{}),
		release:                  make(chan struct{}),
	}
	e := NewEngine("test", &controllableAgent{nextSession: live}, []Platform{&stubPlatformEngine{n: "test"}}, "", LangEnglish)
	managed := &Session{}
	done := make(chan struct{})

	go func() {
		e.getOrCreateInteractiveStateWith("test:user", e.platforms[0], "ctx", managed, e.sessions, nil, "", "首个问题")
		close(done)
	}()

	select {
	case <-live.started:
	case <-time.After(2 * time.Second):
		t.Fatal("title generation did not start")
	}
	if !e.interactiveMu.TryLock() {
		close(live.release)
		<-done
		t.Fatal("interactiveMu remained locked during title generation")
	}
	e.interactiveMu.Unlock()
	close(live.release)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("session creation did not finish after title generation")
	}
}

func TestGetOrCreateInteractiveStateWith_ResumeKeepsExistingTitle(t *testing.T) {
	live := &titleSyncAgentSession{controllableAgentSession: newControllableSession("thread-existing")}
	agent := &controllableAgent{nextSession: live}
	p := &stubPlatformEngine{n: "test"}
	e := NewEngine("test", agent, []Platform{p}, "", LangEnglish)
	managed := &Session{AgentSessionID: "thread-existing"}

	e.getOrCreateInteractiveStateWith(
		"test:user", p, "ctx", managed, e.sessions, nil, "", "不应覆盖已有标题",
	)

	if live.initialCalls != 0 {
		t.Fatalf("resume initialized title %d times, want 0", live.initialCalls)
	}
}

func TestGetOrCreateInteractiveStateWith_ResumeFallbackTitlesFreshSession(t *testing.T) {
	live := &titleSyncAgentSession{controllableAgentSession: newControllableSession("thread-recovered")}
	agent := &controllableAgent{
		startSessionFn: func(_ context.Context, sessionID string) (AgentSession, error) {
			if sessionID != "" {
				return nil, errors.New("stale session")
			}
			return live, nil
		},
	}
	p := &stubPlatformEngine{n: "test"}
	e := NewEngine("test", agent, []Platform{p}, "", LangEnglish)
	managed := &Session{AgentSessionID: "thread-stale"}

	e.getOrCreateInteractiveStateWith(
		"test:user", p, "ctx", managed, e.sessions, nil, "", "恢复失败后的新问题",
	)

	if live.initialCalls != 1 || live.initialPrompt != "恢复失败后的新问题" {
		t.Fatalf("fallback title calls = %d, prompt = %q", live.initialCalls, live.initialPrompt)
	}
}

func TestHandleRelay_TitlesFreshSessionAtCreation(t *testing.T) {
	live := &titleInitializingResultSession{AgentSession: newResultAgentSession("relay complete")}
	e := newTestEngine()
	e.agent = &controllableAgent{nextSession: live}

	response, err := e.HandleRelay(context.Background(), "source", "test:chat-1:user", "relay first request")
	if err != nil {
		t.Fatalf("HandleRelay() error = %v", err)
	}
	if response != "relay complete" {
		t.Fatalf("HandleRelay() response = %q", response)
	}
	if live.initialCalls != 1 || live.initialPrompt != "relay first request" {
		t.Fatalf("relay title calls = %d, prompt = %q", live.initialCalls, live.initialPrompt)
	}
}

func TestCmdName_SyncsCodexAppTitle(t *testing.T) {
	for _, tt := range []struct {
		name    string
		syncErr error
	}{
		{name: "success"},
		{name: "backend failure is fail-open", syncErr: errors.New("app-server unavailable")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := &stubPlatformEngine{n: "test"}
			e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
			key := "test:user"
			managed := e.sessions.GetOrCreateActive(key)
			managed.SetAgentSessionID("thread-1", "codex")
			live := &titleSyncAgentSession{
				controllableAgentSession: newControllableSession("thread-1"),
				err:                      tt.syncErr,
			}
			e.interactiveStates[key] = &interactiveState{agentSession: live, agent: e.agent}

			e.cmdName(p, &Message{SessionKey: key, ReplyCtx: "ctx"}, []string{"Readable title"})

			if got := e.sessions.GetSessionName("thread-1"); got != "Readable title" {
				t.Fatalf("local session name = %q, want Readable title", got)
			}
			if live.threadID != "thread-1" || live.title != "Readable title" {
				t.Fatalf("synced title = (%q, %q), want (thread-1, Readable title)", live.threadID, live.title)
			}
			if len(p.sent) != 1 {
				t.Fatalf("reply count = %d, want rename acknowledgement even when sync fails", len(p.sent))
			}
		})
	}
}
