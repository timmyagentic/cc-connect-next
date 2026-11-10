package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Only the outbound boundary is mocked. Mentions remain visible in the
// original conversation; this platform has no private-send capability.
type ownerEscalationPlatform struct {
	*stubPlatformEngine
	mu           sync.Mutex
	notices      []OwnerEscalationNotice
	contexts     []any
	before       [][]string
	mentionErr   error
	failAnswer   bool
	sendDeadline time.Time
}

func newOwnerEscalationPlatform() *ownerEscalationPlatform {
	return &ownerEscalationPlatform{stubPlatformEngine: &stubPlatformEngine{n: "test"}}
}
func (p *ownerEscalationPlatform) Reply(ctx context.Context, rc any, content string) error {
	if p.failAnswer {
		return errors.New("answer delivery failed")
	}
	return p.stubPlatformEngine.Reply(ctx, rc, content)
}
func (p *ownerEscalationPlatform) Send(ctx context.Context, rc any, content string) error {
	return p.Reply(ctx, rc, content)
}
func (p *ownerEscalationPlatform) SendOwnerEscalation(ctx context.Context, rc any, n OwnerEscalationNotice) error {
	p.mu.Lock()
	p.sendDeadline, _ = ctx.Deadline()
	p.notices = append(p.notices, n)
	p.contexts = append(p.contexts, rc)
	p.before = append(p.before, p.getSent())
	err := p.mentionErr
	p.mu.Unlock()
	if err != nil {
		return err
	}
	return p.stubPlatformEngine.Reply(ctx, rc, "@"+n.OwnerName+" "+n.Text)
}
func (p *ownerEscalationPlatform) snapshot() []OwnerEscalationNotice {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]OwnerEscalationNotice(nil), p.notices...)
}

func ownerEscalationTestConfig() OwnerEscalationConfig {
	return OwnerEscalationConfig{Enabled: true, OwnerID: "owner", OwnerName: "Tingjun", APIKey: "test-key", TimeoutSeconds: 2}
}
func needsOwnerTestDecision() escalationDecision {
	return escalationDecision{Status: "needs_owner", Reason: "Two checks confirmed the decision requires the owner.", Summary: "The release policy is undecided.", OwnerAction: "Choose whether the release should proceed."}
}

