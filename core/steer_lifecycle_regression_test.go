package core

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

type blockedSteerPreviewPlatform struct {
	steerRichCardPlatform
	entered chan struct{}
	release chan struct{}
}

func (p *blockedSteerPreviewPlatform) SendPreviewStart(ctx context.Context, target any, content string) (any, error) {
	handle, err := p.steerRichCardPlatform.SendPreviewStart(ctx, target, content)
	if target == "supplement" {
		close(p.entered)
		<-p.release
	}
	return handle, err
}

func TestCUJ_SteerStartupRecallCancelsPendingDelivery(t *testing.T) {
	for _, phase := range []string{"context", "preview"} {
		t.Run(phase, func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			var releasePreparation, releaseSend sync.Once
			backend := &startupSteerSession{
				answerProfileTestSession: newAnswerProfileTestSession(),
				sendEntered:              make(chan struct{}), releaseSend: make(chan struct{}),
			}
			var platform Platform
			if phase == "context" {
				platform = &contextTestPlatform{
					stubPlatformEngine: &stubPlatformEngine{n: "test"},
					backgrounds:        map[string]MessageContext{},
					onLoad: func(count int) {
						if count == 2 {
							close(entered)
							<-release
						}
					},
				}
			} else {
				platform = &blockedSteerPreviewPlatform{
					steerRichCardPlatform: steerRichCardPlatform{stubPlatformEngine: stubPlatformEngine{n: "test"}},
					entered:               entered, release: release,
				}
			}
			engine := NewEngine("test", &controllableAgent{nextSession: backend}, []Platform{platform}, "", LangEnglish)
			if phase == "preview" {
				engine.SetDisplayConfig(DisplayCfg{Mode: "full", CardMode: "rich"})
			}
			t.Cleanup(func() { _ = engine.Stop() })
			t.Cleanup(func() {
				releasePreparation.Do(func() { close(release) })
				releaseSend.Do(func() { close(backend.releaseSend) })
			})
			first := answerProfileMessage("first", "original task")
			first.ReplyCtx = first.SessionKey
			engine.ReceiveMessage(platform, first)
			select {
			case <-backend.sendEntered:
			case <-time.After(3 * time.Second):
				t.Fatal("original task did not reach its startup wait")
			}
			supplement := answerProfileMessage("supplement", "recalled correction")
			supplement.ReplyCtx = first.SessionKey
			if phase == "preview" {
				supplement.ReplyCtx = "supplement"
			}
			engine.ReceiveMessage(platform, supplement)
			waitAnswerProfileTest(t, "startup correction queued", func() bool { return cronModelQueueDepth(engine) == 1 })
			releaseSend.Do(func() { close(backend.releaseSend) })
			select {
			case <-entered:
			case <-time.After(3 * time.Second):
				t.Fatal("queued correction did not reach controlled preparation")
			}
			engine.ReceiveMessage(platform, &Message{SessionKey: first.SessionKey, MessageID: supplement.MessageID, Platform: "test", Recalled: true})
			releasePreparation.Do(func() { close(release) })
			engine.interactiveMu.Lock()
			state := engine.interactiveStates[first.SessionKey]
			engine.interactiveMu.Unlock()
			// The worker owns steerMu through preparation and dispatch. Acquiring
			// it after releasing preparation proves the attempt has settled.
			state.steerMu.Lock()
			state.steerMu.Unlock()
			sends, steers := backend.inputs()
			if len(sends) != 1 || len(steers) != 0 || cronModelQueueDepth(engine) != 0 {
				t.Fatalf("recalled startup correction was delivered: sends=%+v steers=%+v", sends, steers)
			}
			backend.events <- Event{Type: EventResult, Content: "original complete", Done: true}
			waitAnswerProfileTest(t, "original task completes", func() bool { return !engine.sessions.GetOrCreateActive(first.SessionKey).Busy() })
		})
	}
}

type lifecycleSteerSession struct {
	*startupSteerSession
	onSend  func(int) error
	onSteer func() error
}

func (s *lifecycleSteerSession) Send(prompt string, images []ImageAttachment, files []FileAttachment) error {
	s.mu.Lock()
	index := len(s.sends)
	s.sends = append(s.sends, startupSteerInput{prompt, images, files})
	s.mu.Unlock()
	return s.onSend(index)
}

func (s *lifecycleSteerSession) SendWithTurnOptions(prompt string, images []ImageAttachment, files []FileAttachment, _ TurnOptions) error {
	return s.Send(prompt, images, files)
}

func (s *lifecycleSteerSession) Steer(prompt string, images []ImageAttachment, files []FileAttachment) error {
	if err := s.startupSteerSession.Steer(prompt, images, files); err != nil {
		return err
	}
	if s.onSteer != nil {
		return s.onSteer()
	}
	return nil
}

