package core

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestStopWaitsForOldProcessBeforeResuming(t *testing.T) {
	key := "test:stop-respawn"
	started := make(chan struct{}, 1)
	agent := &controllableAgent{startSessionFn: func(context.Context, string) (AgentSession, error) {
		started <- struct{}{}
		return newControllableSession("lineage"), nil
	}}
	e := NewEngine("test", agent, nil, "", LangEnglish)
	p := &stubPlatformEngine{n: "plain"}
	session := e.sessions.GetOrCreateActive(key)
	session.SetAgentSessionID("lineage", agent.Name())
	old := &failOnceCodexLikeSession{threadID: "lineage", events: make(chan Event, 4)}
	old.alive.Store(true)
	old.sends.Store(1)
	closing := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	probe := &idleResetCloseProbe{AgentSession: old, check: func() { close(closing); <-release }}
	e.interactiveStates[key] = &interactiveState{agentSession: probe, agent: agent, platform: p, replyCtx: "ctx", stopCh: make(chan struct{})}
	if !e.stopInteractiveSession(key, p, "ctx") {
		t.Fatal("stop failed")
	}
	select {
	case <-closing:
	case <-time.After(time.Second):
		t.Fatal("teardown did not start")
	}
	created := make(chan *interactiveState, 1)
	go func() {
		created <- e.getOrCreateInteractiveStateWith(key, p, "new", session, e.sessions, nil, key, "retry")
	}()
	select {
	case <-started:
		t.Error("new backend resumed while old process was still closing")
	case <-time.After(50 * time.Millisecond):
	}
	releaseOnce.Do(func() { close(release) })
	select {
	case state := <-created:
		if state == nil {
			t.Error("no replacement after successful close")
		}
	case <-time.After(time.Second):
		t.Fatal("replacement blocked after close")
	}
	e.cleanupInteractiveState(key)
}

func TestOverlappingTeardownsWaitForBothCompletionOrders(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(fmt.Sprint(reverse), func(t *testing.T) {
			e := newTestEngine()
			first := e.beginSessionClose("key")
			second := e.beginSessionClose("key")
			returned := make(chan bool, 1)
			go func() { returned <- e.awaitSessionClose("key") }()
			if reverse {
				second()
			} else {
				first()
			}
			select {
			case <-returned:
				t.Fatal("one teardown still runs")
			case <-time.After(20 * time.Millisecond):
			}
			if reverse {
				first()
			} else {
				second()
			}
			select {
			case ok := <-returned:
				if !ok {
					t.Fatal("normal teardown marked unsafe")
				}
			case <-time.After(time.Second):
				t.Fatal("fence did not release")
			}
		})
	}
}

func TestFailedCloseStartsFreshAndKeepsLocalHistory(t *testing.T) {
	key := "test:failed-close"
	var resumed string
	agent := &controllableAgent{startSessionFn: func(_ context.Context, id string) (AgentSession, error) {
		resumed = id
		return newControllableSession("fresh"), nil
	}}
	e := NewEngine("test", agent, nil, "", LangEnglish)
	session := e.sessions.GetOrCreateActive(key)
	session.SetAgentSessionID("unsafe-old", agent.Name())
	session.AddHistory("user", "keep the local history")
	e.markUnsafeResume(key)
	p := &stubPlatformEngine{n: "plain"}
	state := e.getOrCreateInteractiveStateWith(key, p, "ctx", session, e.sessions, nil, key, "next")
	if state == nil || resumed != "" {
		t.Fatalf("unsafe resume = %q", resumed)
	}
	if len(session.GetHistory(0)) != 1 {
		t.Fatal("local history was deleted")
	}
	if e.consumeUnsafeResume(key) {
		t.Fatal("unsafe flag not consumed")
	}
	e.cleanupInteractiveState(key)
}
