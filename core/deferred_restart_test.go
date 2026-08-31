package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func prepareDeferredRestartTurn(t *testing.T, userID, sessionKey string) (*Engine, *interactiveState) {
	t.Helper()
	platform := &stubPlatformEngine{n: "feishu"}
	engine := NewEngine("demo", &stubAgent{}, []Platform{platform}, "", LangEnglish)
	engine.SetAdminFrom("admin-user")
	token, err := newAgentTurnToken()
	if err != nil {
		t.Fatal(err)
	}
	state := &interactiveState{
		agentSession:      &stubAgentSession{},
		platform:          platform,
		currentUserID:     userID,
		currentSessionKey: sessionKey,
		restartTurnToken:  token,
		presentationOpen:  true,
	}
	engine.interactiveStates[sessionKey] = state
	session := engine.sessions.GetOrCreateActive(sessionKey)
	if !session.TryLock() {
		t.Fatal("failed to mark test session busy")
	}
	t.Cleanup(session.Unlock)
	return engine, state
}

func TestRequestDeferredRestartUsesTrustedActiveTurnIdentity(t *testing.T) {
	drainTestRestartRequests()
	const sessionKey = "feishu:d:chat-1:admin-user:thread:omt-1"
	engine, state := prepareDeferredRestartTurn(t, "admin-user", sessionKey)

	req, err := engine.RequestDeferredRestart(state.restartTurnToken)
	if err != nil {
		t.Fatalf("RequestDeferredRestart() error = %v", err)
	}
	if req.SessionKey != sessionKey || req.Platform != "feishu" {
		t.Fatalf("restart request = %#v", req)
	}
	state.mu.Lock()
	fenced := state.restartAdmissionFenced
	state.mu.Unlock()
	if !fenced {
		t.Fatal("restart scheduling did not freeze new message admission")
	}
	if _, err := engine.RequestDeferredRestart(state.restartTurnToken); !errors.Is(err, ErrDeferredRestartAlreadyPending) {
		t.Fatalf("duplicate error = %v, want ErrDeferredRestartAlreadyPending", err)
	}
	select {
	case got := <-RestartCh:
		t.Fatalf("restart dispatched before turn completion: %#v", got)
	default:
	}
}

func TestRequestDeferredRestartFailsClosed(t *testing.T) {
	t.Run("non-admin", func(t *testing.T) {
		engine, state := prepareDeferredRestartTurn(t, "ordinary-user", "feishu:d:chat:user")
		if _, err := engine.RequestDeferredRestart(state.restartTurnToken); !errors.Is(err, ErrDeferredRestartUnauthorized) {
			t.Fatalf("error = %v, want ErrDeferredRestartUnauthorized", err)
		}
	})

	t.Run("no active turn", func(t *testing.T) {
		engine, state := prepareDeferredRestartTurn(t, "admin-user", "feishu:d:chat:admin")
		state.mu.Lock()
		state.presentationOpen = false
		state.mu.Unlock()
		if _, err := engine.RequestDeferredRestart(state.restartTurnToken); !errors.Is(err, ErrDeferredRestartNoActiveTurn) {
			t.Fatalf("error = %v, want ErrDeferredRestartNoActiveTurn", err)
		}
	})

	t.Run("wrong credential", func(t *testing.T) {
		engine, _ := prepareDeferredRestartTurn(t, "admin-user", "feishu:d:chat:admin")
		if _, err := engine.RequestDeferredRestart("other-turn-token"); !errors.Is(err, ErrDeferredRestartCredentialInvalid) {
			t.Fatalf("error = %v, want ErrDeferredRestartCredentialInvalid", err)
		}
	})

	t.Run("dead Agent session", func(t *testing.T) {
		engine, state := prepareDeferredRestartTurn(t, "admin-user", "feishu:d:chat:admin")
		state.mu.Lock()
		state.agentSession = nil
		state.mu.Unlock()
		if _, err := engine.RequestDeferredRestart(state.restartTurnToken); !errors.Is(err, ErrDeferredRestartNoActiveTurn) {
			t.Fatalf("error = %v, want ErrDeferredRestartNoActiveTurn", err)
		}
	})
}

