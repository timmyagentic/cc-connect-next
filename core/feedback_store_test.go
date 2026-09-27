package core

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	featurefeedback "github.com/timmyagentic/awesome-agent-app-features/feedback"
	"github.com/timmyagentic/awesome-agent-app-features/feedback/diagnostic"
	"github.com/timmyagentic/cc-connect-next/internal/appfeatures"
)

func approvedFeedbackBytes(t *testing.T, draft appfeatures.FeedbackDraft) string {
	t.Helper()
	value, err := draft.Approve(true)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestFeedbackPendingSurvivesRestartWithoutChangingApprovedPayload(t *testing.T) {
	e, _ := newFeedbackTestEngine(t)
	dir := t.TempDir()
	e.SetDataDir(dir)
	preparedAt := time.Now().Add(-6 * time.Minute)
	draft, err := (diagnostic.Builder{Now: func() time.Time { return preparedAt }}).Build(diagnostic.Input{
		Description: "same approved issue", Environment: featurefeedback.Environment{Product: "test"},
		RecentError: &featurefeedback.RecentError{Text: "original failure", At: preparedAt.Add(-25 * time.Minute)},
	})
	if err != nil {
		t.Fatal(err)
	}
	msg := feedbackTestMsg()
	token, err := e.rememberPendingFeedback(msg.SessionKey, msg.UserID, draft)
	if err != nil {
		t.Fatal(err)
	}
	want := approvedFeedbackBytes(t, draft)
	if err := e.Stop(); err != nil {
		t.Fatal(err)
	}

	restarted, _ := newFeedbackTestEngine(t)
	restarted.SetDataDir(dir)
	t.Cleanup(func() { _ = restarted.Stop() })
	if _, ok := restarted.takePendingFeedback(msg.SessionKey, "another-user", token); ok {
		t.Fatal("another user consumed stored approval")
	}
	restored, ok := restarted.takePendingFeedback(msg.SessionKey, msg.UserID, token)
	if !ok {
		t.Fatal("pending grant was lost across restart")
	}
	if got := approvedFeedbackBytes(t, restored); got != want {
		t.Fatalf("approved payload changed after restart\nwant %s\ngot %s", want, got)
	}
	if _, ok := restarted.takePendingFeedback(msg.SessionKey, msg.UserID, token); ok {
		t.Fatal("stored token replayed")
	}
	data, err := os.ReadFile(restarted.feedbackStatePath())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), msg.SessionKey) || strings.Contains(string(data), msg.UserID) {
		t.Fatal("local state retained raw transport identity")
	}
	info, _ := os.Stat(restarted.feedbackStatePath())
	if info.Mode().Perm() != 0600 {
		t.Fatalf("state permissions = %v", info.Mode())
	}
}

func TestFeedbackUnknownSubmissionRetainsIdentityAcrossRestart(t *testing.T) {
	e, _ := newFeedbackTestEngine(t)
	dir := t.TempDir()
	e.SetDataDir(dir)
	msg := feedbackTestMsg()
	draft, err := e.buildFeedbackDraft(msg.SessionKey, msg.UserID, "repeat the same approved report", nil)
	if err != nil {
		t.Fatal(err)
	}
	want := approvedFeedbackBytes(t, draft)
	e.feedbackSubmitFn = func(context.Context, appfeatures.FeedbackDraft, bool) (appfeatures.FeedbackReceipt, error) {
		return appfeatures.FeedbackReceipt{}, errors.New("connection lost after remote acceptance")
	}
	if _, err := e.submitFeedbackDraft(context.Background(), draft, msg.SessionKey, msg.UserID); err == nil {
		t.Fatal("unknown outcome reported success")
	}
	if err := e.Stop(); err != nil {
		t.Fatal(err)
	}

	restarted, _ := newFeedbackTestEngine(t)
	restarted.SetDataDir(dir)
	t.Cleanup(func() { _ = restarted.Stop() })
	var calls atomic.Int32
	restarted.feedbackSubmitFn = func(_ context.Context, got appfeatures.FeedbackDraft, _ bool) (appfeatures.FeedbackReceipt, error) {
		calls.Add(1)
		if approvedFeedbackBytes(t, got) != want {
			t.Error("retry changed ID or payload")
		}
		return appfeatures.FeedbackReceipt{ReferenceURL: "https://github.com/owner/repo/issues/1"}, nil
	}
	if calls.Load() != 0 {
		t.Fatal("restart submitted without authorization")
	}
	retry, err := restarted.buildFeedbackDraft(msg.SessionKey, msg.UserID, "repeat the same approved report", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.submitFeedbackDraft(context.Background(), retry, msg.SessionKey, msg.UserID); err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.submitFeedbackDraft(context.Background(), retry, msg.SessionKey, msg.UserID); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("receipt replay made %d requests", calls.Load())
	}
}

