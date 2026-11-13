package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The only mocks in these journeys are the native agent and the remote chat.
// A new Engine reads the real on-disk SessionManager/checkpoints after Stop.
type recoveryNativeLedger struct {
	mu           sync.Mutex
	starts       []string
	sends        []string
	recoveries   []string
	sessions     []*recoveryNativeSession
	answer       string
	autoReply    bool
	gracefulStop bool
	cancels      int
	steer        bool
	steers       []string
}

type recoveryNativeAgent struct{ ledger *recoveryNativeLedger }

func (*recoveryNativeAgent) Name() string               { return "recovery-native" }
func (*recoveryNativeAgent) SupportsTurnRecovery() bool { return true }
func (*recoveryNativeAgent) Stop() error                { return nil }
func (*recoveryNativeAgent) ListSessions(context.Context) ([]AgentSessionInfo, error) {
	return nil, nil
}
func (a *recoveryNativeAgent) StartSession(_ context.Context, id string) (AgentSession, error) {
	a.ledger.mu.Lock()
	defer a.ledger.mu.Unlock()
	a.ledger.starts = append(a.ledger.starts, id)
	if id == "" {
		id = fmt.Sprintf("native-%d", len(a.ledger.starts))
	}
	s := &recoveryNativeSession{id: id, ledger: a.ledger, events: make(chan Event, 16)}
	a.ledger.sessions = append(a.ledger.sessions, s)
	if a.ledger.gracefulStop {
		return &recoveryCancellableSession{s}, nil
	}
	if a.ledger.steer {
		return &recoverySteerSession{s}, nil
	}
	return s, nil
}

type recoveryNativeSession struct {
	id     string
	ledger *recoveryNativeLedger
	events chan Event
	closed atomic.Bool
}

func (s *recoveryNativeSession) CurrentSessionID() string                       { return s.id }
func (s *recoveryNativeSession) Alive() bool                                    { return !s.closed.Load() }
func (s *recoveryNativeSession) Close() error                                   { s.closed.Store(true); return nil }
func (s *recoveryNativeSession) Events() <-chan Event                           { return s.events }
func (*recoveryNativeSession) RespondPermission(string, PermissionResult) error { return nil }
func (s *recoveryNativeSession) Send(prompt string, _ []ImageAttachment, _ []FileAttachment) error {
	s.ledger.mu.Lock()
	s.ledger.sends = append(s.ledger.sends, prompt)
	auto := s.ledger.autoReply
	n := len(s.ledger.sends)
	s.ledger.mu.Unlock()
	if auto {
		s.events <- Event{Type: EventResult, Done: true, Content: fmt.Sprintf("queued answer %d", n)}
	}
	return nil
}
func (s *recoveryNativeSession) SendWithTurnOptions(prompt string, images []ImageAttachment, files []FileAttachment, _ TurnOptions) error {
	return s.Send(prompt, images, files)
}
func (s *recoveryNativeSession) PrepareTurnRecovery(TurnRecoveryInput) (json.RawMessage, error) {
	return json.Marshal(s.id)
}
func (s *recoveryNativeSession) ResumeInterruptedTurn(checkpoint json.RawMessage, input TurnRecoveryInput, _ func(json.RawMessage) error) error {
	var id string
	if err := json.Unmarshal(checkpoint, &id); err != nil {
		return err
	}
	if id != s.id {
		return fmt.Errorf("native checkpoint %s belongs to another session, got %s", id, s.id)
	}
	s.ledger.mu.Lock()
	s.ledger.recoveries = append(s.ledger.recoveries, input.Prompt)
	answer := s.ledger.answer
	s.ledger.mu.Unlock()
	if answer != "" {
		s.events <- Event{Type: EventResult, Done: true, Content: answer}
	}
	return nil
}

type recoveryCancellableSession struct{ *recoveryNativeSession }

func (s *recoveryCancellableSession) CancelTurn() error {
	s.ledger.mu.Lock()
	s.ledger.cancels++
	s.ledger.mu.Unlock()
	// Deliberately delay the native interrupted event. /stop must persist its
	// intent before waiting for an acknowledgement that can be lost at restart.
	return nil
}

type recoverySteerSession struct{ *recoveryNativeSession }

func (s *recoverySteerSession) Steer(prompt string, _ []ImageAttachment, _ []FileAttachment) error {
	s.ledger.mu.Lock()
	s.ledger.steers = append(s.ledger.steers, prompt)
	s.ledger.mu.Unlock()
	return nil
}

