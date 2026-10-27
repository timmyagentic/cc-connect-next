package core

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// Hold the first hydration until cancellation, and then until the test releases
// it. This separates cancellation of the network operation from the later Send.
type cancelContextMediaPlatform struct {
	*mediaContextPlatform
	blockOnce sync.Once
	entered   chan struct{}
	cancelled chan error
	release   chan struct{}
}

func (p *cancelContextMediaPlatform) HydrateMessageContext(ctx context.Context, replyCtx any, entries []ContextMessage) ([]ContextMessage, error) {
	block := false
	p.blockOnce.Do(func() { block = true })
	if !block {
		return p.mediaContextPlatform.HydrateMessageContext(ctx, replyCtx, entries)
	}
	close(p.entered)
	select {
	case <-ctx.Done():
		p.cancelled <- ctx.Err()
		<-p.release
		return entries, ctx.Err()
	case <-p.release:
		// Cleanup must also unblock the pre-fix implementation, whose context
		// never receives the user's cancellation.
		return entries, nil
	}
}

type cancelContextAgent struct {
	stubAgent
	mu      sync.Mutex
	starts  int
	session *cancelContextAgentSession
}

func (a *cancelContextAgent) StartSession(context.Context, string) (AgentSession, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.starts++
	return a.session, nil
}

type cancelContextAgentSession struct {
	*cujAgentSession
	holdNext    bool // protected by cujAgentSession.mu
	cancelCalls int  // protected by cujAgentSession.mu
}

func (s *cancelContextAgentSession) Send(prompt string, images []ImageAttachment, files []FileAttachment) error {
	s.mu.Lock()
	if s.holdNext {
		s.holdNext = false
		s.sentPrompts = append(s.sentPrompts, prompt)
		s.mu.Unlock()
		// The test emits completion after the follow-up is in the queue.
		return nil
	}
	s.mu.Unlock()
	return s.cujAgentSession.Send(prompt, images, files)
}

func (s *cancelContextAgentSession) CancelTurn() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cancelCalls++
	// Nothing has reached the backend during hydration. A graceful cancel
	// therefore emits no completion and deliberately keeps the session alive.
	return nil
}

func TestMessageContextCancellationDuringHydrationDoesNotSendAndReusesSession(t *testing.T) {
	for _, path := range []string{"foreground", "queued", "drain"} {
		for _, action := range []string{"stop", "recall"} {
			t.Run(path+"/"+action, func(t *testing.T) {
				env, base := newContextCUJEnv(t)
				p := &cancelContextMediaPlatform{
					mediaContextPlatform: &mediaContextPlatform{contextTestPlatform: base},
					entered:              make(chan struct{}),
					cancelled:            make(chan error, 1),
					release:              make(chan struct{}),
				}
				var releaseOnce sync.Once
				release := func() { releaseOnce.Do(func() { close(p.release) }) }
				t.Cleanup(release)
				as := &cancelContextAgentSession{cujAgentSession: newCUJAgentSession()}
				agent := &cancelContextAgent{session: as}
				env.engine.agent = agent
				const key = "test:media-cancel"
				const targetID = "cancel-during-hydration"
				session := env.engine.sessions.GetOrCreateActive(key)
				expectedSends := 0
				var state *interactiveState
				if path != "foreground" {
					as.holdNext = path == "queued"
					contextReceive(env, p, key, "alice", "warmup", "warmup")
					env.waitFor("warmup send", 2*time.Second, func() bool {
						return len(as.getSentPrompts()) == 1
					})
					env.engine.interactiveMu.Lock()
					state = env.engine.interactiveStates[key]
					env.engine.interactiveMu.Unlock()
					expectedSends = 1
					if path == "drain" {
						env.waitFor("warmup completion", 2*time.Second, func() bool { return !session.Busy() })
						env.engine.stopUnsolicitedReader(state)
					}
				}
				p.setBackground(key, []ContextMessage{{
					ID: "history-photo", SenderID: "bob", Text: "retryable history photo",
					Resources: []ContextResource{{Kind: "image", Key: "opaque"}},
				}}, nil)
				if path == "drain" {
					// Deterministically exercise the orphan-drain caller without
					// racing the foreground loop's empty-queue/unlock boundary.
					if !session.TryLock() {
						t.Fatal("could not own the idle session for drain")
					}
					state.mu.Lock()
					state.pendingMessages = append(state.pendingMessages, queuedMessage{
						messageID: targetID, platform: p, replyCtx: key,
						content: "cancel this request", userID: "alice", userName: "alice",
						msgPlatform: p.Name(), msgSessionKey: key,
					})
					state.mu.Unlock()
					go func() {
						if !env.engine.drainPendingMessages(state, session, env.engine.sessions, key) {
							session.Unlock() // same early-return fallback as the foreground owner
						}
					}()
				} else {
					contextReceive(env, p, key, "alice", targetID, "cancel this request")
					if path == "queued" {
						state.mu.Lock()
						queued := len(state.pendingMessages)
						state.mu.Unlock()
						if queued != 1 {
							t.Fatalf("expected one queued request, got %d", queued)
						}
						as.events <- Event{Type: EventResult, Content: "warmup complete", Done: true}
					}
				}
				select {
				case <-p.entered:
				case <-time.After(2 * time.Second):
					t.Fatal("request never reached blocking hydration")
				}
				env.plat.clearSent()
				if action == "stop" {
					contextReceive(env, p, key, "alice", "stop-command", "/stop")
				} else {
					env.engine.ReceiveMessage(p, &Message{
						SessionKey: key, Platform: p.Name(), MessageID: targetID,
						UserID: "alice", ReplyCtx: key, Recalled: true,
					})
				}
				select {
				case err := <-p.cancelled:
					if !errors.Is(err, context.Canceled) {
						t.Fatalf("hydration ended with %v, want user cancellation", err)
					}
				case <-time.After(2 * time.Second):
					t.Fatal("stop/recall did not cancel hydration context")
				}
				as.mu.Lock()
				cancels := as.cancelCalls
				as.mu.Unlock()
				if cancels != 1 {
					t.Fatalf("graceful cancellation called %d times, want 1", cancels)
				}
				release()
				env.waitFor("cancelled preparation releases session", 2*time.Second, func() bool { return !session.Busy() })
				if got := as.getSentPrompts(); len(got) != expectedSends {
					t.Fatalf("cancelled request reached Send: %v", got)
				}
				if action == "recall" && len(env.plat.getSent()) != 0 {
					t.Fatalf("recalled preparation produced a reply: %v", env.plat.getSent())
				}
				if env.sentContains("context is incomplete") || env.sentContains("attachments could not be read") {
					t.Fatalf("user cancellation produced a failure warning: %v", env.plat.getSent())
				}
				env.plat.clearSent()
				contextReceive(env, p, key, "alice", "after-cancellation", "retry after cancellation")
				env.waitFor("same session answers subsequent request", 2*time.Second, func() bool {
					return len(as.getSentPrompts()) == expectedSends+1 && env.sentContains("ok") && !session.Busy()
				})
				prompts := as.getSentPrompts()
				if !strings.Contains(prompts[len(prompts)-1], "retryable history photo") {
					t.Fatal("cancelled preparation consumed history receipts")
				}
				agent.mu.Lock()
				starts := agent.starts
				agent.mu.Unlock()
				as.mu.Lock()
				closes := as.closeCount
				as.mu.Unlock()
				if starts != 1 || closes != 0 || !as.Alive() {
					t.Fatalf("graceful cancellation replaced or closed session: starts=%d closes=%d alive=%v", starts, closes, as.Alive())
				}
			})
		}
	}
}
