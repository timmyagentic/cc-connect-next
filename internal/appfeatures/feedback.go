// Package appfeatures maps CC Connect Next host decisions onto the reusable
// Awesome Agent App Features contracts. Product UI and policy stay in core.
package appfeatures

import (
	"context"
	"net/http"
	"regexp"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	featurefeedback "github.com/timmyagentic/awesome-agent-app-features/feedback"
	featurehttp "github.com/timmyagentic/awesome-agent-app-features/feedback/httpclient"
)

const ProductName = "cc-connect-next"

var (
	ccConnectIdentifierKVRE = regexp.MustCompile(`(?i)\b(app[_-]?id|session[_-]?key)(\s*[=:]\s*)("[^"]*"|'[^']*'|[^\s,;]+)`)
	ccConnectKnownIDRE      = regexp.MustCompile(`\b(?:ou|oc|om|on|cli)_[0-9A-Za-z_-]{8,}\b`)
)

type FeedbackDraft = featurefeedback.Draft
type FeedbackReport = featurefeedback.Report
type FeedbackReceipt = featurehttp.Receipt

// FeedbackContext is the complete allowlist of host state that may enter a
// feedback draft. It permits only two explicitly bounded adjacent-message
// fields, never an arbitrary transcript, environment, card payload, tool event,
// or credential-bearing configuration map.
type FeedbackContext struct {
	Description               string
	PreviousUserMessage       string
	PreviousAssistantResponse string
	RecentError               string
	RecentErrorAt             time.Time
	CapabilityGaps            []string
	Version                   string
	Agent                     string
}

// BuildFeedbackDraft maps CC Connect Next state into the provider-neutral v1
// report and applies product-specific identifier redaction in addition to the
// foundation's generic credential and path redaction.
func BuildFeedbackDraft(input FeedbackContext) (FeedbackDraft, error) {
	var recentError *featurefeedback.RecentError
	if strings.TrimSpace(input.RecentError) != "" {
		recentError = &featurefeedback.RecentError{Text: input.RecentError, At: input.RecentErrorAt}
	}
	return (featurefeedback.Builder{AdditionalRedact: redactCCConnectFeedback}).Build(featurefeedback.Input{
		Description:    composeFeedbackDescription(input),
		RecentError:    recentError,
		CapabilityGaps: input.CapabilityGaps,
		Environment: featurefeedback.Environment{
			Product: ProductName,
			Version: input.Version,
			OS:      runtime.GOOS,
			Arch:    runtime.GOARCH,
			Agent:   input.Agent,
		},
	})
}

func composeFeedbackDescription(input FeedbackContext) string {
	description := strings.TrimSpace(input.Description)
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
	return (featurehttp.Client{
		Endpoint:   relay.Endpoint,
		HTTPClient: relay.HTTPClient,
		UserAgent:  "cc-connect-next-feedback/1",
	}).Submit(ctx, approved)
}