func (l *recoveryNativeLedger) counts() (sends, recoveries, starts int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.sends), len(l.recoveries), len(l.starts)
}
func (l *recoveryNativeLedger) complete(answer string) {
	l.mu.Lock()
	s := l.sessions[len(l.sessions)-1]
	l.mu.Unlock()
	s.events <- Event{Type: EventResult, Done: true, Content: answer}
}

type recoveryCardPlatform struct {
	stubRichCardSilentPlatform
	recoveryMu     sync.Mutex
	updatedHandles []string
	visible        map[string]string
	blockFinal     bool
	finalAttempt   chan struct{}
	checkpoints    map[string]func(json.RawMessage) error
	restoreError   string
	restores       int
}

func newRecoveryCardPlatform() *recoveryCardPlatform {
	return &recoveryCardPlatform{stubRichCardSilentPlatform: stubRichCardSilentPlatform{
		stubPlatformEngine: stubPlatformEngine{n: "recovery-chat"},
	}, visible: make(map[string]string), checkpoints: make(map[string]func(json.RawMessage) error)}
}
func (p *recoveryCardPlatform) SnapshotTurnReplyCtx(ctx any) (json.RawMessage, error) {
	return json.Marshal(ctx)
}
func (p *recoveryCardPlatform) RestoreTurnReplyCtx(b json.RawMessage) (any, error) {
	var ctx string
	err := json.Unmarshal(b, &ctx)
	return ctx, err
}
func (p *recoveryCardPlatform) CheckpointPreview(handle any, save func(json.RawMessage) error) error {
	p.recoveryMu.Lock()
	p.checkpoints[fmt.Sprint(handle)] = save
	p.recoveryMu.Unlock()
	b, err := json.Marshal(handle)
	if err != nil {
		return err
	}
	return save(b)
}
func (p *recoveryCardPlatform) RestorePreview(b json.RawMessage, save func(json.RawMessage) error) (any, error) {
	p.recoveryMu.Lock()
	p.restores++
	failure := p.restoreError
	p.recoveryMu.Unlock()
	if failure != "" {
		return nil, errors.New(failure)
	}
	var handle string
	if err := json.Unmarshal(b, &handle); err != nil {
		return nil, err
	}
	if err := p.CheckpointPreview(handle, save); err != nil {
		return nil, err
	}
	return handle, nil
}
func (p *recoveryCardPlatform) checkpointHandle(handle any) error {
	p.recoveryMu.Lock()
	save := p.checkpoints[fmt.Sprint(handle)]
	p.recoveryMu.Unlock()
	if save == nil {
		return nil
	}
	b, err := json.Marshal(handle)
	if err != nil {
		return err
	}
	return save(b)
}
func (p *recoveryCardPlatform) UpdateMessage(ctx context.Context, handle any, content string) error {
	if err := p.checkpointHandle(handle); err != nil {
		return err
	}
	p.recoveryMu.Lock()
	block := p.blockFinal && strings.Contains(content, "status=done")
	attempt := p.finalAttempt
	if block {
		p.recoveryMu.Unlock()
		select {
		case attempt <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return ctx.Err()
	}
	p.updatedHandles = append(p.updatedHandles, fmt.Sprint(handle))
	p.visible[fmt.Sprint(handle)] = content
	p.recoveryMu.Unlock()
	return p.stubRichCardSilentPlatform.UpdateMessage(ctx, handle, content)
}
func (p *recoveryCardPlatform) StreamRichCardText(ctx context.Context, handle any, content string) error {
	if err := p.checkpointHandle(handle); err != nil {
		return err
	}
	return p.stubRichCardSilentPlatform.StreamRichCardText(ctx, handle, content)
}
func (p *recoveryCardPlatform) finalContains(handle, content string) bool {
	p.recoveryMu.Lock()
	defer p.recoveryMu.Unlock()
	return strings.Contains(p.visible[handle], "status=done") && strings.Contains(p.visible[handle], content)
}
func (p *recoveryCardPlatform) startCount() int { starts, _, _, _ := p.snapshot(); return len(starts) }

type recoveryAsyncPlatform struct {
	*recoveryCardPlatform
	lifecycle PlatformLifecycleHandler
}

func (p *recoveryAsyncPlatform) SetLifecycleHandler(h PlatformLifecycleHandler) { p.lifecycle = h }

func recoveryEngine(t *testing.T, name, store string, ledger *recoveryNativeLedger, p Platform) *Engine {
	t.Helper()
	e := NewEngine(name, &recoveryNativeAgent{ledger}, []Platform{p}, store, LangEnglish)
	e.SetDisplayConfig(DisplayCfg{Mode: "full", CardMode: "rich"})
	e.SetBusyMessageMode(BusyMessageModeQueue)
	e.SetAdminFrom("alice")
	if err := e.Start(); err != nil {
		t.Fatal(err)
	}
	return e
}
func recoveryUserMessage(id, content string) *Message {
	return &Message{Platform: "recovery-chat", SessionKey: "recovery-chat:alice", MessageID: id,
		UserID: "alice", UserName: "Alice", Content: content, ReplyCtx: "reply:" + id}
}
func recoveryWait(t *testing.T, reason string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", reason)
}
func stopRecoveryEngine(t *testing.T, e *Engine) {
	t.Helper()
	if err := e.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestChatRecovery_RestartResumesSameTurnAndCard(t *testing.T) {
	store := filepath.Join(t.TempDir(), "sessions.json")
	ledger := &recoveryNativeLedger{answer: "finished original request"}
	p := newRecoveryCardPlatform()
	e := recoveryEngine(t, "project", store, ledger, p)
	e.ReceiveMessage(p, recoveryUserMessage("m1", "finish my original task"))
	recoveryWait(t, "original model execution and visible card", func() bool { n, _, _ := ledger.counts(); return n == 1 && p.startCount() == 1 })
	stopRecoveryEngine(t, e)
	_, _, updates, deletes := p.snapshot()
	for _, update := range updates {
		if strings.Contains(update, "status=error") {
			t.Fatalf("restart turned the user's card into a failure: %s", update)
		}
	}
	if deletes != 0 {
		t.Fatalf("restart deleted %d existing answer cards", deletes)
	}

	// No ReceiveMessage call is made to this fresh Engine.
	e2 := recoveryEngine(t, "project", store, ledger, p)
	recoveryWait(t, "automatic final answer on the original card", func() bool { return p.finalContains("handle-1", "finished original request") })
	if p.startCount() != 1 {
		t.Fatalf("recovery created a duplicate answer card; starts=%d", p.startCount())
	}
	if sends, recoveries, _ := ledger.counts(); sends != 1 || recoveries != 1 {
		t.Fatalf("original input was replayed: sends=%d recoveries=%d", sends, recoveries)
	}
	ledger.mu.Lock()
	starts := append([]string(nil), ledger.starts...)
	ledger.mu.Unlock()
	if len(starts) != 2 || starts[1] != "native-1" {
		t.Fatalf("recovery started a different native conversation: %v", starts)
	}
	recoveryWait(t, "completed turn releases session", func() bool { return !e2.sessions.GetOrCreateActive("recovery-chat:alice").Busy() })
	stopRecoveryEngine(t, e2)
	e3 := recoveryEngine(t, "project", store, ledger, p)
	stopRecoveryEngine(t, e3)
	if _, recoveries, _ := ledger.counts(); recoveries != 1 {
		t.Fatal("an already delivered answer was revived by another restart")
	}
}

func TestCUJ_R1_RestartContinuesOriginalThenQueuedRequests(t *testing.T) {
	store := filepath.Join(t.TempDir(), "sessions.json")
	ledger := &recoveryNativeLedger{answer: "original completed"}
	p := newRecoveryCardPlatform()
	e := recoveryEngine(t, "project", store, ledger, p)
	e.ReceiveMessage(p, recoveryUserMessage("m1", "first request"))
	recoveryWait(t, "first request active", func() bool { n, _, _ := ledger.counts(); return n == 1 && p.startCount() == 1 })
	e.ReceiveMessage(p, recoveryUserMessage("m2", "second request"))
	e.ReceiveMessage(p, recoveryUserMessage("m3", "third request"))
	stopRecoveryEngine(t, e)
	ledger.mu.Lock()
	ledger.autoReply = true
	ledger.mu.Unlock()
	e2 := recoveryEngine(t, "project", store, ledger, p)
	recoveryWait(t, "all three answers after restart", func() bool {
		return p.finalContains("handle-1", "original completed") && p.finalContains("handle-2", "queued answer 2") && p.finalContains("handle-3", "queued answer 3")
	})
	ledger.mu.Lock()
	prompts := append([]string(nil), ledger.sends...)
	ledger.mu.Unlock()
	if len(prompts) != 3 || !strings.Contains(prompts[1], "second request") || !strings.Contains(prompts[2], "third request") {
		t.Fatalf("accepted queue order changed: %v", prompts)
	}
	if p.startCount() != 3 {
		t.Fatalf("expected one card per original user request, got %d", p.startCount())
	}
	recoveryWait(t, "queue drained", func() bool { return !e2.sessions.GetOrCreateActive("recovery-chat:alice").Busy() })
	stopRecoveryEngine(t, e2)
}

func TestChatRecovery_UserStopBeforeNativeAcknowledgementIsDurable(t *testing.T) {
	store := filepath.Join(t.TempDir(), "sessions.json")
	ledger := &recoveryNativeLedger{answer: "must not return", gracefulStop: true}
	p := newRecoveryCardPlatform()
	e := recoveryEngine(t, "project", store, ledger, p)
	e.ReceiveMessage(p, recoveryUserMessage("m1", "long running task"))
	recoveryWait(t, "task active", func() bool { n, _, _ := ledger.counts(); return n == 1 && p.startCount() == 1 })
	e.ReceiveMessage(p, recoveryUserMessage("queued", "also cancel this queued request"))
	e.ReceiveMessage(p, recoveryUserMessage("stop", "/stop"))
	stopRecoveryEngine(t, e)
	e2 := recoveryEngine(t, "project", store, ledger, p)
	time.Sleep(50 * time.Millisecond)
	stopRecoveryEngine(t, e2)
	if _, recoveries, _ := ledger.counts(); recoveries != 0 {
		t.Fatalf("/stop was lost before native acknowledgement; revived %d times", recoveries)
	}
}

func TestChatRecovery_InvalidSavedCardPreventsNativeRecovery(t *testing.T) {
	store := filepath.Join(t.TempDir(), "sessions.json")
	ledger := &recoveryNativeLedger{answer: "must not run without restored delivery"}
	p := newRecoveryCardPlatform()
	e := recoveryEngine(t, "project", store, ledger, p)
	e.ReceiveMessage(p, recoveryUserMessage("m1", "original request"))
	recoveryWait(t, "original request", func() bool { n, _, _ := ledger.counts(); return n == 1 && p.startCount() == 1 })
	stopRecoveryEngine(t, e)
	p.recoveryMu.Lock()
	p.restoreError = "preview belongs to a different bot"
	p.recoveryMu.Unlock()
	e2 := recoveryEngine(t, "project", store, ledger, p)
	recoveryWait(t, "failed recovery is visible and turn released", func() bool {
		p.recoveryMu.Lock()
		attempted := p.restores > 0
		p.recoveryMu.Unlock()
		return attempted && !e2.sessions.GetOrCreateActive("recovery-chat:alice").Busy()
	})
	stopRecoveryEngine(t, e2)
	if sends, recoveries, _ := ledger.counts(); sends != 1 || recoveries != 0 {
		t.Fatalf("unusable saved target still ran work: sends=%d recoveries=%d", sends, recoveries)
	}
}

func TestChatRecovery_ScheduledFailurePresentationStaysIndependentOfQueuedChat(t *testing.T) {
	store := filepath.Join(t.TempDir(), "sessions.json")
	ledger := &recoveryNativeLedger{}
	p := newRecoveryCardPlatform()
	e := recoveryEngine(t, "project", store, ledger, p)
	msg := recoveryUserMessage("scheduled", "scheduled task")
	msg.scheduledResult = &scheduledTurnResult{}
	e.ReceiveMessage(p, msg)
	recoveryWait(t, "scheduled request", func() bool { n, _, _ := ledger.counts(); return n == 1 && p.startCount() == 1 })
	e.ReceiveMessage(p, recoveryUserMessage("normal", "ordinary queued task"))
	stopRecoveryEngine(t, e)
	p.recoveryMu.Lock()
	scheduledCard := p.visible["handle-1"]
	p.recoveryMu.Unlock()
	if !strings.Contains(scheduledCard, "status=error") {
		t.Fatalf("non-recoverable scheduled card was left running because a chat was queued: %s", scheduledCard)
	}
	ledger.mu.Lock()
	ledger.autoReply = true
	ledger.mu.Unlock()
	e2 := recoveryEngine(t, "project", store, ledger, p)
	recoveryWait(t, "only the ordinary queued task completes", func() bool { return p.finalContains("handle-2", "queued answer 2") })
	stopRecoveryEngine(t, e2)
	if sends, recoveries, _ := ledger.counts(); sends != 2 || recoveries != 0 {
		t.Fatalf("scheduled task was resumed: sends=%d recoveries=%d", sends, recoveries)
	}
}

func TestChatRecovery_StopAndRecallBeforePlatformReady(t *testing.T) {
	for _, action := range []string{"stop", "new", "recall"} {
		t.Run(action, func(t *testing.T) {
			store := filepath.Join(t.TempDir(), "sessions.json")
			ledger := &recoveryNativeLedger{answer: "must not return"}
			p := newRecoveryCardPlatform()
			e := recoveryEngine(t, "project", store, ledger, p)
			e.ReceiveMessage(p, recoveryUserMessage("m1", "long task"))
			recoveryWait(t, "task active", func() bool { n, _, _ := ledger.counts(); return n == 1 && p.startCount() == 1 })
			stopRecoveryEngine(t, e)
			async := &recoveryAsyncPlatform{recoveryCardPlatform: p}
			e2 := recoveryEngine(t, "project", store, ledger, async)
			if _, n, _ := ledger.counts(); n != 0 {
				t.Fatal("resumed before the reply platform was ready")
			}
			if action == "stop" || action == "new" {
				e2.ReceiveMessage(async, recoveryUserMessage(action, "/"+action))
			} else {
				msg := recoveryUserMessage("m1", "")
				msg.Recalled = true
				e2.ReceiveMessage(async, msg)
			}
			async.lifecycle.OnPlatformReady(async)
			time.Sleep(50 * time.Millisecond)
			stopRecoveryEngine(t, e2)
			if _, n, _ := ledger.counts(); n != 0 {
				t.Fatalf("%s while recovering was ignored; recoveries=%d", action, n)
			}
		})
	}
}

func TestChatRecovery_CompletedNativeAnswerIsOnlyDeliveredAfterRestart(t *testing.T) {
	store := filepath.Join(t.TempDir(), "sessions.json")
	ledger := &recoveryNativeLedger{answer: "already computed answer"}
	p := newRecoveryCardPlatform()
	p.blockFinal, p.finalAttempt = true, make(chan struct{}, 1)
	e := recoveryEngine(t, "project", store, ledger, p)
	e.ReceiveMessage(p, recoveryUserMessage("m1", "compute only once"))
	recoveryWait(t, "original request", func() bool { n, _, _ := ledger.counts(); return n == 1 && p.startCount() == 1 })
	ledger.complete("already computed answer")
	select {
	case <-p.finalAttempt:
	case <-time.After(3 * time.Second):
		t.Fatal("native answer did not reach the blocked delivery boundary")
	}
	stopRecoveryEngine(t, e)
	p.recoveryMu.Lock()
	p.blockFinal = false
	p.recoveryMu.Unlock()
	e2 := recoveryEngine(t, "project", store, ledger, p)
	recoveryWait(t, "saved native result is delivered", func() bool { return p.finalContains("handle-1", "already computed answer") })
	if sends, recoveries, _ := ledger.counts(); sends != 1 || recoveries != 1 {
		t.Fatalf("completed native work was executed again: sends=%d recoveries=%d", sends, recoveries)
	}
	if p.startCount() != 1 {
		t.Fatalf("late result created %d cards", p.startCount())
	}
	stopRecoveryEngine(t, e2)
}

func TestChatRecovery_NewInputWhileWaitingForReadinessStaysQueued(t *testing.T) {
	store := filepath.Join(t.TempDir(), "sessions.json")
	ledger := &recoveryNativeLedger{answer: "original answer"}
	p := newRecoveryCardPlatform()
	e := recoveryEngine(t, "project", store, ledger, p)
	e.ReceiveMessage(p, recoveryUserMessage("m1", "original task"))
	recoveryWait(t, "original request", func() bool { n, _, _ := ledger.counts(); return n == 1 && p.startCount() == 1 })
	stopRecoveryEngine(t, e)
	ledger.mu.Lock()
	ledger.autoReply = true
	ledger.mu.Unlock()
	async := &recoveryAsyncPlatform{recoveryCardPlatform: p}
	e2 := recoveryEngine(t, "project", store, ledger, async)
	e2.ReceiveMessage(async, recoveryUserMessage("m2", "new task during recovery"))
	if sends, recoveries, _ := ledger.counts(); sends != 1 || recoveries != 0 {
		t.Fatalf("new input overtook unready recovery: sends=%d recoveries=%d", sends, recoveries)
	}
	async.lifecycle.OnPlatformReady(async)
	recoveryWait(t, "original then new queued answer", func() bool {
		return p.finalContains("handle-1", "original answer") && p.finalContains("handle-2", "queued answer 2")
	})
	stopRecoveryEngine(t, e2)
}

func TestChatRecovery_ScheduledTurnIsNotReplayed(t *testing.T) {
	store := filepath.Join(t.TempDir(), "sessions.json")
	ledger := &recoveryNativeLedger{answer: "must not replay scheduled work"}
	p := newRecoveryCardPlatform()
	e := recoveryEngine(t, "project", store, ledger, p)
	msg := recoveryUserMessage("scheduled", "scheduled work")
	msg.scheduledResult = &scheduledTurnResult{}
	e.ReceiveMessage(p, msg)
	recoveryWait(t, "scheduled execution", func() bool { n, _, _ := ledger.counts(); return n == 1 && p.startCount() == 1 })
	stopRecoveryEngine(t, e)
	e2 := recoveryEngine(t, "project", store, ledger, p)
	stopRecoveryEngine(t, e2)
	if sends, recoveries, _ := ledger.counts(); sends != 1 || recoveries != 0 {
		t.Fatalf("scheduler work entered chat recovery: sends=%d recoveries=%d", sends, recoveries)
	}
}

func TestChatRecovery_SteerFreezingOldCardCannotOverwriteRecoveryTarget(t *testing.T) {
	store := filepath.Join(t.TempDir(), "sessions.json")
	ledger := &recoveryNativeLedger{answer: "answer with accepted supplement", steer: true}
	p := newRecoveryCardPlatform()
	e := recoveryEngine(t, "project", store, ledger, p)
	e.SetBusyMessageMode(BusyMessageModeSteer)
	first := recoveryUserMessage("m1", "original task")
	first.UserMessageTimeMs = 1000
	e.ReceiveMessage(p, first)
	recoveryWait(t, "original execution", func() bool { n, _, _ := ledger.counts(); return n == 1 && p.startCount() == 1 })
	supplement := recoveryUserMessage("m2", "accepted supplement")
	supplement.UserMessageTimeMs = 3000
	e.ReceiveMessage(p, supplement)
	recoveryWait(t, "old answer card freezes after steering", func() bool {
		p.recoveryMu.Lock()
		defer p.recoveryMu.Unlock()
		return strings.Contains(p.visible["handle-1"], "status=redirected")
	})
	stopRecoveryEngine(t, e)
	e2 := recoveryEngine(t, "project", store, ledger, p)
	recoveryWait(t, "latest user card receives resumed answer", func() bool { return p.finalContains("handle-2", "answer with accepted supplement") })
	if p.startCount() != 2 {
		t.Fatalf("steer recovery created duplicate cards: %d", p.startCount())
	}
	p.recoveryMu.Lock()
	old := p.visible["handle-1"]
	p.recoveryMu.Unlock()
	if !strings.Contains(old, "status=redirected") {
		t.Fatalf("old redirected card was overwritten after restart: %s", old)
	}
	recoveryWait(t, "recovered answer finishes", func() bool { return !e2.sessions.GetOrCreateActive("recovery-chat:alice").Busy() })
	late := recoveryUserMessage("old-network-redelivery", "older already superseded input")
	late.UserMessageTimeMs = 2000
	e2.ReceiveMessage(p, late)
	time.Sleep(50 * time.Millisecond)
	stopRecoveryEngine(t, e2)
	if sends, _, _ := ledger.counts(); sends != 1 {
		t.Fatalf("restart lost the latest accepted message watermark and replayed stale input; sends=%d", sends)
	}
}

func TestChatRecovery_CompletedAnswerDoesNotRunAgain(t *testing.T) {
	store := filepath.Join(t.TempDir(), "sessions.json")
	ledger := &recoveryNativeLedger{autoReply: true}
	p := newRecoveryCardPlatform()
	e := recoveryEngine(t, "project", store, ledger, p)
	e.ReceiveMessage(p, recoveryUserMessage("done", "finish now"))
	recoveryWait(t, "completed answer", func() bool {
		return p.finalContains("handle-1", "queued answer 1") && !e.sessions.GetOrCreateActive("recovery-chat:alice").Busy()
	})
	stopRecoveryEngine(t, e)
	e2 := recoveryEngine(t, "project", store, ledger, p)
	stopRecoveryEngine(t, e2)
	if sends, recoveries, _ := ledger.counts(); sends != 1 || recoveries != 0 {
		t.Fatalf("completed task was revived: sends=%d recoveries=%d", sends, recoveries)
	}
}

func TestChatRecovery_ProjectIdentityCannotConsumeOtherCheckpoint(t *testing.T) {
	store := filepath.Join(t.TempDir(), "sessions.json")
	ledger := &recoveryNativeLedger{answer: "original project answer"}
	p := newRecoveryCardPlatform()
	e := recoveryEngine(t, "project-a", store, ledger, p)
	e.ReceiveMessage(p, recoveryUserMessage("m1", "task for project A"))
	recoveryWait(t, "original execution", func() bool { n, _, _ := ledger.counts(); return n == 1 && p.startCount() == 1 })
	stopRecoveryEngine(t, e)
	other := recoveryEngine(t, "project-b", store, ledger, p)
	stopRecoveryEngine(t, other)
	if _, n, _ := ledger.counts(); n != 0 {
		t.Fatal("another project consumed this project's checkpoint")
	}
	e2 := recoveryEngine(t, "project-a", store, ledger, p)
	recoveryWait(t, "owning project recovers", func() bool { return p.finalContains("handle-1", "original project answer") })
	stopRecoveryEngine(t, e2)
}

// A signature repair replaces the native conversation during the same bridge
// turn. Its acknowledgement must establish both durable identities before the
// replacement native thread can execute anything.
type replacingRecoveryAgent struct {
	recoveryNativeAgent
	mu   sync.Mutex
	last *replacingRecoverySession
}

type replacingRecoverySession struct {
	*recoveryNativeSession
	thread atomic.Value
}

func (a *replacingRecoveryAgent) StartSession(ctx context.Context, id string) (AgentSession, error) {
	backend, err := a.recoveryNativeAgent.StartSession(ctx, id)
	if err != nil {
		return nil, err
	}
	s := &replacingRecoverySession{recoveryNativeSession: backend.(*recoveryNativeSession)}
	s.thread.Store(s.id)
	a.mu.Lock()
	a.last = s
	a.mu.Unlock()
	return s, nil
}

func (s *replacingRecoverySession) CurrentSessionID() string { return s.thread.Load().(string) }

func (a *replacingRecoveryAgent) replaceThread(id string) <-chan error {
	a.mu.Lock()
	s := a.last
	a.mu.Unlock()
	s.thread.Store(id)
	checkpoint, _ := json.Marshal(id)
	ready := make(chan error, 1)
	s.events <- Event{Type: EventSessionRecovery, SessionID: id, RecoveryCheckpoint: checkpoint, RecoveryReady: ready}
	return ready
}

func replacingRecoveryEngine(t *testing.T, store string, ledger *recoveryNativeLedger, p *recoveryCardPlatform) (*Engine, *replacingRecoveryAgent) {
	t.Helper()
	a := &replacingRecoveryAgent{recoveryNativeAgent: recoveryNativeAgent{ledger: ledger}}
	e := NewEngine("project", a, []Platform{p}, store, LangEnglish)
	e.SetDisplayConfig(DisplayCfg{Mode: "full", CardMode: "rich"})
	if err := e.Start(); err != nil {
		t.Fatal(err)
	}
	return e, a
}

func readOnlyChatCheckpoint(t *testing.T, store string) (string, map[string]json.RawMessage) {
	t.Helper()
	paths, err := filepath.Glob(store + ".recovery/*/state.json")
	if err != nil || len(paths) != 1 {
		t.Fatalf("expected one durable chat checkpoint: paths=%v err=%v", paths, err)
	}
	b, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	var saved map[string]json.RawMessage
	if err := json.Unmarshal(b, &saved); err != nil {
		t.Fatal(err)
	}
	return paths[0], saved
}

func TestChatRecovery_SignatureReplacementAcknowledgementSurvivesRestart(t *testing.T) {
	store := filepath.Join(t.TempDir(), "sessions.json")
	ledger := &recoveryNativeLedger{answer: "continued from replacement thread"}
	p := newRecoveryCardPlatform()
	e, agent := replacingRecoveryEngine(t, store, ledger, p)
	e.ReceiveMessage(p, recoveryUserMessage("m1", "original task before signature repair"))
	recoveryWait(t, "original turn", func() bool { n, _, _ := ledger.counts(); return n == 1 && p.startCount() == 1 })
	select {
	case err := <-agent.replaceThread("native-replacement"):
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("replacement thread did not receive its durability acknowledgement")
	}
	if got := NewSessionManager(store).GetOrCreateActive("recovery-chat:alice").GetAgentSessionID(); got != "native-replacement" {
		t.Fatalf("replacement was acknowledged before its session binding persisted: %q", got)
	}
	_, saved := readOnlyChatCheckpoint(t, store)
	if string(saved["native"]) != `"native-replacement"` || string(saved["agent_session_id"]) != `"native-replacement"` || string(saved["replaces_agent_session_id"]) != `"native-1"` {
		t.Fatalf("replacement acknowledgement left a stale durable checkpoint: %s", saved)
	}
	stopRecoveryEngine(t, e)
	e2 := recoveryEngine(t, "project", store, ledger, p)
	recoveryWait(t, "replacement thread resumes on the original card", func() bool { return p.finalContains("handle-1", "continued from replacement thread") })
	ledger.mu.Lock()
	ids := append([]string(nil), ledger.starts...)
	ledger.mu.Unlock()
	if len(ids) != 2 || ids[1] != "native-replacement" {
		t.Fatalf("restart resumed the rejected original thread: %v", ids)
	}
	if sends, recovered, _ := ledger.counts(); sends != 1 || recovered != 1 {
		t.Fatalf("replacement replayed the original request: sends=%d recoveries=%d", sends, recovered)
	}
	stopRecoveryEngine(t, e2)
}

func TestChatRecovery_ReplacementJournalRepairsOnlyExactPredecessorBinding(t *testing.T) {
	for _, binding := range []string{"native-1", "unrelated-thread"} {
		t.Run(binding, func(t *testing.T) {
			store := filepath.Join(t.TempDir(), "sessions.json")
			ledger := &recoveryNativeLedger{answer: "replacement recovered"}
			p := newRecoveryCardPlatform()
			e := recoveryEngine(t, "project", store, ledger, p)
			e.ReceiveMessage(p, recoveryUserMessage("m1", "work through a native replacement"))
			recoveryWait(t, "original turn", func() bool { n, _, _ := ledger.counts(); return n == 1 && p.startCount() == 1 })
			stopRecoveryEngine(t, e)

			// Fault injection: only the first of the replacement's two durable
			// writes reached disk before the process exited.
			path, saved := readOnlyChatCheckpoint(t, store)
			saved["native"] = json.RawMessage(`"native-replacement"`)
			saved["agent_session_id"] = json.RawMessage(`"native-replacement"`)
			saved["replaces_agent_session_id"] = json.RawMessage(`"native-1"`)
			b, err := json.Marshal(saved)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, b, 0o600); err != nil {
				t.Fatal(err)
			}
			if binding != "native-1" {
				storeManager := NewSessionManager(store)
				storeManager.GetOrCreateActive("recovery-chat:alice").SetAgentSessionID(binding, "recovery-native")
				if err := storeManager.SaveWithError(); err != nil {
					t.Fatal(err)
				}
			}

			e2 := recoveryEngine(t, "project", store, ledger, p)
			if binding == "native-1" {
				recoveryWait(t, "exact predecessor is repaired and the replacement answers", func() bool { return p.finalContains("handle-1", "replacement recovered") })
				if got := NewSessionManager(store).GetOrCreateActive("recovery-chat:alice").GetAgentSessionID(); got != "native-replacement" {
					t.Fatalf("repair was not persisted: %q", got)
				}
			} else {
				time.Sleep(50 * time.Millisecond)
				if _, recovered, starts := ledger.counts(); recovered != 0 || starts != 1 {
					t.Fatalf("foreign binding was adopted: recoveries=%d native starts=%d", recovered, starts)
				}
				if got := NewSessionManager(store).GetOrCreateActive("recovery-chat:alice").GetAgentSessionID(); got != binding {
					t.Fatalf("foreign binding was overwritten: %q", got)
				}
			}
			stopRecoveryEngine(t, e2)
		})
	}
}

func TestChatRecovery_ReplacementSessionSaveFailureIsNotAcknowledged(t *testing.T) {
	store := filepath.Join(t.TempDir(), "sessions.json")
	ledger := &recoveryNativeLedger{}
	p := newRecoveryCardPlatform()
	e, agent := replacingRecoveryEngine(t, store, ledger, p)
	e.ReceiveMessage(p, recoveryUserMessage("m1", "task before persistence failure"))
	recoveryWait(t, "original turn", func() bool { n, _, _ := ledger.counts(); return n == 1 && p.startCount() == 1 })
	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	e.sessions.mu.Lock()
	e.sessions.storePath = filepath.Join(blocked, "sessions.json")
	e.sessions.mu.Unlock()
	select {
	case err := <-agent.replaceThread("native-replacement"):
		if err == nil {
			t.Fatal("replacement native execution received success before the new session binding was durable")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("persistence failure was not acknowledged")
	}
	e.sessions.mu.Lock()
	e.sessions.storePath = store
	e.sessions.mu.Unlock()
	if got := NewSessionManager(store).GetOrCreateActive("recovery-chat:alice").GetAgentSessionID(); got != "native-1" {
		t.Fatalf("fault did not preserve the old durable binding: %q", got)
	}
	_, saved := readOnlyChatCheckpoint(t, store)
	if string(saved["native"]) != `"native-replacement"` {
		t.Fatalf("replacement checkpoint must precede the failed session save: %s", saved["native"])
	}
	stopRecoveryEngine(t, e)
}