func TestCUJ_SteerStartupDoesNotFollowAnotherQueuedTask(t *testing.T) {
	backend := &lifecycleSteerSession{startupSteerSession: &startupSteerSession{answerProfileTestSession: newAnswerProfileTestSession()}}
	firstStarted, finishFirst, releaseFirst, secondStarted := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
	var finish, release sync.Once
	backend.onSend = func(index int) error {
		switch index {
		case 0:
			close(firstStarted)
			<-finishFirst
			backend.events <- Event{Type: EventResult, Content: "first complete", Done: true}
			<-releaseFirst
		case 1:
			close(secondStarted)
		default:
			backend.events <- Event{Type: EventResult, Content: "correction handled separately", Done: true}
		}
		return nil
	}
	platform := &stubPlatformEngine{n: "test"}
	engine := NewEngine("test", &controllableAgent{nextSession: backend}, []Platform{platform}, "", LangEnglish)
	engine.SetAnswerProfiles(AnswerProfiles{Fast: &AnswerProfileOptions{Model: "fast-model"}})
	t.Cleanup(func() { _ = engine.Stop() })
	t.Cleanup(func() {
		finish.Do(func() { close(finishFirst) })
		release.Do(func() { close(releaseFirst) })
	})
	first := answerProfileMessage("first", "original task")
	engine.ReceiveMessage(platform, first)
	select {
	case <-firstStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("original task did not start")
	}
	engine.ReceiveMessage(platform, answerProfileMessage("independent", "/fast independent task"))
	engine.ReceiveMessage(platform, answerProfileMessage("supplement", "correct original task"))
	waitAnswerProfileTest(t, "both inputs queued before readiness", func() bool { return cronModelQueueDepth(engine) == 2 })
	finish.Do(func() { close(finishFirst) })
	engine.interactiveMu.Lock()
	state := engine.interactiveStates[first.SessionKey]
	engine.interactiveMu.Unlock()
	waitAnswerProfileTest(t, "independent task adopted before first Send returns", func() bool {
		state.mu.Lock()
		defer state.mu.Unlock()
		return state.currentMessageID == "independent"
	})
	state.mu.Lock()
	retargeted := len(state.pendingMessages) == 1 && state.pendingMessages[0].pendingSteer != nil
	state.mu.Unlock()
	if retargeted {
		t.Error("startup correction still targets steering after its original turn completed")
	}
	release.Do(func() { close(releaseFirst) })
	select {
	case <-secondStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("independent task did not start")
	}
	backend.events <- Event{Type: EventResult, Content: "independent complete", Done: true}
	waitAnswerProfileTest(t, "all accepted input completes", func() bool { return !engine.sessions.GetOrCreateActive(first.SessionKey).Busy() })
	sends, steers := backend.inputs()
	if len(sends) != 3 || sends[1].prompt != "independent task" || sends[2].prompt != "correct original task" || len(steers) != 0 {
		t.Fatalf("correction entered a different queued task: sends=%+v steers=%+v", sends, steers)
	}
}

type steerProgressPlatform struct {
	stubPlatformEngine
	progress chan struct{}
	once     sync.Once
}

func (p *steerProgressPlatform) Send(ctx context.Context, target any, content string) error {
	err := p.stubPlatformEngine.Send(ctx, target, content)
	if strings.Contains(content, "progress-during-steer") {
		p.once.Do(func() { close(p.progress) })
	}
	return err
}

func TestCUJ_SteerStartupKeepsConsumingAgentEvents(t *testing.T) {
	platform := &steerProgressPlatform{stubPlatformEngine: stubPlatformEngine{n: "test"}, progress: make(chan struct{})}
	backend := &lifecycleSteerSession{startupSteerSession: &startupSteerSession{answerProfileTestSession: newAnswerProfileTestSession()}}
	started, ready, abortSteer := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var releaseReady, releaseSteer sync.Once
	backend.onSend = func(index int) error {
		if index == 0 {
			close(started)
			<-ready
		}
		return nil
	}
	backend.onSteer = func() error {
		backend.events <- Event{Type: EventThinking, Content: "progress-during-steer"}
		select {
		case <-platform.progress:
			return nil
		case <-abortSteer:
			return ErrSteerRejected
		}
	}
	engine := NewEngine("test", &controllableAgent{nextSession: backend}, []Platform{platform}, "", LangEnglish)
	engine.SetDisplayConfig(DisplayCfg{Mode: "full", ThinkingMessages: true})
	t.Cleanup(func() { _ = engine.Stop() })
	t.Cleanup(func() {
		releaseReady.Do(func() { close(ready) })
		releaseSteer.Do(func() { close(abortSteer) })
	})
	first := answerProfileMessage("first", "original task")
	engine.ReceiveMessage(platform, first)
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("original task did not start")
	}
	engine.ReceiveMessage(platform, answerProfileMessage("supplement", "startup correction"))
	waitAnswerProfileTest(t, "correction accepted during startup", func() bool { return cronModelQueueDepth(engine) == 1 })
	releaseReady.Do(func() { close(ready) })
	select {
	case <-platform.progress:
	case <-time.After(3 * time.Second):
		t.Fatal("steer RPC blocked its own agent event consumer")
	}
	waitAnswerProfileTest(t, "correction accepted", func() bool {
		return strings.Contains(strings.Join(platform.getSent(), "\n"), engine.i18n.T(MsgSteered)) && cronModelQueueDepth(engine) == 0
	})
	backend.events <- Event{Type: EventResult, Content: "complete", Done: true}
	waitAnswerProfileTest(t, "original turn completes", func() bool { return !engine.sessions.GetOrCreateActive(first.SessionKey).Busy() })
	sends, steers := backend.inputs()
	if len(sends) != 1 || len(steers) != 1 {
		t.Fatalf("startup correction was not appended exactly once: sends=%+v steers=%+v", sends, steers)
	}
}
