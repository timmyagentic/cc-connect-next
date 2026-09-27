// Package appfeatures maps CC Connect Next host decisions onto the reusable
// Awesome Agent App Features contracts. Product UI and policy stay in core.
package appfeatures

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	featurefeedback "github.com/timmyagentic/awesome-agent-app-features/feedback"
	featurediagnostic "github.com/timmyagentic/awesome-agent-app-features/feedback/diagnostic"
	featurehttp "github.com/timmyagentic/awesome-agent-app-features/feedback/httpclient"
)

const ProductName = "cc-connect-next"

var (
	ccConnectIdentifierKVRE = regexp.MustCompile(`(?i)\b(app[_-]?id|session[_-]?key)\b(["']?\s*[=:]\s*)("(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'|[^\s,;]+)`)
	ccConnectKnownIDRE      = regexp.MustCompile(`\b(?:ou|oc|om|on|cli)_[0-9A-Za-z_-]{8,}\b`)
)

type FeedbackDraft = featurediagnostic.Draft
type FeedbackReport = featurediagnostic.Report
type FeedbackReceipt = featurehttp.Receipt
type FeedbackDiagnostic = featurediagnostic.Diagnostic
type FeedbackActivity = featurediagnostic.Activity
type FeedbackRuntime = featurediagnostic.Runtime
type FeedbackTransport = featurediagnostic.Transport

// A protocol rejection proves this request was not accepted. Timeouts and
// server failures do not: the remote issue may already exist.
func FeedbackDefinitelyRejected(err error) bool {
	var response *featurehttp.ResponseError
	if !errors.As(err, &response) {
		return false
	}
	switch response.StatusCode {
	case 400, 401, 403, 404, 409, 413, 422, 429:
		return true
	}
	return false
}

// FeedbackContext is the complete allowlist of host state that may enter a
// feedback draft. It permits an owned, bounded turn diagnostic, never an
// arbitrary transcript, environment, card payload, tool event or credential-
// bearing configuration map. The adjacent-text fields remain for legacy callers.
type FeedbackContext struct {
	Description               string
	PreviousUserMessage       string
	PreviousAssistantResponse string
	RecentError               string
	RecentErrorAt             time.Time
	CapabilityGaps            []string
	Version                   string
	Agent                     string
	Diagnostic                *FeedbackDiagnostic
}

// BuildFeedbackDraft maps CC Connect Next state into the provider-neutral v2
// report and applies product-specific identifier redaction in addition to the
// foundation's generic credential and path redaction.
func BuildFeedbackDraft(input FeedbackContext) (FeedbackDraft, error) {
	now := time.Now()
	var recentError *featurefeedback.RecentError
	// Use the Foundation freshness boundary for both the diagnostic and its
	// title summary, so stale/future errors cannot reappear in Description.
	if age := now.Sub(input.RecentErrorAt); strings.TrimSpace(input.RecentError) != "" &&
		!input.RecentErrorAt.IsZero() && age >= 0 && age <= featurefeedback.DefaultErrorMaxAge {
		recentError = &featurefeedback.RecentError{Text: input.RecentError, At: input.RecentErrorAt}
	}
	return (featurediagnostic.Builder{Now: func() time.Time { return now }, AdditionalRedact: redactCCConnectFeedback}).Build(featurediagnostic.Input{
		Description:    composeFeedbackDescription(input, recentError),
		RecentError:    recentError,
		CapabilityGaps: input.CapabilityGaps,
		Diagnostic:     input.Diagnostic,
		Environment: featurefeedback.Environment{
			Product: ProductName,
			Version: input.Version,
			OS:      runtime.GOOS,
			Arch:    runtime.GOARCH,
			Agent:   input.Agent,
		},
	})
}

// CaptureFeedbackDiagnostic applies the outbound allowlist before data enters
// the host's bounded local diagnostic store.
func CaptureFeedbackDiagnostic(value FeedbackDiagnostic) FeedbackDiagnostic {
	return (featurediagnostic.Builder{AdditionalRedact: redactCCConnectFeedback}).Capture(value)
}

// FeedbackDraftSnapshot is local host storage, not a wire submission. Restoring
// it still returns an unapproved Draft and cannot bypass explicit approval.
type FeedbackDraftSnapshot struct {
	PreparedAt time.Time
	Input      featurediagnostic.Input
}

func SnapshotFeedbackDraft(draft FeedbackDraft) FeedbackDraftSnapshot {
	r := draft.Report()
	return FeedbackDraftSnapshot{PreparedAt: draft.PreparedAt(), Input: featurediagnostic.Input{
		Description: r.Description, RecentError: r.RecentError, CapabilityGaps: r.CapabilityGaps,
		Environment: r.Environment, Diagnostic: r.Diagnostic, ReportID: r.ReportID,
	}}
}

