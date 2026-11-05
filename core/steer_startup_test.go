package core

import (
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type startupSteerInput struct {
	prompt string
	images []ImageAttachment
	files  []FileAttachment
}

type startupSteerSession struct {
	*answerProfileTestSession
	sendEntered chan struct{}
	releaseSend chan struct{}
	steerError  error
	sends       []startupSteerInput
	steers      []startupSteerInput
}

func (s *startupSteerSession) Send(prompt string, images []ImageAttachment, files []FileAttachment) error {
	s.mu.Lock()
	first := len(s.sends) == 0
	s.sends = append(s.sends, startupSteerInput{prompt, images, files})
	s.mu.Unlock()
	if first && s.releaseSend != nil {
		close(s.sendEntered)
		<-s.releaseSend
	}
	if !first {
		s.events <- Event{Type: EventResult, Content: "queued request complete", Done: true}
	}
	return nil
}

func (s *startupSteerSession) SendWithTurnOptions(prompt string, images []ImageAttachment, files []FileAttachment, _ TurnOptions) error {
	return s.Send(prompt, images, files)
}

func (s *startupSteerSession) Steer(prompt string, images []ImageAttachment, files []FileAttachment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.steers = append(s.steers, startupSteerInput{prompt, images, files})
	return s.steerError
}

func (s *startupSteerSession) inputs() ([]startupSteerInput, []startupSteerInput) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]startupSteerInput(nil), s.sends...), append([]startupSteerInput(nil), s.steers...)
}

func TestCUJ_SteerStartupSupplementsPreserveOrderAndAttachments(t *testing.T) {
	for _, phase := range []string{"context", "turn"} {
		t.Run(phase, func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			backend := &startupSteerSession{answerProfileTestSession: newAnswerProfileTestSession()}
			agent := &controllableAgent{nextSession: backend}
			basePlatform := &stubPlatformEngine{n: "test"}
			var platform Platform = basePlatform
			switch phase {
			case "context":
				platform = &contextTestPlatform{
					stubPlatformEngine: basePlatform,
					backgrounds:        map[string]MessageContext{},
					onLoad: func(count int) {
						if count == 1 {
							close(entered)
							<-release
						}
					},
				}
			case "turn":
				backend.sendEntered, backend.releaseSend = entered, release
			}
			engine := NewEngine("test", agent, []Platform{platform}, "", LangEnglish)
			t.Cleanup(func() { _ = engine.Stop() })
			t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
			first := answerProfileMessage("first", "original task")
			first.ReplyCtx = first.SessionKey
			first.UserMessageTimeMs = time.Now().UnixMilli()
			engine.ReceiveMessage(platform, first)
			select {
			case <-entered:
			case <-time.After(3 * time.Second):
				t.Fatal("startup did not reach its controlled wait")
			}

			var supplements []*Message
			for i, id := range []string{"second", "third"} {
				msg := answerProfileMessage(id, "supplement "+id)
				msg.ReplyCtx = msg.SessionKey
				msg.UserMessageTimeMs = first.UserMessageTimeMs + int64(i) + 1
				msg.Images = []ImageAttachment{{MimeType: "image/png", Data: []byte(id), FileName: "evidence.png"}}
				msg.Files = []FileAttachment{{MimeType: "text/plain", Data: []byte("file " + id), FileName: "evidence.txt"}}
				supplements = append(supplements, msg)
				engine.ReceiveMessage(platform, msg)
				waitAnswerProfileTest(t, "startup supplement accepted", func() bool { return cronModelQueueDepth(engine) == i+1 })
			}
			before := strings.Join(basePlatform.getSent(), "\n")
			if strings.Count(before, engine.i18n.T(MsgSteerPending)) != 2 || strings.Contains(before, engine.i18n.T(MsgSteered)) {
				t.Fatalf("startup acknowledgement claimed delivery too early: %q", before)
			}
			releaseOnce.Do(func() { close(release) })
			waitAnswerProfileTest(t, "both supplements added before task completion", func() bool {
				_, steers := backend.inputs()
				return len(steers) == 2 && strings.Count(strings.Join(basePlatform.getSent(), "\n"), engine.i18n.T(MsgSteered)) == 2
			})
			sends, steers := backend.inputs()
			if len(sends) != 1 || !engine.sessions.GetOrCreateActive(first.SessionKey).Busy() {
				t.Fatalf("startup supplements started another turn: sends=%+v", sends)
			}
			for i, msg := range supplements {
				if !strings.Contains(steers[i].prompt, msg.Content) || !reflect.DeepEqual(steers[i].images, msg.Images) || !reflect.DeepEqual(steers[i].files, scopeFileAttachments(msg.Files, msg.MessageID)) {
					t.Fatalf("supplement %d lost ordering or attachments: %+v", i, steers[i])
				}
			}
			backend.events <- Event{Type: EventResult, Content: "original task complete", Done: true}
			waitAnswerProfileTest(t, "original turn completes", func() bool { return !engine.sessions.GetOrCreateActive(first.SessionKey).Busy() })
			sends, steers = backend.inputs()
			if len(sends) != 1 || len(steers) != 2 || cronModelQueueDepth(engine) != 0 {
				t.Fatalf("supplements were replayed after completion: sends=%+v steers=%+v", sends, steers)
			}
		})
	}
}

