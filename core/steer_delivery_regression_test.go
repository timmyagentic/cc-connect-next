package core

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCUJ_SteerPreservesEarlierAcceptedQueuedTask(t *testing.T) {
	backend := newAnswerProfileTestSession()
	backend.releaseFirst = make(chan struct{})
	var release sync.Once
	platform := &stubPlatformEngine{n: "test"}
	engine := NewEngine("test", &answerProfileTestAgent{session: backend}, []Platform{platform}, "", LangEnglish)
	engine.SetAnswerProfiles(AnswerProfiles{Fast: &AnswerProfileOptions{Model: "fast-model"}})
	t.Cleanup(func() { _ = engine.Stop() })
	t.Cleanup(func() { release.Do(func() { close(backend.releaseFirst) }) })

	start := time.Now().UnixMilli()
	first := answerProfileMessage("first", "working task")
	first.UserMessageTimeMs = start
	engine.ReceiveMessage(platform, first)
	waitAnswerProfileTest(t, "working task starts", func() bool {
		calls, _ := backend.snapshot()
		return len(calls) == 1
	})

	queued := answerProfileMessage("queued", "/fast independent task")
	queued.UserMessageTimeMs = start + 1
	engine.ReceiveMessage(platform, queued)
	waitAnswerProfileTest(t, "independent task queues", func() bool { return cronModelQueueDepth(engine) == 1 })
	steered := answerProfileMessage("steered", "correct the working task")
	steered.UserMessageTimeMs = start + 2
	engine.ReceiveMessage(platform, steered)
	waitAnswerProfileTest(t, "correction steers", func() bool {
		_, count := backend.snapshot()
		return count == 1
	})

	release.Do(func() { close(backend.releaseFirst) })
	waitAnswerProfileTest(t, "both accepted tasks finish", func() bool {
		return !engine.sessions.GetOrCreateActive(first.SessionKey).Busy()
	})
	calls, steers := backend.snapshot()
	if len(calls) != 2 || calls[1].prompt != "independent task" || calls[1].options.Model != "fast-model" || steers != 1 {
		t.Fatalf("accepted queued task was lost or repeated after newer steer: calls=%+v steers=%d", calls, steers)
	}
}

// Hold presentation behind two successful steers, then pause while replaying
// the older handoff. Authorization must already belong to the newest sender.
type delayedSteerPresentationPlatform struct {
	steerRichCardPlatform
	initialStarted  chan struct{}
	releaseInitial  chan struct{}
	redirectStarted chan struct{}
	releaseRedirect chan struct{}
}

func (p *delayedSteerPresentationPlatform) SendPreviewStart(ctx context.Context, replyCtx any, content string) (any, error) {
	handle, err := p.steerRichCardPlatform.SendPreviewStart(ctx, replyCtx, content)
	if handle == "card-1" {
		close(p.initialStarted)
		<-p.releaseInitial
	}
	return handle, err
}

func (p *delayedSteerPresentationPlatform) UpdateMessage(ctx context.Context, handle any, content string) error {
	if handle == "card-1" && strings.Contains(content, "status=redirected") {
		close(p.redirectStarted)
		<-p.releaseRedirect
	}
	return p.steerRichCardPlatform.UpdateMessage(ctx, handle, content)
}