func RestoreFeedbackDraft(value FeedbackDraftSnapshot) (FeedbackDraft, error) {
	return (featurediagnostic.Builder{Now: func() time.Time { return value.PreparedAt }, AdditionalRedact: redactCCConnectFeedback}).Build(value.Input)
}

func composeFeedbackDescription(input FeedbackContext, recentError *featurefeedback.RecentError) string {
	description := strings.TrimSpace(input.Description)
	if description == "" && input.Diagnostic != nil && input.Diagnostic.Error != "" {
		line, _, _ := strings.Cut(strings.TrimSpace(redactFeedbackContextText(input.Diagnostic.Error)), "\n")
		description = truncateFeedbackUTF8(line, 400)
	}
	if description == "" && recentError != nil {
		// Relay titles use Description's first line. Redact the complete error
		// before extracting/bounding that line; context is supporting evidence.
		firstLine, _, _ := strings.Cut(strings.TrimSpace(redactFeedbackContextText(recentError.Text)), "\n")
		description = truncateFeedbackUTF8(firstLine, 400)
	}
	// Redact before the host-specific truncation. Truncating a credential first
	// could cut away the syntax a redactor needs and expose a partial secret.
	previousUser := strings.TrimSpace(redactFeedbackContextText(input.PreviousUserMessage))
	previousAssistant := strings.TrimSpace(redactFeedbackContextText(input.PreviousAssistantResponse))
	if previousUser == "" && previousAssistant == "" {
		return description
	}
	const header = "Related diagnostic context (recent and subject to redaction)"
	remaining := featurefeedback.MaxDescriptionBytes - len(description) - len(header) - 4
	if remaining < 256 {
		return description
	}
	contextSections := make([]string, 0, 2)
	if previousUser != "" {
		section := "Previous user message:\n" + truncateFeedbackUTF8(previousUser, min(800, remaining/3))
		contextSections = append(contextSections, section)
		remaining -= len(section) + 2
	}
	const assistantLabel = "Previous assistant response:\n"
	if previousAssistant != "" && remaining > len(assistantLabel)+128 {
		contextSections = append(contextSections, assistantLabel+truncateFeedbackUTF8(previousAssistant, remaining-len(assistantLabel)))
	}
	if len(contextSections) == 0 {
		return description
	}
	context := header + "\n\n" + strings.Join(contextSections, "\n\n")
	if description == "" {
		return context
	}
	return description + "\n\n" + context
}

func redactFeedbackContextText(value string) string {
	return featurefeedback.Redact(redactCCConnectFeedback(featurefeedback.Redact(value)))
}

func truncateFeedbackUTF8(value string, maximum int) string {
	if maximum <= 0 || len(value) <= maximum {
		return value
	}
	suffix := "\n[truncated]"
	if maximum <= len(suffix) {
		return ""
	}
	value = value[:maximum-len(suffix)]
	for !utf8.ValidString(value) && value != "" {
		value = value[:len(value)-1]
	}
	return value + suffix
}

func redactCCConnectFeedback(text string) string {
	text = ccConnectIdentifierKVRE.ReplaceAllString(text, "$1$2[REDACTED-ID]")
	return ccConnectKnownIDRE.ReplaceAllString(text, "[REDACTED-ID]")
}

// RedactFeedbackText exposes the same combined redaction for host-owned
// capability descriptions and runtime errors that never become a Draft.
func RedactFeedbackText(text string) string {
	return featurefeedback.Redact(redactCCConnectFeedback(featurefeedback.Redact(text)))
}

// FeedbackRelay submits a host-built Draft. userApproved must reflect an
// explicit user action. Chat commands and card clicks are themselves approval;
// callers such as the local-Agent CLI may keep a separate preview/token step.
// False is always rejected before any request.
type FeedbackRelay struct {
	Endpoint   string
	HTTPClient *http.Client
}

func (relay FeedbackRelay) Submit(ctx context.Context, draft FeedbackDraft, userApproved bool) (FeedbackReceipt, error) {
	approved, err := draft.Approve(userApproved)
	if err != nil {
		return FeedbackReceipt{}, err
	}
	endpoint := relay.Endpoint
	// Existing configuration remains valid. Only the exact same-origin v1
	// path is upgraded; validation still rejects credentials/query/redirects.
	if parsed, parseErr := url.Parse(endpoint); parseErr == nil && parsed.EscapedPath() == featurehttp.EndpointPath {
		parsed.Path = featurehttp.DiagnosticEndpointPath
		parsed.RawPath = ""
		endpoint = parsed.String()
	}
	return (featurehttp.Client{
		Endpoint:   endpoint,
		HTTPClient: relay.HTTPClient,
		UserAgent:  "cc-connect-next-feedback/2",
	}).SubmitDiagnostic(ctx, approved)
}