func TestRequestDeferredRestartUsesCommittedSteerIdentityBeforeAdoption(t *testing.T) {
	const sessionKey = "feishu:d:chat:admin-user:thread:omt-steer-auth"
	engine, state := prepareDeferredRestartTurn(t, "admin-user", sessionKey)
	if !engine.commitSteerPresentation(sessionKey, steerHandoff{
		messageID: "steer-message", platform: &stubPlatformEngine{n: "feishu"},
		replyCtx: "steer-reply", userID: "ordinary-user",
	}) {
		t.Fatal("steer handoff was not committed")
	}
	state.mu.Lock()
	credential := state.restartTurnToken
	state.mu.Unlock()
	if _, err := engine.RequestDeferredRestart(credential); !errors.Is(err, ErrDeferredRestartUnauthorized) {
		t.Fatalf("error before handoff adoption = %v, want ErrDeferredRestartUnauthorized", err)
	}
}

func TestAgentTurnCredentialRotatesPerTurnAndClears(t *testing.T) {
	engine := NewEngine("demo", &stubAgent{}, nil, "", LangEnglish)
	path, err := newAgentTurnNoncePath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	secret, err := newAgentTurnToken()
	if err != nil {
		t.Fatal(err)
	}
	state := &interactiveState{restartSessionSecret: secret, restartNoncePath: path}
	engine.activateAgentTurnCredential(state, "admin-user", "feishu:d:chat:admin")
	state.mu.Lock()
	first := state.restartTurnToken
	state.mu.Unlock()
	if len(first) != 64 {
		t.Fatalf("first credential length = %d", len(first))
	}
	firstNonceBytes, err := os.ReadFile(path)
	firstNonce := strings.TrimSpace(string(firstNonceBytes))
	if err != nil || agentTurnToken(secret, firstNonce) != first {
		t.Fatalf("first nonce file = %q err=%v", firstNonceBytes, err)
	}
	otherSecret, err := newAgentTurnToken()
	if err != nil {
		t.Fatal(err)
	}
	if borrowed, err := BuildAgentTurnCredential(otherSecret, firstNonce); err != nil || borrowed == first {
		t.Fatalf("different Agent session secret reproduced turn credential: equal=%v err=%v", borrowed == first, err)
	}

	engine.activateAgentTurnCredential(state, "ordinary-user", "feishu:d:chat:ordinary")
	state.mu.Lock()
	second := state.restartTurnToken
	userID := state.currentUserID
	sessionKey := state.currentSessionKey
	state.mu.Unlock()
	if second == first || len(second) != 64 || userID != "ordinary-user" || sessionKey != "feishu:d:chat:ordinary" {
		t.Fatalf("rotated state token_changed=%v user=%q session=%q", second != first, userID, sessionKey)
	}
	secondNonceBytes, err := os.ReadFile(path)
	secondNonce := strings.TrimSpace(string(secondNonceBytes))
	if err != nil || secondNonce == firstNonce || agentTurnToken(secret, secondNonce) != second {
		t.Fatalf("second nonce file = %q err=%v", secondNonceBytes, err)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o644 {
		t.Fatalf("credential mode info=%v err=%v", info, err)
	}

	clearAgentTurnCredential(state, false)
	if data, err := os.ReadFile(path); err != nil || strings.TrimSpace(string(data)) != "" {
		t.Fatalf("cleared credential file = %q err=%v", data, err)
	}
	clearAgentTurnCredential(state, true)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("credential file still exists after cleanup: %v", err)
	}
}

func TestInteractiveAgentReceivesDedicatedTurnCredentialMarker(t *testing.T) {
	dataDir := t.TempDir()
	platform := &stubPlatformEngine{n: "feishu"}
	agent := &sessionEnvRecordingAgent{session: &stubAgentSession{}}
	engine := NewEngine("demo", agent, []Platform{platform}, "", LangEnglish)
	engine.SetDataDir(dataDir)
	const sessionKey = "feishu:d:chat:admin"
	session := engine.sessions.GetOrCreateActive(sessionKey)
	state := engine.getOrCreateInteractiveStateWith(sessionKey, platform, "reply", session, engine.sessions, nil, sessionKey, "hello")
	if got := agent.EnvValue(AgentTurnMarkerEnv); got != "1" {
		t.Fatalf("%s = %q, want 1", AgentTurnMarkerEnv, got)
	}
	secret := agent.EnvValue(AgentSessionSecretEnv)
	path := agent.EnvValue(AgentTurnNonceFileEnv)
	if len(secret) != 64 || path == "" || state.restartSessionSecret != secret || state.restartNoncePath != path {
		t.Fatalf("credential env/state secret_len=%d path=%q state_path=%q", len(secret), path, state.restartNoncePath)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o644 {
		t.Fatalf("credential path info=%v err=%v", info, err)
	}
	engine.cleanupInteractiveState(sessionKey, state)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("credential path survived session cleanup: %v", err)
	}
}

