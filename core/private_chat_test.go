package core

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func privateChatMessage(user, id, content string) *Message {
	key := "test:inbox:" + user // Deliberately different from the persisted identity key.
	return &Message{SessionKey: key, Platform: "test", UserID: user, IsDirect: true, MessageID: id, Content: content, ReplyCtx: key}
}

func privateChatTurn(t *testing.T, e *Engine, p *cronModelPlatform, a *groupModelTestAgent, msg *Message, model, effort string) {
	t.Helper()
	p.clearSent()
	groupModelChat(t, e, p, a, msg, model)
	if got, want := strings.Join(p.getSent(), "\n"), "effort="+effort+" model="+model; !strings.Contains(got, want) {
		t.Fatalf("%s: want visible reply %q, got %s", msg.MessageID, want, got)
	}
}

func TestPrivateChat_VerifiedIdentityIsolatesUsersGroupsAndProjects(t *testing.T) {
	e, p, a := newGroupReasoningEngine(t, "")
	e.ReceiveMessage(p, groupModelMessage("alice", "topic", "group-model", "/model group group-C"))
	e.ReceiveMessage(p, groupModelMessage("alice", "topic", "group-effort", "/reasoning group low"))
	direct := privateChatMessage("alice", "model", "/model chat group-B")
	direct.GroupID = "transport-chat" // A p2p transport chat ID must not become a group scope.
	e.ReceiveMessage(p, direct)
	e.ReceiveMessage(p, privateChatMessage("alice", "effort", "/effort chat high"))
	for session, want := range map[string]string{"test:inbox:alice": "test:dm:alice", "test:alice:root:topic": "test:alice"} {
		if got := e.groupModelKey(session); got != want {
			t.Fatalf("identity for %q = %q, want %q", session, got, want)
		}
	}
	privateChatTurn(t, e, p, a, privateChatMessage("alice", "alice", "hello"), "group-B", "high")
	reasoningChat(t, e, p, a, privateChatMessage("bob", "bob", "hello"), "medium")
	privateChatTurn(t, e, p, a, groupModelMessage("alice", "other-topic", "group", "hello"), "group-C", "low")

	// A private thread has a different session key but the same verified user.
	thread := privateChatMessage("alice", "private-thread", "hello")
	thread.SessionKey, thread.ReplyCtx = "test:private-thread:root:one", "private-thread"
	privateChatTurn(t, e, p, a, thread, "group-B", "high")
	if got := e.groupModelKey(thread.SessionKey); got != "test:dm:alice" {
		t.Fatalf("private thread identity = %q", got)
	}

	// Each project's store must isolate even identical platform/user identities.
	other, op, oa := newGroupReasoningEngine(t, "")
	reasoningChat(t, other, op, oa, privateChatMessage("alice", "other-project", "hello"), "medium")
	if _, _, setters := a.snapshot(); setters != 0 || a.GetModel() != "project-A" || a.GetReasoningEffort() != "medium" {
		t.Fatal("chat selection mutated shared defaults")
	}
}

func TestPrivateChat_MissingVerifiedIdentityCannotBind(t *testing.T) {
	for _, tc := range []struct {
		name, user, group string
		direct            bool
		callback          bool
	}{
		{name: "missing-user", direct: true},
		{name: "missing-user-with-chat-id", group: "transport-chat", direct: true},
		{name: "blank-user", user: "  ", direct: true},
		{name: "unverified-user", user: "alice"},
		{name: "unverified-callback", callback: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, p, a := newGroupReasoningEngine(t, "")
			msg := &Message{SessionKey: "test:dm:alice", Platform: "test", UserID: tc.user, GroupID: tc.group, IsDirect: tc.direct, IsCardAction: tc.callback, ReplyCtx: "private"}
			for i, command := range []string{"/model chat group-B", "/effort chat high"} {
				msg.MessageID, msg.Content = fmt.Sprint(i), command
				e.ReceiveMessage(p, msg)
			}
			if key := e.groupModelKey(msg.SessionKey); key != "" {
				t.Fatalf("inferred unverified identity %q from session key", key)
			}
			if _, bound := e.projectState.GroupModelBinding("test:dm:alice"); bound {
				t.Fatal("unverified identity acquired a model binding")
			}
			if _, bound := e.projectState.GroupReasoningBinding("test:dm:alice"); bound {
				t.Fatal("unverified identity acquired an effort binding")
			}
			if _, _, setters := a.snapshot(); setters != 0 {
				t.Fatal("rejected chat command changed the shared default")
			}
		})
	}
}