func TestFeedbackConcurrentSubmissionSharesOneApprovedIntent(t *testing.T) {
	e, _ := newFeedbackTestEngine(t)
	msg := feedbackTestMsg()
	draft, err := e.buildFeedbackDraft(msg.SessionKey, msg.UserID, "one report", nil)
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	entered, release := make(chan struct{}), make(chan struct{})
	e.feedbackSubmitFn = func(context.Context, appfeatures.FeedbackDraft, bool) (appfeatures.FeedbackReceipt, error) {
		calls.Add(1)
		close(entered)
		<-release
		return appfeatures.FeedbackReceipt{ReferenceURL: "https://github.com/owner/repo/issues/1"}, nil
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := e.submitFeedbackDraft(context.Background(), draft, msg.SessionKey, msg.UserID); err != nil {
				t.Error(err)
			}
		}()
	}
	<-entered
	close(release)
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("concurrent approvals sent %d reports", calls.Load())
	}
}

func TestFeedbackDisabledStopDoesNotOverwriteStoredReports(t *testing.T) {
	e, _ := newFeedbackTestEngine(t)
	dir := t.TempDir()
	e.SetDataDir(dir)
	draft, _ := e.buildFeedbackDraft("test:owner", "owner", "keep my report", nil)
	_, _ = e.rememberPendingFeedback("test:owner", "owner", draft)
	_ = e.Stop()
	want, err := os.ReadFile(e.feedbackStatePath())
	if err != nil {
		t.Fatal(err)
	}
	disabled := NewEngine("test", &stubAgent{}, nil, "", LangEnglish)
	disabled.SetFeedbackConfig(false, "")
	disabled.SetDataDir(dir)
	_ = disabled.Stop()
	got, _ := os.ReadFile(e.feedbackStatePath())
	if string(got) != string(want) {
		t.Fatal("disabled feedback erased retained reports")
	}
}

func TestFeedbackRestartFreezesLongActiveTurnBeforeRetention(t *testing.T) {
	e, p := newFeedbackTestEngine(t)
	dir := t.TempDir()
	e.SetDataDir(dir)
	e.sessions = NewSessionManager(filepath.Join(dir, "sessions.json"))
	msg := feedbackTestMsg()
	msg.Content = "a long running request still needs its context"
	turn := e.beginFeedbackTurn(p, msg, e.sessions.GetOrCreateActive(msg.SessionKey), e.agent)
	e.sessions.Save()
	turn.mu.Lock()
	turn.at = time.Now().Add(-feedbackRetain - time.Hour)
	turn.diagnostic.StartedAt = turn.at
	turn.mu.Unlock()
	if err := e.Stop(); err != nil {
		t.Fatal(err)
	}
	restarted, _ := newFeedbackTestEngine(t)
	restarted.sessions = NewSessionManager(filepath.Join(dir, "sessions.json"))
	restarted.SetDataDir(dir)
	t.Cleanup(func() { _ = restarted.Stop() })
	draft, err := restarted.buildFeedbackDraft(msg.SessionKey, msg.UserID, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	d := draft.Report().Diagnostic
	if d == nil || d.Request != msg.Content || d.Phase != "interrupted" || d.ErrorCode != "host_restart" {
		t.Fatalf("restart lost the active turn: %#v", d)
	}
	if time.Since(d.StartedAt) < feedbackRetain || time.Since(d.OccurredAt) > time.Minute || !strings.Contains(d.Error, "unknown") {
		t.Fatalf("restart invented the incident time or outcome: %#v", d)
	}
}
