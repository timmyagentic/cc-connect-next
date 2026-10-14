package core

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"
)

// Only the outbound platform boundary is mocked: its completion method models
// the extra, fresh message (never an edit of an already-read progress card).
type unreadReminderRecorder struct {
	mu              sync.Mutex
	enabled         bool
	err             error
	notices         []TurnCompletionNotice
	observed        []Message
	repliesAtNotice [][]string
	replyContexts   []any
	base            *stubPlatformEngine
}

func (r *unreadReminderRecorder) UnreadRemindersEnabled() bool { return r.enabled }
func (r *unreadReminderRecorder) ObserveReminderMessage(_ context.Context, msg *Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.observed = append(r.observed, *msg)
	return nil
}
func (r *unreadReminderRecorder) NotifyTurnCompleted(ctx context.Context, replyCtx any, notice TurnCompletionNotice) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.notices = append(r.notices, notice)
	r.replyContexts = append(r.replyContexts, replyCtx)
	r.repliesAtNotice = append(r.repliesAtNotice, r.base.getSent())
	if r.err != nil {
		return r.err
	}
	return r.base.Reply(ctx, replyCtx, notice.Text)
}
func (r *unreadReminderRecorder) snapshot() ([]TurnCompletionNotice, []Message) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]TurnCompletionNotice(nil), r.notices...), append([]Message(nil), r.observed...)
}

type unreadReminderPlatform struct {
	*stubPlatformEngine
	*unreadReminderRecorder
	failDelivery bool
}

func newUnreadReminderPlatform() *unreadReminderPlatform {
	base := &stubPlatformEngine{n: "test"}
	return &unreadReminderPlatform{stubPlatformEngine: base, unreadReminderRecorder: &unreadReminderRecorder{enabled: true, base: base}}
}
func (p *unreadReminderPlatform) Reply(ctx context.Context, replyCtx any, text string) error {
	if p.failDelivery {
		return errors.New("platform unavailable")
	}
	return p.stubPlatformEngine.Reply(ctx, replyCtx, text)
}
func (p *unreadReminderPlatform) Send(ctx context.Context, replyCtx any, text string) error {
	return p.Reply(ctx, replyCtx, text)
}

type unreadReminderNamedAgent struct {
	*cujAgent
	label string
}

func (a *unreadReminderNamedAgent) Name() string { return a.label }