func TestPrivateChat_IndependentResetsAndLegacyAliasesPreserveHistory(t *testing.T) {
	e, p, a := newGroupReasoningEngine(t, "")
	privateChatTurn(t, e, p, a, privateChatMessage("alice", "before", "remember this"), "project-A", "medium")
	for i, step := range []struct{ command, model, effort string }{
		{"/model chat group-B", "group-B", "medium"},
		{"/effort chat high", "group-B", "high"},
		{"/reasoning chat reset", "group-B", "medium"},
		{"/reasoning group high", "group-B", "high"},
		{"/model group reset", "project-A", "high"},
	} {
		session := e.sessions.GetOrCreateActive("test:inbox:alice")
		history, backendID := session.GetHistory(0), session.GetAgentSessionID()
		e.ReceiveMessage(p, privateChatMessage("alice", fmt.Sprintf("command-%d", i), step.command))
		current := e.sessions.GetOrCreateActive("test:inbox:alice")
		if current != session || current.GetAgentSessionID() != backendID || !reflect.DeepEqual(current.GetHistory(0), history) {
			t.Fatalf("%s reset the conversation", step.command)
		}
		privateChatTurn(t, e, p, a, privateChatMessage("alice", fmt.Sprintf("turn-%d", i), "continue"), step.model, step.effort)
	}
	// Commands without chat/group retain their existing shared-default meaning.
	e.ReceiveMessage(p, privateChatMessage("alice", "shared-model", "/model project-C"))
	e.ReceiveMessage(p, privateChatMessage("alice", "shared-effort", "/effort low"))
	if a.GetModel() != "project-C" || a.GetReasoningEffort() != "low" {
		t.Fatal("bare commands stopped updating shared defaults")
	}
	privateChatTurn(t, e, p, a, privateChatMessage("alice", "still-pinned-effort", "hello"), "project-C", "high")
	groupModelChat(t, e, p, a, privateChatMessage("bob", "shared-default", "hello"), "project-C")
}

func TestPrivateChat_RestartAndSessionResetKeepVerifiedCallbackScope(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	e, p, a := newGroupReasoningEngine(t, path)
	e.ReceiveMessage(p, privateChatMessage("alice", "model", "/model chat group-B"))
	e.ReceiveMessage(p, privateChatMessage("alice", "effort", "/effort chat high"))
	privateChatTurn(t, e, p, a, privateChatMessage("alice", "before-new", "hello"), "group-B", "high")
	for _, command := range []string{"/new", "/reset"} {
		e.ReceiveMessage(p, privateChatMessage("alice", command, command))
		// /reset can be forwarded to the native Agent instead of handled by
		// core. Wait for that turn before counting the following user turn.
		waitAnswerProfileTest(t, command+" complete", func() bool {
			return !e.sessions.GetOrCreateActive("test:inbox:alice").Busy()
		})
		privateChatTurn(t, e, p, a, privateChatMessage("alice", command+"-after", "hello"), "group-B", "high")
	}
	if err := e.Stop(); err != nil {
		t.Fatal(err)
	}
	e, p, a = newGroupReasoningEngine(t, path)
	// Legacy callbacks omit routing flags but retain the verified operator ID.
	callback := &Message{SessionKey: "test:inbox:alice", Platform: "test", UserID: "alice", IsCardAction: true, ReplyCtx: "private"}
	for _, command := range []string{"/model", "/effort"} {
		p.clearSent()
		callback.MessageID, callback.Content = command, command
		e.ReceiveMessage(p, callback)
		want := "group-B"
		if command == "/effort" {
			want = "high"
		}
		if got := strings.Join(p.getSent(), "\n"); !strings.Contains(got, want) {
			t.Fatalf("%s lost persisted private selection: %s", command, got)
		}
	}
	for _, tc := range []struct {
		card                           *Card
		chatPrefix, sharedPrefix       string
		selected, sharedSelected, next string
	}{
		{e.renderModelCard(callback.SessionKey), "cmd:/model chat ", "cmd:/model switch ", "group-B", "project-A", "group-C"},
		{e.renderReasoningCardForAgent(callback.SessionKey, a), "cmd:/reasoning chat ", "cmd:/reasoning ", "high", "medium", "low"},
	} {
		var chatBox, sharedBox *CardSelect
		var nextCommand string
		var selectors int
		for _, element := range tc.card.Elements {
			box, ok := element.(CardSelect)
			if !ok {
				continue
			}
			selectors++
			isChat := box.InitValue == tc.chatPrefix+tc.selected
			prefix := tc.sharedPrefix
			if isChat {
				prefix, chatBox = tc.chatPrefix, &box
			} else {
				sharedBox = &box
			}
			var hasReset bool
			for _, option := range box.Options {
				if !strings.HasPrefix(option.Value, prefix) || !isChat && strings.HasPrefix(option.Value, tc.chatPrefix) {
					t.Fatalf("mixed chat/shared card choices: %+v", box)
				}
				hasReset = hasReset || option.Value == tc.chatPrefix+"reset"
				if option.Value == tc.chatPrefix+tc.next {
					nextCommand = strings.TrimPrefix(option.Value, "cmd:")
				}
			}
			if isChat && !hasReset {
				t.Fatalf("chat selector lacks reset: %+v", box)
			}
		}
		if selectors != 2 || chatBox == nil || sharedBox == nil || sharedBox.InitValue != tc.sharedPrefix+tc.sharedSelected || nextCommand == "" {
			t.Fatalf("missing separate chat/shared selectors: %+v", tc.card)
		}
		callback.MessageID, callback.Content = nextCommand, nextCommand
		e.ReceiveMessage(p, callback)
	}
	if got := e.groupModelKey(callback.SessionKey); got != "test:dm:alice" {
		t.Fatalf("callback replaced verified identity: %q", got)
	}
	privateChatTurn(t, e, p, a, privateChatMessage("alice", "after-callbacks", "hello"), "group-C", "low")
	if a.GetModel() != "project-A" || a.GetReasoningEffort() != "medium" {
		t.Fatal("chat card changed shared defaults")
	}
}