func TestOwnerEscalation_OnlyEligibleDeliveredAnswers(t *testing.T) {
	for _, tc := range []struct {
		name                                                      string
		event                                                     Event
		owner, missingOwner, failed, stopped, scheduled, disabled bool
		decision                                                  escalationDecision
		judgeErr, mentionErr                                      error
		wantJudge, wantMentions                                   int
	}{
		{name: "needs owner", event: Event{Type: EventResult, Content: "final answer", Done: true}, decision: needsOwnerTestDecision(), wantJudge: 1, wantMentions: 1},
		{name: "waiting", event: Event{Type: EventResult, Content: "final answer", Done: true}, decision: escalationDecision{Status: "waiting", Reason: "Asked for reproduction details."}, wantJudge: 1},
		{name: "resolved", event: Event{Type: EventResult, Content: "final answer", Done: true}, decision: escalationDecision{Status: "resolved", Reason: "Verification passed."}, wantJudge: 1},
		{name: "owner's own request", event: Event{Type: EventResult, Content: "final answer", Done: true}, owner: true},
		{name: "no trusted reply owner", event: Event{Type: EventResult, Content: "final answer", Done: true}, missingOwner: true},
		{name: "disabled", event: Event{Type: EventResult, Content: "final answer", Done: true}, disabled: true},
		{name: "silent", event: Event{Type: EventResult, Content: "NO_REPLY", Done: true}},
		{name: "failed delivery", event: Event{Type: EventResult, Content: "final answer", Done: true}, failed: true},
		{name: "stopped", event: Event{Type: EventResult, Content: "final answer", Done: true}, stopped: true},
		{name: "scheduled receipt", event: Event{Type: EventResult, Content: "final answer", Done: true}, scheduled: true},
		{name: "agent error", event: Event{Type: EventResult, Content: "final answer", Done: true, Error: errors.New("agent unavailable")}},
		{name: "judge error preserves answer", event: Event{Type: EventResult, Content: "final answer", Done: true}, judgeErr: errors.New("judge unavailable"), wantJudge: 1},
		{name: "invalid judgment preserves answer", event: Event{Type: EventResult, Content: "final answer", Done: true}, decision: escalationDecision{Status: "needs_owner"}, wantJudge: 1},
		{name: "mention failure is truthful", event: Event{Type: EventResult, Content: "final answer", Done: true}, decision: needsOwnerTestDecision(), mentionErr: errors.New("owner is not in group"), wantJudge: 1, wantMentions: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newOwnerEscalationPlatform()
			p.failAnswer, p.mentionErr = tc.failed, tc.mentionErr
			e := NewEngine("project", &stubAgent{}, []Platform{p}, filepath.Join(t.TempDir(), "sessions.json"), LangEnglish)
			t.Cleanup(func() { _ = e.Stop() })
			cfg := ownerEscalationTestConfig()
			cfg.Enabled = !tc.disabled
			if err := e.SetOwnerEscalation(cfg); err != nil {
				t.Fatal(err)
			}
			calls := 0
			if e.ownerEscalation != nil {
				e.ownerEscalation.judge = func(_ context.Context, in escalationInput) (escalationDecision, error) {
					calls++
					if len(in.History) == 0 || in.History[len(in.History)-1].Content != "final answer" {
						t.Errorf("judge missed committed answer: %+v", in.History)
					}
					return tc.decision, tc.judgeErr
				}
			}
			key := "test:group:topic"
			session := e.sessions.GetOrCreateActive(key)
			as := newControllableSession("turn")
			state := &interactiveState{agentSession: as, platform: p, replyCtx: "original-topic", currentUserID: "requester", currentReplyUserID: "requester", currentSessionKey: key}
			if tc.owner {
				state.currentReplyUserID = "owner"
			}
			if tc.missingOwner {
				state.currentReplyUserID = ""
			}
			e.interactiveStates[key] = state
			if tc.stopped {
				state.markStopped()
			}
			var scheduled *scheduledTurnResult
			if tc.scheduled {
				scheduled = &scheduledTurnResult{}
			}
			as.events <- tc.event
			e.processInteractiveEvents(state, session, e.sessions, key, "source-message", time.Now(), nil, nil, state.replyCtx, scheduled)
			if calls != tc.wantJudge || len(p.snapshot()) != tc.wantMentions {
				t.Fatalf("judge calls %d, mentions %v; want %d/%d", calls, p.snapshot(), tc.wantJudge, tc.wantMentions)
			}
			if tc.wantMentions > 0 {
				n := p.snapshot()[0]
				if n.OwnerID != "owner" || n.TriggerMessageID != "source-message" || n.SessionKey != key || p.contexts[0] != "original-topic" || len(n.IdempotencyKey) != 32 {
					t.Fatalf("wrong escalation target: %+v, context %v", n, p.contexts)
				}
				if len(p.before[0]) != 1 || !strings.Contains(p.before[0][0], "final answer") {
					t.Fatalf("mention preceded normal answer: %v", p.before)
				}
				if len(p.getSent()) != 2 {
					t.Fatalf("expected normal answer and one follow-up only: %v", p.getSent())
				}
				if tc.mentionErr != nil && !strings.Contains(p.getSent()[1], "could not confirm") {
					t.Fatalf("failure claimed success: %v", p.getSent())
				}
				if tc.mentionErr == nil && !strings.HasPrefix(p.getSent()[1], "@Tingjun ") {
					t.Fatalf("missing visible owner mention: %v", p.getSent())
				}
			}
			if tc.wantJudge == 1 && tc.wantMentions == 0 && len(p.getSent()) != 1 {
				t.Fatalf("judge result disrupted original answer: %v", p.getSent())
			}
		})
	}
}