func TestHandleDeferredRestartSchedulesWithoutClientIdentity(t *testing.T) {
	const sessionKey = "feishu:d:chat-1:admin-user:thread:omt-1"
	engine, state := prepareDeferredRestartTurn(t, "admin-user", sessionKey)
	api := &APIServer{engines: map[string]*Engine{"demo": engine}}
	// Unknown identity fields are deliberately ignored; authorization and the
	// post-restart platform are taken from the active Engine state.
	body, err := json.Marshal(map[string]string{
		"credential": state.restartTurnToken, "project": "wrong-project", "session_key": "feishu:d:other:ordinary-user",
		"user_id": "ordinary-user", "platform": "telegram",
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/restart/defer", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	api.handleDeferredRestart(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202: %s", rec.Code, rec.Body.String())
	}
	var response DeferredRestartResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Status != "scheduled" || response.SessionKey != sessionKey || response.Platform != "feishu" {
		t.Fatalf("response = %#v", response)
	}
	if strings.Contains(rec.Body.String(), "ordinary-user") || strings.Contains(rec.Body.String(), "telegram") {
		t.Fatalf("response trusted client-supplied identity: %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	api.handleDeferredRestart(rec, httptest.NewRequest(http.MethodPost, "/restart/defer", bytes.NewReader(body)))
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), ErrDeferredRestartAlreadyPending.Error()) {
		t.Fatalf("duplicate status = %d, want 409: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleDeferredRestartMapsAuthorizationAndLifecycleErrors(t *testing.T) {
	tests := []struct {
		name     string
		userID   string
		open     bool
		wantCode int
	}{
		{name: "unauthorized", userID: "ordinary-user", open: true, wantCode: http.StatusForbidden},
		{name: "inactive", userID: "admin-user", open: false, wantCode: http.StatusConflict},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const sessionKey = "feishu:d:chat:user"
			engine, state := prepareDeferredRestartTurn(t, tt.userID, sessionKey)
			state.mu.Lock()
			state.presentationOpen = tt.open
			state.mu.Unlock()
			api := &APIServer{engines: map[string]*Engine{"demo": engine}}
			body, _ := json.Marshal(map[string]string{
				"credential": state.restartTurnToken, "project": "demo", "session_key": sessionKey,
				"user_id": "admin-user", "platform": "feishu",
			})
			req := httptest.NewRequest(http.MethodPost, "/restart/defer", bytes.NewReader(body))
			rec := httptest.NewRecorder()
			api.handleDeferredRestart(rec, req)
			if rec.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tt.wantCode, rec.Body.String())
			}
		})
	}
}

