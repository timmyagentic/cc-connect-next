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
// feedback draft. It intentionally has no transcript, arbitrary environment,
// card payload, tool event, or credential-bearing configuration map.
type FeedbackContext struct {
	Description    string
	RecentError    string
	RecentErrorAt  time.Time
	CapabilityGaps []string
	Version        string
	Agent          string
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
		Description:    input.Description,
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

func redactCCConnectFeedback(text string) string {
	text = ccConnectIdentifierKVRE.ReplaceAllString(text, "$1$2[REDACTED-ID]")
	return ccConnectKnownIDRE.ReplaceAllString(text, "[REDACTED-ID]")
}

// RedactFeedbackText exposes the same combined redaction for host-owned
// capability descriptions and runtime errors that never become a Draft.
func RedactFeedbackText(text string) string {
	return featurefeedback.Redact(redactCCConnectFeedback(featurefeedback.Redact(text)))
}

// FeedbackRelay submits a previously rendered draft. userApproved must come
// from a separate explicit host action; false is rejected before any request.
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