func TestOwnerEscalation_ToolEvidenceIndependentOfProgressDisplay(t *testing.T) {
	p := newOwnerEscalationPlatform()
	e := NewEngine("project", &stubAgent{}, []Platform{p}, filepath.Join(t.TempDir(), "sessions.json"), LangEnglish)
	defer func() { _ = e.Stop() }()
	if err := e.SetOwnerEscalation(ownerEscalationTestConfig()); err != nil {
		t.Fatal(err)
	}
	e.SetDisplayConfig(DisplayCfg{ToolMessages: false})
	var captured escalationInput
	e.ownerEscalation.judge = func(_ context.Context, in escalationInput) (escalationDecision, error) {
		captured = in
		return escalationDecision{Status: "waiting", Reason: "A recoverable failure."}, nil
	}
	key := "test:group:topic"
	s := e.sessions.GetOrCreateActive(key)
	s.AddHistory("user", "Please verify the issue.")
	as := newControllableSession("turn")
	state := &interactiveState{agentSession: as, platform: p, replyCtx: "topic", currentReplyUserID: "requester", currentSessionKey: key}
	e.interactiveStates[key] = state
	code := 1
	success := false
	as.events <- Event{Type: EventToolResult, ToolName: "check", ToolResult: "verification failed", ToolStatus: "error", ToolExitCode: &code, ToolSuccess: &success}
	as.events <- Event{Type: EventResult, Content: "Please share the missing detail.", Done: true}
	e.processInteractiveEvents(state, s, e.sessions, key, "source", time.Now(), nil, nil, state.replyCtx)
	if len(captured.Tools) != 1 || captured.Tools[0].Result != "verification failed" || captured.Tools[0].ExitCode == nil || *captured.Tools[0].ExitCode != 1 {
		t.Fatalf("missing actual execution evidence: %+v", captured.Tools)
	}
	if len(captured.History) != 2 || captured.History[0].Content != "Please verify the issue." || captured.History[1].Content != "Please share the missing detail." {
		t.Fatalf("missing conversation context: %+v", captured.History)
	}
	if strings.Contains(strings.Join(p.getSent(), "\n"), "verification failed") {
		t.Fatalf("judge evidence enabled hidden progress: %v", p.getSent())
	}
}

func TestOwnerEscalation_DeduplicationSurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	cfg := ownerEscalationTestConfig()
	for run := 0; run < 2; run++ {
		p := newOwnerEscalationPlatform()
		e := NewEngine("project", &stubAgent{}, []Platform{p}, path, LangEnglish)
		if err := e.SetOwnerEscalation(cfg); err != nil {
			t.Fatal(err)
		}
		e.ownerEscalation.judge = func(context.Context, escalationInput) (escalationDecision, error) {
			return needsOwnerTestDecision(), nil
		}
		key := "test:topic"
		s := e.sessions.GetOrCreateActive(key)
		state := &interactiveState{currentReplyUserID: "requester", currentSessionKey: key}
		c := turnCompletion{processor: &turnProcessor{engine: e, state: state, session: s, sessions: e.sessions}, platform: p, replyCtx: "topic", terminalDeliveryCtx: context.Background(), msgID: "same-source"}
		c.escalateOwner(preparedTurnResponse{base: "normal answer"})
		c.escalateOwner(preparedTurnResponse{base: "normal answer"})
		want := 0
		if run == 0 {
			want = 1
		}
		if len(p.snapshot()) != want {
			t.Fatalf("run %d repeated mention: %+v", run, p.snapshot())
		}
		_ = e.Stop()
	}
}