func TestUnreadReminder_OnlySuccessfulVisibleTerminalAnswers(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		event                    Event
		enabled, failed, stopped bool
		missingReplyOwner        bool
		noticeError              error
		wantNotices              int
	}{
		{name: "answer", event: Event{Type: EventResult, Content: "finished answer", Done: true}, enabled: true, wantNotices: 1},
		{name: "missing presentation owner never borrows credentials", event: Event{Type: EventResult, Content: "finished answer", Done: true}, enabled: true, missingReplyOwner: true},
		{name: "disabled", event: Event{Type: EventResult, Content: "finished answer", Done: true}},
		{name: "silent", event: Event{Type: EventResult, Content: "NO_REPLY", Done: true}, enabled: true},
		{name: "agent error", event: Event{Type: EventResult, Content: "failed answer", Error: errors.New("agent failure"), Done: true}, enabled: true},
		{name: "delivery failure", event: Event{Type: EventResult, Content: "finished answer", Done: true}, enabled: true, failed: true},
		{name: "cancelled", event: Event{Type: EventResult, Content: "finished answer", Done: true}, enabled: true, stopped: true},
		{name: "notice failure preserves answer", event: Event{Type: EventResult, Content: "finished answer", Done: true}, enabled: true, noticeError: errors.New("notice persistence unavailable"), wantNotices: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newUnreadReminderPlatform()
			p.enabled, p.failDelivery, p.err = tc.enabled, tc.failed, tc.noticeError
			e := NewEngine("project", &stubAgent{}, []Platform{p}, filepath.Join(t.TempDir(), "sessions.json"), LangEnglish)
			t.Cleanup(func() { _ = e.Stop() })
			key := "test:group:topic"
			e.sessions.UpdateUserMeta(key, "recipient", "original conversation")
			session := e.sessions.GetOrCreateActive(key)
			agentSession := newControllableSession("agent-turn")
			state := &interactiveState{agentSession: agentSession, platform: p, replyCtx: "original-topic", currentUserID: "recipient", currentReplyUserID: "recipient", currentSessionKey: key}
			if tc.missingReplyOwner {
				state.currentReplyUserID = ""
			}
			e.interactiveStates[key] = state
			if tc.stopped {
				state.markStopped()
			}
			agentSession.events <- tc.event
			e.processInteractiveEvents(state, session, e.sessions, key, "trigger-message", time.Now(), nil, nil, state.replyCtx)
			notices, _ := p.snapshot()
			if len(notices) != tc.wantNotices {
				t.Fatalf("notices = %v, want %d", notices, tc.wantNotices)
			}
			if tc.wantNotices == 0 {
				return
			}
			n := notices[0]
			if n.UserID != "recipient" || n.SessionKey != key || n.TriggerMessageID != "trigger-message" {
				t.Fatalf("wrong completion destination: %+v", n)
			}
			if !strings.Contains(n.Summary, "finished answer") || !strings.Contains(n.Summary, "project") || !strings.Contains(n.Summary, "original conversation") || n.ReminderText == "" {
				t.Fatalf("incomplete completion text: %+v", n)
			}
			if got := p.repliesAtNotice[0]; len(got) != 1 || !strings.Contains(got[0], "finished answer") {
				t.Fatalf("notice ran before answer delivery: %v", got)
			}
			history := session.GetHistory(0)
			if len(history) != 1 || history[0].Content != "finished answer" {
				t.Fatalf("notice affected committed answer history: %v", history)
			}
			if tc.noticeError != nil && len(p.getSent()) != 1 {
				t.Fatalf("notice error replaced or duplicated final answer: %v", p.getSent())
			}
		})
	}
}

func TestUnreadReminder_NonTerminalResultDoesNotCreateNotice(t *testing.T) {
	p := newUnreadReminderPlatform()
	e := NewEngine("project", &stubAgent{}, []Platform{p}, "", LangEnglish)
	defer func() { _ = e.Stop() }()
	key := "test:recipient"
	session := e.sessions.GetOrCreateActive(key)
	agentSession := newControllableSession("compacting")
	state := &interactiveState{agentSession: agentSession, platform: p, replyCtx: "topic", currentUserID: "recipient", currentReplyUserID: "recipient", currentSessionKey: key}
	e.interactiveStates[key] = state
	agentSession.events <- Event{Type: EventResult, Content: "compacting context", Done: false}
	agentSession.events <- Event{Type: EventResult, Content: "actual answer", Done: true}
	e.processInteractiveEvents(state, session, e.sessions, key, "trigger", time.Now(), nil, nil, state.replyCtx)
	notices, _ := p.snapshot()
	if len(notices) != 1 || !strings.Contains(notices[0].Summary, "actual answer") {
		t.Fatalf("non-terminal event produced completion: %v", notices)
	}
	if strings.Contains(strings.Join(p.getSent(), "\n"), "compacting context") {
		t.Fatalf("intermediate result leaked: %v", p.getSent())
	}
}

type unreadReminderRichPlatform struct {
	*stubRichCardSilentPlatform
	*unreadReminderRecorder
}

func (p *unreadReminderRichPlatform) RichCardAnsweringDwell() time.Duration { return 0 }

