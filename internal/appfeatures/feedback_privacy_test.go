package appfeatures

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFeedbackDraftAndSubmissionRedactQuotedDiagnostics(t *testing.T) {
	const diagnostic = `{"OPENAI_API_KEY":"fixture-api-value","session_key":"fixture-session-value","app_id":"fixture-app-value","chat_id":"fixture-chat-value","password":"prefix\"fixture-password-tail"}`
	draft, err := BuildFeedbackDraft(FeedbackContext{
		Description: diagnostic, PreviousUserMessage: diagnostic,
		PreviousAssistantResponse: diagnostic, RecentError: diagnostic,
		RecentErrorAt: time.Now(), Version: `app_id="fixture-version-id"`,
		Agent: `session_key="fixture-agent-id"`,
	})
	if err != nil {
		t.Fatal(err)
	}
	report := draft.Report()
	preview := strings.Join([]string{report.Description, report.RecentError.Text, report.Environment.Version, report.Environment.Agent}, "\n")
	assertNoDiagnosticLeak := func(stage, value string) {
		t.Helper()
		for _, secret := range []string{"fixture-api-value", "fixture-session-value", "fixture-app-value", "fixture-chat-value", "fixture-password-tail", "fixture-version-id", "fixture-agent-id"} {
			if strings.Contains(value, secret) {
				t.Errorf("%s retained synthetic sensitive value %q", stage, secret)
			}
		}
	}
	assertNoDiagnosticLeak("draft", preview)
	var payload []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var readErr error
		payload, readErr = io.ReadAll(r.Body)
		if readErr != nil {
			t.Error(readErr)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"reference_url":"https://github.com/example/repo/issues/1"}`))
	}))
	defer server.Close()
	_, err = (FeedbackRelay{Endpoint: server.URL + "/v1/feedback", HTTPClient: server.Client()}).Submit(context.Background(), draft, true)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(payload) {
		t.Fatal("submitted payload is not JSON")
	}
	assertNoDiagnosticLeak("submitted payload", string(payload))
}

func TestFeedbackHostIdentifiersRedactEscapedQuotedValues(t *testing.T) {
	for _, value := range []string{
		`{"app_id":"prefix\"fixture-tail"}`,
		`{'session_key':'prefix\'fixture-tail'}`,
	} {
		if strings.Contains(RedactFeedbackText(value), "fixture-tail") {
			t.Errorf("quoted identifier tail was retained")
		}
	}
}