func TestOwnerEscalation_ReceiptPersistenceFailureDoesNotSend(t *testing.T) {
	p := newOwnerEscalationPlatform()
	e := NewEngine("project", &stubAgent{}, []Platform{p}, filepath.Join(t.TempDir(), "sessions.json"), LangEnglish)
	defer func() { _ = e.Stop() }()
	if err := e.SetOwnerEscalation(ownerEscalationTestConfig()); err != nil {
		t.Fatal(err)
	}
	q := e.ownerEscalation
	if err := os.Mkdir(q.path, 0700); err != nil {
		t.Fatal(err)
	}
	q.judge = func(context.Context, escalationInput) (escalationDecision, error) {
		return needsOwnerTestDecision(), nil
	}
	s := e.sessions.GetOrCreateActive("test:topic")
	c := turnCompletion{processor: &turnProcessor{engine: e, state: &interactiveState{currentReplyUserID: "requester", currentSessionKey: "test:topic"}, session: s, sessions: e.sessions}, platform: p, replyCtx: "topic", terminalDeliveryCtx: context.Background(), msgID: "source"}
	c.escalateOwner(preparedTurnResponse{base: "answer"})
	if len(p.snapshot()) != 0 {
		t.Fatal("sent without persistent deduplication")
	}
}

func TestOwnerEscalation_SendHasFreshBudgetAfterJudgment(t *testing.T) {
	p := newOwnerEscalationPlatform()
	e := NewEngine("project", &stubAgent{}, []Platform{p}, filepath.Join(t.TempDir(), "sessions.json"), LangEnglish)
	defer func() { _ = e.Stop() }()
	if err := e.SetOwnerEscalation(ownerEscalationTestConfig()); err != nil {
		t.Fatal(err)
	}
	var judgeDeadline time.Time
	e.ownerEscalation.judge = func(ctx context.Context, _ escalationInput) (escalationDecision, error) {
		judgeDeadline, _ = ctx.Deadline()
		return needsOwnerTestDecision(), nil
	}
	s := e.sessions.GetOrCreateActive("test:topic")
	c := turnCompletion{processor: &turnProcessor{engine: e, state: &interactiveState{currentReplyUserID: "requester", currentSessionKey: "test:topic"}, session: s, sessions: e.sessions}, platform: p, replyCtx: "topic", terminalDeliveryCtx: context.Background(), msgID: "source"}
	c.escalateOwner(preparedTurnResponse{base: "answer"})
	if len(p.snapshot()) != 1 || p.sendDeadline.Sub(judgeDeadline) < 10*time.Second {
		t.Fatal("mention delivery reused the nearly exhausted judge budget")
	}
}

func TestOwnerEscalation_RecallDuringJudgmentCancelsMention(t *testing.T) {
	p := newOwnerEscalationPlatform()
	e := NewEngine("project", &stubAgent{}, []Platform{p}, filepath.Join(t.TempDir(), "sessions.json"), LangEnglish)
	defer func() { _ = e.Stop() }()
	if err := e.SetOwnerEscalation(ownerEscalationTestConfig()); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	e.ownerEscalation.judge = func(ctx context.Context, _ escalationInput) (escalationDecision, error) {
		close(started)
		<-ctx.Done()
		return escalationDecision{}, ctx.Err()
	}
	s := e.sessions.GetOrCreateActive("test:topic")
	recalled := make(chan struct{})
	c := turnCompletion{processor: &turnProcessor{engine: e, state: &interactiveState{currentReplyUserID: "requester", currentSessionKey: "test:topic"}, session: s, sessions: e.sessions}, platform: p, replyCtx: "topic", terminalDeliveryCtx: context.Background(), silentCancel: recalled, msgID: "source"}
	done := make(chan struct{})
	go func() { c.escalateOwner(preparedTurnResponse{base: "answer"}); close(done) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("judge did not start")
	}
	close(recalled)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("recall did not cancel judgment")
	}
	if len(p.getSent()) != 0 || len(p.snapshot()) != 0 {
		t.Fatalf("recalled source still produced a mention or failure reply: %v", p.getSent())
	}
}