func TestPrivateChat_CallbackCannotBorrowAnotherUsersScope(t *testing.T) {
	for _, user := range []string{"bob", ""} {
		t.Run("operator="+user, func(t *testing.T) {
			e, p, a := newGroupReasoningEngine(t, "")
			e.ReceiveMessage(p, privateChatMessage("alice", "model", "/model chat group-B"))
			e.ReceiveMessage(p, privateChatMessage("alice", "effort", "/effort chat high"))
			for _, command := range []string{"/model chat group-C", "/reasoning chat low"} {
				p.clearSent()
				e.ReceiveMessage(p, &Message{SessionKey: "test:inbox:alice", Platform: "test", UserID: user, IsCardAction: true, MessageID: command, Content: command, ReplyCtx: "forwarded-card"})
				if got := strings.Join(p.getSent(), "\n"); !strings.Contains(got, e.i18n.T(MsgModelGroupOnly)) {
					t.Fatalf("%s did not visibly reject the mismatched operator: %s", command, got)
				}
				if key := e.groupModelKey("test:inbox:alice"); key != "test:dm:alice" {
					t.Fatalf("rejected callback changed verified identity: %q", key)
				}
				if binding, ok := e.groupModelBinding("test:inbox:alice"); !ok || binding.Model != "group-B" {
					t.Fatalf("rejected callback changed the owner's model: %+v", binding)
				}
				if binding, ok := e.groupReasoningBinding("test:inbox:alice"); !ok || binding.Effort != "high" {
					t.Fatalf("rejected callback changed the owner's effort: %+v", binding)
				}
			}
			privateChatTurn(t, e, p, a, privateChatMessage("alice", "owner-after", "hello"), "group-B", "high")
		})
	}
}

func TestPrivateChat_OneShotAndScheduledOverridesResolveEachField(t *testing.T) {
	e, p, a := newGroupReasoningEngine(t, "")
	e.ReceiveMessage(p, privateChatMessage("alice", "model", "/model chat group-B"))
	e.ReceiveMessage(p, privateChatMessage("alice", "effort", "/effort chat high"))
	e.SetAnswerProfiles(AnswerProfiles{Fast: &AnswerProfileOptions{Model: "fast-D"}, Quality: &AnswerProfileOptions{ReasoningEffort: "low"}})
	privateChatTurn(t, e, p, a, privateChatMessage("alice", "fast", "/fast hello"), "fast-D", "high")
	privateChatTurn(t, e, p, a, privateChatMessage("alice", "quality", "/quality hello"), "group-B", "low")
	explicit := privateChatMessage("alice", "explicit-model", "/fast hello")
	explicit.ModelOverride = "group-C"
	privateChatTurn(t, e, p, a, explicit, "group-C", "high")
	explicit = privateChatMessage("alice", "explicit-effort", "/quality hello")
	explicit.ReasoningEffortOverride = "xhigh"
	privateChatTurn(t, e, p, a, explicit, "group-B", "xhigh")
	for i, tc := range []struct{ model, effort, wantModel, wantEffort string }{
		{"", "", "group-B", "high"},
		{"project-C", "", "project-C", "high"},
		{"", "low", "group-B", "low"},
		{"project-C", "low", "project-C", "low"},
	} {
		job := cronModelJob(fmt.Sprintf("private-task-%d", i), tc.model)
		job.SessionKey, job.SessionMode, job.ReasoningEffort = "test:inbox:alice", "new_per_run", tc.effort
		before, _, _ := a.snapshot()
		if err := e.ExecuteCronJob(job); err != nil {
			t.Fatal(err)
		}
		calls, _, _ := a.snapshot()
		if len(calls) != len(before)+1 {
			t.Fatalf("task did not execute exactly once: %+v", calls)
		}
		if got := calls[len(calls)-1]; got.model != tc.wantModel || got.options.ReasoningEffort != tc.wantEffort {
			t.Fatalf("task per-field precedence: got %+v, want %s/%s", got, tc.wantModel, tc.wantEffort)
		}
	}
	privateChatTurn(t, e, p, a, privateChatMessage("alice", "ordinary-after", "hello"), "group-B", "high")
	if _, _, setters := a.snapshot(); setters != 0 {
		t.Fatal("one-shot/task selection changed shared defaults")
	}
}
