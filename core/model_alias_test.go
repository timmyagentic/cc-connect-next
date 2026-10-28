package core

import (
	"strings"
	"testing"
)

func TestResolveModelAlias_CaseInsensitive(t *testing.T) {
	models := []ModelOption{{Name: "gpt-5.3-codex", Alias: "Codex"}}

	got := resolveModelAlias(models, "codex")
	if got != "gpt-5.3-codex" {
		t.Fatalf("resolveModelAlias() = %q, want %q", got, "gpt-5.3-codex")
	}
}

func TestResolveModelAlias_NoMatchFallsBackToInput(t *testing.T) {
	models := []ModelOption{{Name: "gpt-5.3-codex", Alias: "codex"}}

	got := resolveModelAlias(models, "gpt-5.4")
	if got != "gpt-5.4" {
		t.Fatalf("resolveModelAlias() = %q, want original input", got)
	}
}

func TestModelCommand_NameAliasAndNumberRespectScope(t *testing.T) {
	for _, scope := range []string{"chat", "default"} {
		for _, selection := range []string{"group-B", "PIN", "3"} {
			t.Run(scope+"/"+selection, func(t *testing.T) {
				e, p, a := newChatModelEngine(t, "")
				e.SetAdminFrom("owner")
				var saves int
				e.SetModelSaveFunc(func(string) error { saves++; return nil })
				command := "/model " + selection
				if scope == "default" {
					command = "/model default " + selection
				}
				e.ReceiveMessage(p, chatModelMessage("A", "topic", "selection", command))
				binding, pinned := e.chatModelBinding("test:A:root:topic")
				if scope == "chat" {
					if !pinned || binding.Model != "group-B" || a.GetModel() != "project-A" || saves != 0 {
						t.Fatalf("chat selection changed wrong scope: binding=%+v pinned=%v default=%q saves=%d", binding, pinned, a.GetModel(), saves)
					}
				} else if pinned || a.GetModel() != "group-B" || saves != 1 {
					t.Fatalf("default selection changed wrong scope: binding=%+v pinned=%v default=%q saves=%d", binding, pinned, a.GetModel(), saves)
				}
				if visible := strings.Join(p.getSent(), "\n"); !strings.Contains(visible, "group-B") {
					t.Fatalf("missing selection feedback: %s", visible)
				}
			})
		}
	}
}
