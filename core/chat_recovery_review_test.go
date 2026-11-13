package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestChatRecovery_HistoryMarkerRequiresDurableSession(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blocker, []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	sessions := NewSessionManager(filepath.Join(blocker, "sessions.json"))
	r := &chatRecovery{dir: dir, state: chatRecoveryState{Version: 1}}
	if err := r.saveHistory(sessions); err == nil {
		t.Fatal("history marker acknowledged a failed session save")
	}
	if r.hadHistory() {
		t.Fatal("failed history save was marked durable")
	}
}

func TestChatRecovery_StartupCleansOnlyOwnedClosedCheckpoints(t *testing.T) {
	e := NewEngine("owner", &stubAgent{}, nil, filepath.Join(t.TempDir(), "sessions.json"), LangEnglish)
	for _, tc := range []struct {
		name    string
		version int
		project string
		remove  bool
	}{
		{"owned", 1, "owner", true}, {"foreign", 1, "other", false}, {"future", 2, "owner", false},
	} {
		dir := filepath.Join(e.chatRecoveryRoot(), tc.name)
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		state := chatRecoveryState{Version: tc.version, Project: tc.project, Closed: true}
		if err := writeChatRecoveryFile(filepath.Join(dir, "state.json"), state); err != nil {
			t.Fatal(err)
		}
	}
	e.restoreChatRecoveries()
	for _, name := range []string{"owned", "foreign", "future"} {
		_, err := os.Stat(filepath.Join(e.chatRecoveryRoot(), name))
		if name == "owned" {
			if !os.IsNotExist(err) {
				t.Fatalf("owned sensitive checkpoint retained: %v", err)
			}
		} else if err != nil {
			t.Fatalf("unowned checkpoint %s touched: %v", name, err)
		}
	}
}

func TestChatRecovery_ChangedNativeIdentityClearsStaleGroup(t *testing.T) {
	p := newRecoveryCardPlatform()
	e := NewEngine("owner", &recoveryNativeAgent{ledger: &recoveryNativeLedger{}}, []Platform{p}, filepath.Join(t.TempDir(), "sessions.json"), LangEnglish)
	session := e.sessions.GetOrCreateActive("chat")
	session.SetAgentSessionID("replacement", "recovery-native")
	var group []*chatRecovery
	for _, name := range []string{"first", "queued"} {
		dir := filepath.Join(e.chatRecoveryRoot(), name)
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		r := &chatRecovery{dir: dir, state: chatRecoveryState{Version: 1, Project: e.name, Platform: p.Name(), SessionID: session.ID, SessionKey: "chat", InteractiveKey: "chat", AgentSessionID: "old", Reply: json.RawMessage(`"reply"`)}}
		if err := r.saveLocked(); err != nil {
			t.Fatal(err)
		}
		group = append(group, r)
	}
	e.reserveChatRecovery(group)
	for _, r := range group {
		if _, err := os.Stat(r.dir); !os.IsNotExist(err) {
			t.Fatalf("stale sensitive checkpoint retained: %v", err)
		}
	}
	if len(p.getSent()) != 0 {
		t.Fatal("notification sent before platform readiness")
	}
	e.startChatRecoveries(p)
	recoveryWait(t, "identity mismatch notice", func() bool { return len(p.getSent()) == 1 })
	if sends, recoveries, starts := e.agent.(*recoveryNativeAgent).ledger.counts(); sends != 0 || recoveries != 0 || starts != 0 {
		t.Fatal("native work replayed")
	}

}