func TestHandleDeferredRestartRejectsExposedSessionWithoutCredential(t *testing.T) {
	const sessionKey = "feishu:d:chat:admin"
	engine, _ := prepareDeferredRestartTurn(t, "admin-user", sessionKey)
	api := &APIServer{engines: map[string]*Engine{"demo": engine}}
	body, _ := json.Marshal(map[string]string{
		"credential": strings.Repeat("f", 64),
		"project":    "demo", "session_key": sessionKey, "user_id": "admin-user",
	})
	rec := httptest.NewRecorder()
	api.handleDeferredRestart(rec, httptest.NewRequest(http.MethodPost, "/restart/defer", bytes.NewReader(body)))
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), ErrDeferredRestartCredentialInvalid.Error()) {
		t.Fatalf("status = %d, want 403: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleDeferredRestartCannotBorrowConcurrentAdminTurn(t *testing.T) {
	adminEngine, adminState := prepareDeferredRestartTurn(t, "admin-user", "feishu:d:admin-chat:admin-user")
	ordinaryEngine, ordinaryState := prepareDeferredRestartTurn(t, "ordinary-user", "feishu:d:ordinary-chat:ordinary-user")
	api := &APIServer{engines: map[string]*Engine{
		"admin-project":    adminEngine,
		"ordinary-project": ordinaryEngine,
	}}
	body, _ := json.Marshal(map[string]string{
		"credential": ordinaryState.restartTurnToken,
		// These exposed routing values name the admin turn but are ignored.
		"project": "admin-project", "session_key": adminState.currentSessionKey,
		"user_id": "admin-user", "platform": "feishu",
	})
	rec := httptest.NewRecorder()
	api.handleDeferredRestart(rec, httptest.NewRequest(http.MethodPost, "/restart/defer", bytes.NewReader(body)))
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), ErrDeferredRestartUnauthorized.Error()) {
		t.Fatalf("status = %d, want ordinary turn authorization failure: %s", rec.Code, rec.Body.String())
	}
	adminState.mu.Lock()
	adminPending := adminState.deferredRestart != nil
	adminState.mu.Unlock()
	if adminPending {
		t.Fatal("ordinary turn credential scheduled restart against concurrent admin turn")
	}
}

type deferredRestartLifecycleSession struct {
	events       chan Event
	firstStarted chan struct{}
	releaseFirst chan struct{}
	firstEvent   Event
	mu           sync.Mutex
	sends        int
	closed       bool
}

func newDeferredRestartLifecycleSession() *deferredRestartLifecycleSession {
	return &deferredRestartLifecycleSession{
		events:       make(chan Event, 4),
		firstStarted: make(chan struct{}),
		releaseFirst: make(chan struct{}),
		firstEvent:   Event{Type: EventResult, Content: "restart scheduled", Done: true},
	}
}

func (s *deferredRestartLifecycleSession) Send(_ string, _ []ImageAttachment, _ []FileAttachment) error {
	s.mu.Lock()
	s.sends++
	call := s.sends
	s.mu.Unlock()
	if call == 1 {
		close(s.firstStarted)
		<-s.releaseFirst
		s.events <- s.firstEvent
		return nil
	}
	s.events <- Event{Type: EventResult, Content: "queued turn completed", Done: true}
	return nil
}

func (s *deferredRestartLifecycleSession) RespondPermission(string, PermissionResult) error {
	return nil
}
func (s *deferredRestartLifecycleSession) Events() <-chan Event     { return s.events }
func (s *deferredRestartLifecycleSession) CurrentSessionID() string { return "codex-thread-1" }
func (s *deferredRestartLifecycleSession) Alive() bool              { return true }
func (s *deferredRestartLifecycleSession) Close() error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	return nil
}
func (s *deferredRestartLifecycleSession) sendCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sends
}
func (s *deferredRestartLifecycleSession) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