func TestUnreadReminder_RichCardCompletionGetsFreshNotification(t *testing.T) {
	rich := &stubRichCardSilentPlatform{stubPlatformEngine: stubPlatformEngine{n: "test"}}
	r := &unreadReminderRecorder{enabled: true, base: &rich.stubPlatformEngine}
	p := &unreadReminderRichPlatform{stubRichCardSilentPlatform: rich, unreadReminderRecorder: r}
	e := NewEngine("project", &stubAgent{}, []Platform{p}, "", LangEnglish)
	defer func() { _ = e.Stop() }()
	e.SetDisplayConfig(DisplayCfg{Mode: "compact", CardMode: "rich"})
	e.SetStreamPreviewCfg(StreamPreviewCfg{Enabled: true, IntervalMs: 0, MinDeltaChars: 1})
	key := "test:recipient"
	session := e.sessions.GetOrCreateActive(key)
	agentSession := newControllableSession("rich-answer")
	state := &interactiveState{agentSession: agentSession, platform: p, replyCtx: "topic", currentUserID: "recipient", currentReplyUserID: "recipient", currentSessionKey: key}
	e.interactiveStates[key] = state
	agentSession.events <- Event{Type: EventText, Content: "answer in an existing progress card"}
	agentSession.events <- Event{Type: EventResult, Content: "answer in an existing progress card", Done: true}
	e.processInteractiveEvents(state, session, e.sessions, key, "trigger", time.Now(), nil, nil, state.replyCtx)
	starts, _, updates, _ := rich.snapshot()
	if len(starts) != 1 || len(updates) == 0 || !strings.Contains(updates[len(updates)-1], "status=done") {
		t.Fatalf("rich final answer was not delivered: starts=%v updates=%v", starts, updates)
	}
	notices, _ := r.snapshot()
	if len(notices) != 1 {
		t.Fatalf("existing rich card suppressed fresh completion notification: %v", notices)
	}
	if sent := p.getSent(); len(sent) != 1 || sent[0] != notices[0].Text {
		t.Fatalf("expected a distinct fresh completion message: %v", sent)
	}
}

func TestUnreadReminderSummary_BoundedUnicodeAndWhitespace(t *testing.T) {
	got := unreadReminderSummary("项目\n一", strings.Repeat("中文\t", 200))
	if !utf8.ValidString(got) || len([]rune(got)) != 160 || !strings.HasSuffix(got, "…") || strings.ContainsAny(got, "\n\t") {
		t.Fatalf("invalid bounded summary: %q", got)
	}
}

func TestUnreadReminder_QueuedTurnTargetsItsOwnUserAndTrigger(t *testing.T) {
	p := newUnreadReminderPlatform()
	agentSession := newQueuingSession("queued-reminders")
	e := NewEngine("project", &controllableAgent{nextSession: agentSession}, []Platform{p}, filepath.Join(t.TempDir(), "sessions.json"), LangEnglish)
	defer func() { _ = e.Stop() }()
	env := &cujEnv{t: t, engine: e, plat: p.stubPlatformEngine}
	send := func(user, id string) {
		e.ReceiveMessage(p, &Message{Platform: "test", SessionKey: "test:shared-topic", UserID: user, UserName: user, MessageID: id, Content: "do work " + id, ReplyCtx: "reply-" + id})
	}
	sentPrompts := func() int {
		agentSession.sendMu.Lock()
		defer agentSession.sendMu.Unlock()
		return len(agentSession.sendCalls)
	}
	send("first-user", "first-trigger")
	env.waitFor("first task started", 2*time.Second, func() bool { return sentPrompts() == 1 })
	send("second-user", "second-trigger")
	agentSession.events <- Event{Type: EventResult, Content: "first task done", Done: true}
	env.waitFor("queued task started", 2*time.Second, func() bool { return sentPrompts() == 2 })
	agentSession.events <- Event{Type: EventResult, Content: "second task done", Done: true}
	env.waitFor("both completion notifications", 2*time.Second, func() bool { notices, _ := p.snapshot(); return len(notices) == 2 })
	notices, _ := p.snapshot()
	for i, want := range []struct{ user, trigger string }{{"first-user", "first-trigger"}, {"second-user", "second-trigger"}} {
		if n := notices[i]; n.UserID != want.user || n.TriggerMessageID != want.trigger || n.SessionKey != "test:shared-topic" {
			t.Fatalf("turn %d borrowed another turn's destination: %+v", i, n)
		}
		if p.replyContexts[i] != "reply-"+want.trigger {
			t.Fatalf("turn %d used wrong reply context: %v", i, p.replyContexts[i])
		}
	}
}

