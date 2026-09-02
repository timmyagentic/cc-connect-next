package appfeatures

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	featurefeedback "github.com/timmyagentic/awesome-agent-app-features/feedback"
)

func TestBuildFeedbackDraftUsesFoundationStructuredRedactedReport(t *testing.T) {
	draft, err := BuildFeedbackDraft(FeedbackContext{
		Description:    "fails for user@example.com app_secret=very-secret ou_1234567890 /Users/timmy/private/config.toml",
		RecentError:    "session_key=feishu:oc_1234567890:ou_1234567890 app_id=cli_1234567890",
		RecentErrorAt:  time.Now(),
		CapabilityGaps: []string{"display.sparkles", "display.sparkles"},
		Version:        "v0.2.1",
		Agent:          "codex",
	})
	if err != nil {
		t.Fatalf("BuildFeedbackDraft: %v", err)
	}

	report := draft.Report()
	if report.Environment.Product != ProductName || report.Environment.Version != "v0.2.1" || report.Environment.Agent != "codex" {
		t.Fatalf("unexpected environment mapping: %#v", report.Environment)
	}
	if !reflect.DeepEqual(report.CapabilityGaps, []string{"display.sparkles"}) {
		t.Fatalf("capability gaps = %v", report.CapabilityGaps)
	}
	preview := strings.Join([]string{
		report.Description,
		report.RecentError.Text,
		strings.Join(report.CapabilityGaps, "\n"),
		report.Environment.Product,
		report.Environment.Version,
		report.Environment.OS,
		report.Environment.Arch,
		report.Environment.Agent,
	}, "\n")
	for _, leaked := range []string{"very-secret", "user@example.com", "/Users/timmy", "oc_1234567890", "ou_1234567890", "cli_1234567890"} {
		if strings.Contains(preview, leaked) {
			t.Errorf("feedback preview leaked %q: %s", leaked, preview)
		}
	}
	if _, err := json.Marshal(report); !errors.Is(err, featurefeedback.ErrApprovalRequired) {
		t.Fatalf("Report.MarshalJSON error = %v", err)
	}
}

func TestFeedbackRelayRequiresApprovalBeforeNetwork(t *testing.T) {
	var requests atomic.Int32
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		if request.URL.Path != "/v1/feedback" {
			t.Errorf("path = %q", request.URL.Path)
		}
		if err := json.NewDecoder(request.Body).Decode(&received); err != nil {
			t.Errorf("decode request: %v", err)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"reference_url":"https://github.com/timmyagentic/cc-connect-next/issues/999"}`))
	}))
	defer server.Close()

	draft, err := BuildFeedbackDraft(FeedbackContext{Description: "one real problem", Version: "v0.2.1", Agent: "codex"})
	if err != nil {
		t.Fatalf("BuildFeedbackDraft: %v", err)
	}
	relay := FeedbackRelay{Endpoint: server.URL + "/v1/feedback", HTTPClient: server.Client()}
	if _, err := relay.Submit(context.Background(), draft, false); !errors.Is(err, featurefeedback.ErrApprovalRequired) {
		t.Fatalf("unapproved submit error = %v", err)
	}
	if requests.Load() != 0 {
		t.Fatalf("unapproved draft made %d request(s)", requests.Load())
	}

	receipt, err := relay.Submit(context.Background(), draft, true)
	if err != nil {
		t.Fatalf("approved Submit: %v", err)
	}
	if requests.Load() != 1 || received["user_approved"] != true || received["schema"] != float64(1) {
		t.Fatalf("approved payload mismatch: requests=%d payload=%#v", requests.Load(), received)
	}
	for _, forbidden := range []string{"title", "body", "install_id"} {
		if _, exists := received[forbidden]; exists {
			t.Errorf("foundation payload contains legacy field %q", forbidden)
		}
	}
	if !strings.HasSuffix(receipt.ReferenceURL, "/999") {
		t.Errorf("receipt = %#v", receipt)
	}
}

func TestBuildFeedbackDraftDropsStaleError(t *testing.T) {
	draft, err := BuildFeedbackDraft(FeedbackContext{
		Description:   "unrelated request",
		RecentError:   "ancient failure",
		RecentErrorAt: time.Now().Add(-time.Hour),
		Version:       "v0.2.1",
		Agent:         "codex",
	})
	if err != nil {
		t.Fatalf("BuildFeedbackDraft: %v", err)
	}
	if draft.Report().RecentError != nil {
		t.Fatalf("stale error was attached: %+v", draft.Report().RecentError)
	}
}

func TestBuildFeedbackDraftBoundsAndRedactsTypedAdjacentContext(t *testing.T) {
	draft, err := BuildFeedbackDraft(FeedbackContext{
		Description:               "channel copy is wrong",
		PreviousUserMessage:       "token=never-send-this " + strings.Repeat("用户上下文 ", 300),
		PreviousAssistantResponse: "/Users/private/project " + strings.Repeat("diagnosis ", 500),
		Version:                   "v0.3.0-beta.2",
		Agent:                     "codex",
	})
	if err != nil {
		t.Fatal(err)
	}
	description := draft.Report().Description
	if len(description) > featurefeedback.MaxDescriptionBytes {
		t.Fatalf("description size = %d", len(description))
	}
	for _, want := range []string{"Related diagnostic context", "Previous user message", "Previous assistant response", "[REDACTED"} {
		if !strings.Contains(description, want) {
			t.Fatalf("description missing %q: %s", want, description)
		}
	}
	for _, leaked := range []string{"never-send-this", "/Users/private"} {
		if strings.Contains(description, leaked) {
			t.Fatalf("description leaked %q", leaked)
		}
	}
}

func TestApprovedFeedbackV1FixtureMatchesFoundationEncoder(t *testing.T) {
	draft, err := (featurefeedback.Builder{Now: func() time.Time {
		return time.Date(2026, 8, 30, 6, 1, 0, 0, time.UTC)
	}}).Build(featurefeedback.Input{
		Description: "Cross-language contract proof",
		RecentError: &featurefeedback.RecentError{
			Text: "redacted failure",
			At:   time.Date(2026, 8, 30, 6, 0, 0, 0, time.UTC),
		},
		CapabilityGaps: []string{"feature.example"},
		Environment: featurefeedback.Environment{
			Product: "cc-connect-next",
			Version: "v0.2.1",
			OS:      "darwin",
			Arch:    "arm64",
			Agent:   "codex",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	approved, err := draft.Approve(true)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(approved)
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile("testdata/approved-feedback-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var actual, expected any
	if err := json.Unmarshal(encoded, &actual); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(fixture, &expected); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("Foundation payload drifted:\nactual=%s\nfixture=%s", encoded, fixture)
	}
}