func TestDeferredRestartWaitsForWriterCardAndQueuedTurns(t *testing.T) {
	drainTestRestartRequests()
	session := newDeferredRestartLifecycleSession()
	platform := &steerRichCardPlatform{stubPlatformEngine: stubPlatformEngine{n: "feishu"}}
	engine := NewEngine("demo", &resultAgent{session: session}, []Platform{platform}, "", LangEnglish)
	engine.SetDisplayConfig(DisplayCfg{Mode: "full", CardMode: "rich", ThinkingMessages: true, ToolMessages: true})
	engine.SetAdminFrom("admin-user")
	const sessionKey = "feishu:d:chat-1:admin-user:thread:omt-1"

	engine.handleMessage(platform, &Message{
		SessionKey: sessionKey,
		MessageID:  "message-1",
		Platform:   "feishu",
		UserID:     "admin-user",
		Content:    "restart the daemon",
		ReplyCtx:   "reply-1",
	})
	select {
	case <-session.firstStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("first agent turn did not start")
	}

	deadline := time.Now().Add(2 * time.Second)
	credential := ""
	for {
		engine.interactiveMu.Lock()
		state := engine.interactiveStates[sessionKey]
		engine.interactiveMu.Unlock()
		if state != nil {
			state.mu.Lock()
			open := state.presentationOpen
			if open {
				credential = state.restartTurnToken
			}
			state.mu.Unlock()
			if open {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("turn presentation never opened")
		}
		time.Sleep(5 * time.Millisecond)
	}

	engine.handleMessage(platform, &Message{
		SessionKey: sessionKey,
		MessageID:  "message-2",
		Platform:   "feishu",
		UserID:     "ordinary-user",
		Content:    "queued follow-up",
		ReplyCtx:   "reply-2",
	})
	if _, err := engine.RequestDeferredRestart(credential); err != nil {
		t.Fatalf("schedule deferred restart: %v", err)
	}
	select {
	case got := <-RestartCh:
		t.Fatalf("restart dispatched while writer was active: %#v", got)
	default:
	}

	close(session.releaseFirst)
	select {
	case got := <-RestartCh:
		if got.SessionKey != sessionKey || got.Platform != "feishu" {
			t.Fatalf("restart request = %#v", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("deferred restart was not dispatched")
	}

	if session.sendCount() != 2 {
		t.Fatalf("agent sends = %d, want both original and queued turns", session.sendCount())
	}
	if engine.sessions.GetOrCreateActive(sessionKey).Busy() {
		t.Fatal("session writer remained busy when restart dispatched")
	}
	cardCalls := platform.cardCalls()
	cardTexts := make([]string, 0, len(cardCalls))
	doneCards := 0
	for _, call := range cardCalls {
		cardTexts = append(cardTexts, call.content)
		if call.kind == "update" && strings.Contains(call.content, "status=done") {
			doneCards++
		}
	}
	joined := strings.Join(cardTexts, "\n")
	if !strings.Contains(joined, "restart scheduled") || !strings.Contains(joined, "queued turn completed") {
		t.Fatalf("visible terminal replies were not delivered before restart: %q", joined)
	}
	if doneCards != 2 || strings.Contains(joined, "status=error") {
		t.Fatalf("cards did not reach successful terminal state before restart: done=%d calls=%+v", doneCards, cardCalls)
	}
	// main consumes RestartCh by stopping every Engine before exec. Verify that
	// this existing final step now closes the already-idle Agent session cleanly,
	// which releases Codex's persistent thread writer.
	if err := engine.Stop(); err != nil {
		t.Fatalf("Engine.Stop() after deferred restart: %v", err)
	}
	if !session.isClosed() {
		t.Fatal("Agent session was not closed during graceful restart shutdown")
	}
}

func TestDeferredRestartAuthorizationTracksAdoptedSteerUser(t *testing.T) {
	drainTestRestartRequests()
	h := newSteerCUJHarness(t, "feishu:d:chat:admin-user:thread:omt-steer")
	h.e.SetAdminFrom("admin-user")
	if !h.session.TryLock() {
		t.Fatal("failed to mark steer session busy")
	}
	defer h.session.Unlock()
	h.state.mu.Lock()
	h.state.currentUserID = "admin-user"
	h.state.currentSessionKey = h.key
	h.state.restartTurnToken = strings.Repeat("b", 64)
	h.state.mu.Unlock()

	msg := &Message{
		SessionKey: h.key,
		MessageID:  "m-non-admin",
		Content:    "steered by another participant",
		ReplyCtx:   "ctx-non-admin",
		Platform:   "feishu",
		UserID:     "ordinary-user",
	}
	if !h.e.trySteerBusyMessage(h.p, msg, h.key, h.session, h.e.sessions) {
		t.Fatal("non-admin steer was not accepted")
	}
	waitForSteer(t, "steer identity adoption", func() bool {
		h.state.mu.Lock()
		defer h.state.mu.Unlock()
		return h.state.currentUserID == "ordinary-user"
	})
	h.state.mu.Lock()
	credential := h.state.restartTurnToken
	h.state.mu.Unlock()
	if _, err := h.e.RequestDeferredRestart(credential); !errors.Is(err, ErrDeferredRestartUnauthorized) {
		t.Fatalf("error after non-admin steer = %v, want ErrDeferredRestartUnauthorized", err)
	}
	h.finish(t, "steered turn done")
}

func TestDeferredRestartDispatchesAfterSafeErrorTerminal(t *testing.T) {
	drainTestRestartRequests()
	session := newDeferredRestartLifecycleSession()
	session.firstEvent = Event{Type: EventError, Error: errors.New("agent turn failed after scheduling")}
	platform := &steerRichCardPlatform{stubPlatformEngine: stubPlatformEngine{n: "feishu"}}
	engine := NewEngine("demo", &resultAgent{session: session}, []Platform{platform}, "", LangEnglish)
	engine.SetDisplayConfig(DisplayCfg{Mode: "full", CardMode: "rich", ThinkingMessages: true, ToolMessages: true})
	engine.SetAdminFrom("admin-user")
	const sessionKey = "feishu:d:chat-1:admin-user:thread:omt-error"

	engine.handleMessage(platform, &Message{
		SessionKey: sessionKey, MessageID: "message-error", Platform: "feishu",
		UserID: "admin-user", Content: "restart even if final rendering fails", ReplyCtx: "reply-error",
	})
	select {
	case <-session.firstStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("agent turn did not start")
	}
	waitForSteer(t, "error turn presentation", func() bool {
		engine.interactiveMu.Lock()
		state := engine.interactiveStates[sessionKey]
		engine.interactiveMu.Unlock()
		if state == nil {
			return false
		}
		state.mu.Lock()
		defer state.mu.Unlock()
		return state.presentationOpen
	})
	engine.interactiveMu.Lock()
	state := engine.interactiveStates[sessionKey]
	engine.interactiveMu.Unlock()
	state.mu.Lock()
	credential := state.restartTurnToken
	state.mu.Unlock()
	if _, err := engine.RequestDeferredRestart(credential); err != nil {
		t.Fatalf("schedule deferred restart: %v", err)
	}
	close(session.releaseFirst)
	select {
	case <-RestartCh:
	case <-time.After(3 * time.Second):
		t.Fatal("deferred restart was not dispatched after EventError")
	}
	if engine.sessions.GetOrCreateActive(sessionKey).Busy() {
		t.Fatal("session writer remained busy after EventError")
	}
	joined := ""
	for _, call := range platform.cardCalls() {
		joined += "\n" + call.content
	}
	if !strings.Contains(joined, "status=error") {
		t.Fatalf("error terminal card was not delivered before restart: %s", joined)
	}
}

func TestDrainPendingMessagesFencesAdmissionBeforeDeferredRestart(t *testing.T) {
	platform := &stubPlatformEngine{n: "feishu"}
	engine := NewEngine("demo", &stubAgent{}, []Platform{platform}, "", LangEnglish)
	const sessionKey = "feishu:d:chat:admin-user:thread:omt-fence"
	request := RestartRequest{SessionKey: sessionKey, Platform: "feishu"}
	state := &interactiveState{
		agentSession:    &stubAgentSession{},
		platform:        platform,
		deferredRestart: &request,
	}
	engine.interactiveStates[sessionKey] = state
	session := engine.sessions.GetOrCreateActive(sessionKey)
	if !session.TryLock() {
		t.Fatal("failed to mark session busy")
	}
	defer session.Unlock()

	unlocked := engine.drainPendingMessages(state, session, engine.sessions, sessionKey)
	if unlocked {
		t.Fatal("drain unlocked the session before deferred restart admission was fenced")
	}
	if !session.Busy() {
		t.Fatal("session must remain busy until the restart request is handed off")
	}
	state.mu.Lock()
	fenced := state.restartAdmissionFenced
	state.mu.Unlock()
	if !fenced {
		t.Fatal("interactive state did not reject new turns during restart handoff")
	}

	// Reproduce the old unlock-before-dispatch window: even if the previous
	// processor releases its session lock, a newly arriving message observes the
	// fence and gets an explicit restarting response instead of starting a turn.
	session.Unlock()
	engine.handleMessage(platform, &Message{
		SessionKey: sessionKey, MessageID: "message-after-fence", Platform: "feishu",
		UserID: "admin-user", Content: "do not start this turn", ReplyCtx: "reply-after-fence",
	})
	if session.Busy() {
		t.Fatal("fenced message acquired and retained the session writer")
	}
	if sent := strings.Join(platform.getSent(), "\n"); !strings.Contains(sent, engine.i18n.T(MsgRestarting)) {
		t.Fatalf("fenced message did not receive restart lifecycle reply: %q", sent)
	}
}
