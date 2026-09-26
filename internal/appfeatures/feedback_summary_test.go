package appfeatures

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	featurefeedback "github.com/timmyagentic/awesome-agent-app-features/feedback"
)

func TestFeedbackSummaryUsesFreshProblemBeforeDiagnosticContext(t *testing.T) {
	for _, description := range []string{"", "The user's own issue summary"} {
		input := FeedbackContext{Description: description, RecentError: "Login refresh failed. Please sign in again.\nMore diagnostics", RecentErrorAt: time.Now(), PreviousUserMessage: "continue the task", Version: "v1.0.0"}
		draft, err := BuildFeedbackDraft(input)
		if err != nil {
			t.Fatal(err)
		}
		report := draft.Report()
		first, _, _ := strings.Cut(report.Description, "\n")
		want := description
		if want == "" {
			want = "Login refresh failed. Please sign in again."
		}
		if first != want {
			t.Fatalf("issue title source=%q, want problem summary %q", first, want)
		}
		if !strings.Contains(report.Description, "Related diagnostic context") || !strings.Contains(report.Description, input.PreviousUserMessage) || report.RecentError == nil {
			t.Fatalf("summary discarded supporting evidence: %+v", report)
		}
	}
}

func TestFeedbackSummaryDoesNotReviveStaleFutureOrUnknownError(t *testing.T) {
	for _, at := range []time.Time{time.Time{}, time.Now().Add(-time.Hour), time.Now().Add(time.Hour)} {
		draft, err := BuildFeedbackDraft(FeedbackContext{RecentError: "should-not-appear", RecentErrorAt: at, PreviousUserMessage: "relevant context", Version: "v1.0.0"})
		if err != nil {
			t.Fatal(err)
		}
		report := draft.Report()
		if report.RecentError != nil || strings.Contains(report.Description, "should-not-appear") {
			t.Fatalf("summary bypassed error freshness: %+v", report)
		}
	}
}

func TestFeedbackSummaryRedactsBeforeTruncation(t *testing.T) {
	credential := strings.Repeat("secret-fragment-", 100)
	draft, err := BuildFeedbackDraft(FeedbackContext{
		RecentError: "登录失效 token=" + credential + " /Users/private/project\nmore diagnostics", RecentErrorAt: time.Now(),
		PreviousUserMessage: strings.Repeat("用户上下文 ", 1000), Version: "v1.0.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	description := draft.Report().Description
	if !strings.HasPrefix(description, "登录失效") || !strings.Contains(description, "[REDACTED") || !utf8.ValidString(description) || len(description) > featurefeedback.MaxDescriptionBytes {
		t.Fatalf("summary lost redaction/bounds: %q", description)
	}
	if strings.Contains(description, "secret-fragment") || strings.Contains(description, "/Users/private") {
		t.Fatalf("summary leaked diagnostic data: %q", description)
	}
}