func TestUnreadReminder_ManifestExposesOptInAndSideEffects(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		p := newUnreadReminderPlatform()
		p.enabled = enabled
		e := NewEngine("project", &stubAgent{}, []Platform{p}, "", LangEnglish)
		e.OnPlatformReady(p)
		manifest := e.QueryAgentCapabilityManifest("", "", false)
		adapter := findRuntimeAdapter(t, manifest.Runtime, "platform", "test")
		reminder := findRuntimeFeature(t, adapter.Capabilities, "unread_completion_reminder")
		expected := CapabilityUnavailable
		if enabled {
			expected = CapabilityConditional
		}
		if reminder.Availability.State != expected || reminder.Availability.Reason == "" {
			t.Fatalf("enabled=%v has misleading availability: %+v", enabled, reminder.Availability)
		}
		if reminder.ReadOnly == nil || *reminder.ReadOnly || reminder.Permission != CapabilityPermissionConditional || len(reminder.SideEffects) == 0 {
			t.Fatalf("reminder writes missing from manifest: %+v", reminder)
		}
		if reminder.Fallback.Mode != "skip" {
			t.Fatalf("reminder failure must preserve the answer: %+v", reminder.Fallback)
		}
		_ = e.Stop()
	}
}

// A legacy /ps changes the authority for agent actions but does not move the
// visible answer. A native steer explicitly adopts the new sender's presentation.
func TestUnreadReminder_SupplementRecipientFollowsAnswerPresentation(t *testing.T) {
	for _, nativeSteer := range []bool{false, true} {
		name := "legacy_ps_keeps_original_recipient"
		if nativeSteer {
			name = "native_steer_adopts_latest_recipient"
		}
		t.Run(name, func(t *testing.T) {
			p := newUnreadReminderPlatform()
			queued := newQueuingSession("supplement-reminders")
			var agentSession AgentSession = queued
			if nativeSteer {
				steered := newSteerableSession("native-reminders")
				queued = &steered.queuingAgentSession
				agentSession = steered
			}
			e := NewEngine("project", &controllableAgent{nextSession: agentSession}, []Platform{p}, filepath.Join(t.TempDir(), "sessions.json"), LangEnglish)
			defer func() { _ = e.Stop() }()
			env := &cujEnv{t: t, engine: e, plat: p.stubPlatformEngine}
			const key = "test:shared-topic"
			send := func(user, id, content string) {
				e.ReceiveMessage(p, &Message{Platform: "test", SessionKey: key, UserID: user, UserName: user, MessageID: id, Content: content, ReplyCtx: "reply-" + id})
			}
			send("original-user", "original-trigger", "complete the task")
			var state *interactiveState
			env.waitFor("original answer presentation is open", 2*time.Second, func() bool {
				e.interactiveMu.Lock()
				state = e.interactiveStates[key]
				e.interactiveMu.Unlock()
				if state == nil {
					return false
				}
				state.mu.Lock()
				defer state.mu.Unlock()
				return state.presentationOpen
			})
			send("supplement-user", "supplement-trigger", "/ps include one more detail")
			state.mu.Lock()
			credentialUser := state.currentUserID
			state.mu.Unlock()
			if credentialUser != "supplement-user" {
				t.Fatalf("test did not exercise supplement authorization handoff: %q", credentialUser)
			}
			queued.events <- Event{Type: EventResult, Content: "task finished with the extra detail", Done: true}
			env.waitFor("completion notification", 2*time.Second, func() bool { notices, _ := p.snapshot(); return len(notices) == 1 })
			notices, _ := p.snapshot()
			wantUser, wantTrigger := "original-user", "original-trigger"
			if nativeSteer {
				wantUser, wantTrigger = "supplement-user", "supplement-trigger"
			}
			if n := notices[0]; n.UserID != wantUser || n.TriggerMessageID != wantTrigger || n.SessionKey != key {
				t.Fatalf("completion followed credential identity rather than visible answer: %+v; want %s/%s", n, wantUser, wantTrigger)
			}
			if p.replyContexts[0] != "reply-"+wantTrigger {
				t.Fatalf("completion and answer presentation diverged: %v", p.replyContexts[0])
			}
		})
	}
}