func TestCUJ_SteerPresentationCannotRestoreOlderSenderAuthorization(t *testing.T) {
	platform := &delayedSteerPresentationPlatform{
		steerRichCardPlatform: steerRichCardPlatform{stubPlatformEngine: stubPlatformEngine{n: "test"}},
		initialStarted:        make(chan struct{}), releaseInitial: make(chan struct{}),
		redirectStarted: make(chan struct{}), releaseRedirect: make(chan struct{}),
	}
	engine := NewEngine("test", &stubAgent{}, []Platform{platform}, "", LangEnglish)
	engine.SetDisplayConfig(DisplayCfg{Mode: "full", CardMode: "rich"})
	engine.SetAdminFrom("admin")
	backend := newSteerableSession("presentation-authorization")
	const key = "test:authorization"
	state := &interactiveState{agentSession: backend, platform: platform, replyCtx: "first"}
	engine.interactiveStates[key] = state
	session := engine.sessions.GetOrCreateActive(key)
	done := make(chan struct{})
	var releaseInitial, releaseRedirect sync.Once
	t.Cleanup(func() {
		releaseInitial.Do(func() { close(platform.releaseInitial) })
		releaseRedirect.Do(func() { close(platform.releaseRedirect) })
		backend.events <- Event{Type: EventResult, Content: "done", Done: true}
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("presentation event loop did not finish")
		}
		_ = engine.Stop()
	})
	go func() {
		defer close(done)
		engine.processInteractiveEvents(state, session, engine.sessions, key, "first", time.Now(), nil, nil, "first")
	}()
	select {
	case <-platform.initialStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("initial card did not start")
	}
	for _, sender := range []string{"admin", "member"} {
		msg := &Message{SessionKey: key, MessageID: sender, Content: "correction by " + sender, ReplyCtx: sender, Platform: "test", UserID: sender}
		if !engine.trySteerBusyMessage(platform, msg, key, session, engine.sessions) {
			t.Fatalf("steer by %s was not accepted", sender)
		}
	}
	releaseInitial.Do(func() { close(platform.releaseInitial) })
	select {
	case <-platform.redirectStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("older handoff did not reach its card update")
	}
	state.mu.Lock()
	currentUser := state.currentUserID
	state.mu.Unlock()
	if currentUser != "member" || engine.isAdmin(currentUser) {
		t.Fatalf("older card handoff restored authorization to %q; latest accepted sender is member", currentUser)
	}
}

func TestCUJ_SteerKeepsScheduledTurnAndUserReplySeparate(t *testing.T) {
	for _, tc := range []struct {
		name       string
		muted      bool
		boundModel bool
		taskModel  string
	}{
		{name: "default scheduled turn"},
		{name: "muted task with chat binding", muted: true, boundModel: true, taskModel: "task-model"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engine, platform, agent := newChatModelEngine(t, "")
			if tc.boundModel {
				engine.ReceiveMessage(platform, chatModelMessage("user", "", "bind", "/model group-B"))
			}
			release := make(chan struct{})
			var releaseOnce sync.Once
			t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
			agent.onSend = func(session *cronModelSession, call cronModelCall) error {
				if call.prompt == "scheduled-work" {
					go func() {
						<-release
						session.events <- Event{Type: EventResult, Content: "private scheduled result", Done: true}
					}()
					return nil
				}
				session.events <- Event{Type: EventResult, Content: "public user result", Done: true}
				return nil
			}
			job := cronModelJob("scheduled-work", tc.taskModel)
			job.Mute = tc.muted
			done := make(chan error, 1)
			go func() { done <- engine.ExecuteCronJob(job) }()
			waitAnswerProfileTest(t, "scheduled turn ready for input", func() bool {
				engine.interactiveMu.Lock()
				state := engine.interactiveStates["test:user"]
				engine.interactiveMu.Unlock()
				if state == nil {
					return false
				}
				state.mu.Lock()
				defer state.mu.Unlock()
				return state.presentationOpen && !state.contextSending
			})
			engine.ReceiveMessage(platform, chatModelMessage("user", "", "user-reply", "ordinary user request"))
			waitAnswerProfileTest(t, "user input routed", func() bool {
				_, sessions, _ := agent.snapshot()
				if len(sessions) == 0 {
					return false
				}
				sessions[0].mu.Lock()
				steers := sessions[0].steerCalls
				sessions[0].mu.Unlock()
				return steers > 0 || cronModelQueueDepth(engine) == 1
			})
			_, sessions, _ := agent.snapshot()
			sessions[0].mu.Lock()
			steers := sessions[0].steerCalls
			sessions[0].mu.Unlock()
			if steers != 0 {
				t.Fatalf("user request entered the scheduled turn: steers=%d", steers)
			}
			releaseOnce.Do(func() { close(release) })
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("scheduled turn and queued user request did not finish")
			}
			calls, _, _ := agent.snapshot()
			if len(calls) != 2 || calls[1].prompt != "ordinary user request" {
				t.Fatalf("user request was not delivered independently: %+v", calls)
			}
			visible := strings.Join(platform.getSent(), "\n")
			if !strings.Contains(visible, "public user result") || tc.muted && strings.Contains(visible, "private scheduled result") {
				t.Fatalf("scheduled output boundary was lost: %q", visible)
			}
		})
	}
}