func TestCUJ_SteerStartupOutcomeControlsQueueFallback(t *testing.T) {
	for _, tc := range []struct {
		name       string
		steerError error
		legacy     bool
		wantSends  int
		wantSteers int
		receipt    MsgKey
	}{
		{name: "rejected", steerError: ErrSteerRejected, wantSends: 2, wantSteers: 1, receipt: MsgMessageQueued},
		{name: "unknown", steerError: ErrSteerOutcomeUnknown, wantSends: 1, wantSteers: 1, receipt: MsgSteerOutcomeUnknown},
		{name: "unsupported", legacy: true, wantSends: 2, receipt: MsgMessageQueued},
	} {
		t.Run(tc.name, func(t *testing.T) {
			backend := &startupSteerSession{
				answerProfileTestSession: newAnswerProfileTestSession(),
				sendEntered:              make(chan struct{}), releaseSend: make(chan struct{}), steerError: tc.steerError,
			}
			var session AgentSession = backend
			if tc.legacy {
				session = struct{ AgentSession }{backend}
			}
			platform := &stubPlatformEngine{n: "test"}
			engine := NewEngine("test", &controllableAgent{nextSession: session}, []Platform{platform}, "", LangEnglish)
			var release sync.Once
			t.Cleanup(func() { _ = engine.Stop() })
			t.Cleanup(func() { release.Do(func() { close(backend.releaseSend) }) })
			first := answerProfileMessage("first", "original task")
			first.UserMessageTimeMs = time.Now().UnixMilli()
			engine.ReceiveMessage(platform, first)
			select {
			case <-backend.sendEntered:
			case <-time.After(3 * time.Second):
				t.Fatal("turn did not reach its controlled startup wait")
			}
			supplement := answerProfileMessage("supplement", "startup correction")
			supplement.UserMessageTimeMs = first.UserMessageTimeMs + 1
			engine.ReceiveMessage(platform, supplement)
			waitAnswerProfileTest(t, "supplement queued during startup", func() bool { return cronModelQueueDepth(engine) == 1 })
			release.Do(func() { close(backend.releaseSend) })
			waitAnswerProfileTest(t, "accurate outcome acknowledgement", func() bool {
				return strings.Contains(strings.Join(platform.getSent(), "\n"), engine.i18n.T(tc.receipt))
			})
			if strings.Contains(strings.Join(platform.getSent(), "\n"), engine.i18n.T(MsgSteered)) {
				t.Fatal("unsuccessful or unknown steer was acknowledged as delivered")
			}
			backend.events <- Event{Type: EventResult, Content: "original task complete", Done: true}
			waitAnswerProfileTest(t, "fallback completes", func() bool { return !engine.sessions.GetOrCreateActive(first.SessionKey).Busy() })
			sends, steers := backend.inputs()
			if len(sends) != tc.wantSends || len(steers) != tc.wantSteers || cronModelQueueDepth(engine) != 0 {
				t.Fatalf("startup input was lost or redelivered: sends=%+v steers=%+v", sends, steers)
			}
			if tc.wantSends == 2 && sends[1].prompt != supplement.Content {
				t.Fatalf("fallback changed the input: %+v", sends[1])
			}
		})
	}
}
