package core

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestAuthenticationFailurePreservesResumeBinding(t *testing.T) {
	for _, relay := range []bool{false, true} {
		name := "interactive"
		if relay {
			name = "relay"
		}
		t.Run(name, func(t *testing.T) {
			authenticated := false
			var starts []string
			agent := &controllableAgent{startSessionFn: func(_ context.Context, id string) (AgentSession, error) {
				starts = append(starts, id)
				if !authenticated {
					return nil, WrapAuthenticationRequired(errors.New("login required"))
				}
				s := newControllableSession("saved-thread")
				if relay {
					s.events <- Event{Type: EventResult, Content: "resumed answer", Done: true}
				}
				return s, nil
			}}
			p := &stubPlatformEngine{n: "test"}
			e := NewEngine("test", agent, []Platform{p}, "", LangEnglish)
			t.Cleanup(func() { _ = e.Stop() })
			key := "test:user"
			sessions := e.sessions
			if relay {
				var err error
				_, sessions, key, err = e.relayContextForSourceSessionKey("source", "test:user")
				if err != nil {
					t.Fatal(err)
				}
			}
			session := sessions.GetOrCreateActive(key)
			session.SetAgentSessionID("saved-thread", agent.Name())
			session.AddHistory("user", "earlier context")
			attempt := func() {
				if relay {
					_, err := e.HandleRelay(context.Background(), "source", "test:user", "continue")
					if authenticated && err != nil {
						t.Fatal(err)
					}
					if !authenticated && !errors.Is(err, ErrAuthenticationRequired) {
						t.Fatalf("lost authentication error: %v", err)
					}
				} else {
					e.getOrCreateInteractiveStateWith(key, p, "ctx", session, sessions, nil, "", "")
				}
			}
			attempt()
			if len(starts) != 1 || starts[0] != "saved-thread" || session.GetAgentSessionID() != "saved-thread" {
				t.Fatalf("login failure started a fresh conversation: starts=%v binding=%q", starts, session.GetAgentSessionID())
			}
			authenticated = true
			attempt()
			if len(starts) != 2 || starts[1] != "saved-thread" || session.GetAgentSessionID() != "saved-thread" {
				t.Fatalf("retry did not resume the saved conversation: starts=%v binding=%q", starts, session.GetAgentSessionID())
			}
			if history := session.GetHistory(0); len(history) == 0 || history[0].Content != "earlier context" {
				t.Fatalf("authentication recovery lost history: %+v", history)
			}
		})
	}
}

func TestAuthenticationFailureHasSafeRecoveryCopyWithoutAutomaticFeedback(t *testing.T) {
	for _, rich := range []bool{false, true} {
		name := "plain"
		if rich {
			name = "rich"
		}
		t.Run(name, func(t *testing.T) {
			base := &stubPlatformEngine{n: "test"}
			var p Platform = base
			var richPlatform *stubRichCardSilentPlatform
			if rich {
				richPlatform = &stubRichCardSilentPlatform{stubPlatformEngine: stubPlatformEngine{n: "test"}}
				p = richPlatform
			}
			e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
			t.Cleanup(func() { _ = e.Stop() })
			if rich {
				e.SetDisplayConfig(DisplayCfg{Mode: "compact", CardMode: "rich"})
			}
			e.SetFeedbackConfig(true, "https://relay.example/v1/feedback")
			key := "test:auth-user"
			session := e.sessions.GetOrCreateActive(key)
			as := newControllableSession("saved-thread")
			state := &interactiveState{agentSession: as, platform: p, replyCtx: "ctx", currentSessionKey: key, currentUserID: "user"}
			e.interactiveStates[key] = state
			private := "refresh failed token=secret /Users/private/auth.json"
			as.events <- Event{Type: EventError, Error: WrapAuthenticationRequired(errors.New(private))}
			e.processInteractiveEvents(state, session, e.sessions, key, "auth-message", time.Now(), nil, nil, state.replyCtx)
			rendered := strings.Join(base.getSent(), "\n")
			if rich {
				starts, streams, updates, _ := richPlatform.snapshot()
				rendered = strings.Join(append(append(starts, streams...), updates...), "\n")
				if sent := richPlatform.getSent(); len(sent) != 0 {
					t.Fatalf("auth card followed by automatic feedback offer: %v", sent)
				}
			} else if len(base.getSent()) != 1 {
				t.Fatalf("auth failure followed by automatic feedback offer: %v", base.getSent())
			}
			for _, want := range []string{"Sign in again", "same system user", "configuration directory"} {
				if !strings.Contains(rendered, want) {
					t.Errorf("missing %q in %s", want, rendered)
				}
			}
			for _, leak := range []string{"token=secret", "/Users/private", private, "submit-token"} {
				if strings.Contains(rendered, leak) {
					t.Errorf("auth reply leaked %q: %s", leak, rendered)
				}
			}
		})
	}
}

func TestAuthenticationFailureQueuedNoticesUseEachUsersLanguage(t *testing.T) {
	zh := &stubPlatformEngine{n: "zh"}
	en := &stubPlatformEngine{n: "en"}
	e := NewEngine("test", &stubAgent{}, []Platform{zh, en}, "", LangAuto)
	state := &interactiveState{pendingMessages: []queuedMessage{{platform: zh, replyCtx: "zh", content: "请继续处理"}, {platform: en, replyCtx: "en", content: "please continue"}}}
	e.notifyDroppedQueuedMessages(state, WrapAuthenticationRequired(errors.New("private token=secret")))
	if got := strings.Join(zh.getSent(), "\n"); !strings.Contains(got, "重新登录") || strings.Contains(got, "secret") {
		t.Fatalf("Chinese recovery notice=%q", got)
	}
	if got := strings.Join(en.getSent(), "\n"); !strings.Contains(got, "Sign in again") || strings.Contains(got, "secret") {
		t.Fatalf("English recovery notice=%q", got)
	}
}

func TestAuthenticationBackgroundErrorsKeepConversationLanguage(t *testing.T) {
	for _, compress := range []bool{false, true} {
		for _, language := range []struct {
			text, other, want string
		}{
			{"请继续处理", "please continue", "重新登录"},
			{"please continue", "请继续处理", "Sign in again"},
		} {
			name := "background/" + language.want
			if compress {
				name = "compress/" + language.want
			}
			t.Run(name, func(t *testing.T) {
				p := &stubPlatformEngine{n: "test"}
				e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangAuto)
				t.Cleanup(func() { _ = e.Stop() })
				as := newControllableSession("saved-thread")
				key := "test:own-conversation"
				session := e.sessions.GetOrCreateActive(key)
				state := &interactiveState{agentSession: as, platform: p, replyCtx: "ctx", currentMessageID: "own-message"}
				state.setTurnRichCardCopy("own-message", e.i18n.RichCardCopyForText(language.text))
				// Another conversation has since changed the engine's auto locale.
				e.i18n.DetectAndSet(language.other)
				as.events <- Event{Type: EventError, Error: ErrAuthenticationRequired}
				if compress {
					unlocked := false
					e.processCompressEvents(state, session, e.sessions, key, p, "ctx", &unlocked, false)
				} else {
					ctx, cancel := context.WithCancel(e.ctx)
					defer cancel()
					e.runUnsolicitedReader(ctx, cancel, make(chan struct{}), state, as, session, e.sessions, key, "")
				}
				if got := strings.Join(p.getSent(), "\n"); !strings.Contains(got, language.want) {
					t.Fatalf("recovery notice borrowed another conversation's locale: %q", got)
				}
			})
		}
	}
}
