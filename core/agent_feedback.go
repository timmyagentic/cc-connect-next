package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/timmyagentic/cc-connect-next/internal/appfeatures"
)

const (
	// AgentFeedbackAPISchema versions the local CLI-to-daemon contract. A
	// mismatch is rejected before the pending Draft is consumed or Relay I/O.
	AgentFeedbackAPISchema = "cc-connect-next.agent-feedback/v1"

	AgentFeedbackStatusApprovalRequired = "approval_required"
	AgentFeedbackStatusSubmitted        = "submitted"
)

var (
	ErrAgentFeedbackCredentialInvalid = errors.New("invalid Agent turn credential")
	ErrAgentFeedbackNoActiveTurn      = errors.New("no active Agent turn for feedback")
	ErrAgentFeedbackDisabled          = errors.New("feedback is disabled")
	ErrAgentFeedbackDescription       = errors.New("feedback description is required")
	ErrAgentFeedbackApprovalInvalid   = errors.New("feedback approval token is invalid")
	ErrAgentFeedbackSchemaMismatch    = errors.New("Agent feedback API schema mismatch")
)

type AgentFeedbackPreviewRequest struct {
	Schema      string `json:"schema"`
	Credential  string `json:"credential"`
	Description string `json:"description"`
}

type AgentFeedbackSubmitRequest struct {
	Schema        string `json:"schema"`
	Credential    string `json:"credential"`
	ApprovalToken string `json:"approval_token"`
}

type AgentFeedbackEnvironment struct {
	Product string `json:"product"`
	Version string `json:"version,omitempty"`
	OS      string `json:"os,omitempty"`
	Arch    string `json:"arch,omitempty"`
	Agent   string `json:"agent,omitempty"`
}

type AgentFeedbackRecentError struct {
	Text       string    `json:"text"`
	OccurredAt time.Time `json:"occurred_at"`
}

// AgentFeedbackDraftPreview is a JSON-safe projection of Foundation Report.
// Foundation deliberately refuses to marshal an unapproved Report, so the
// host copies the exact allowlisted fields for review without creating a wire
// submission value.
type AgentFeedbackDraftPreview struct {
	Description    string                    `json:"description,omitempty"`
	RecentError    *AgentFeedbackRecentError `json:"recent_error,omitempty"`
	CapabilityGaps []string                  `json:"capability_gaps,omitempty"`
	Environment    AgentFeedbackEnvironment  `json:"environment"`
}

type AgentFeedbackPreviewResponse struct {
	Schema        string                    `json:"schema"`
	Status        string                    `json:"status"`
	ApprovalToken string                    `json:"approval_token"`
	ExpiresAt     time.Time                 `json:"expires_at"`
	Draft         AgentFeedbackDraftPreview `json:"draft"`
}

type AgentFeedbackSubmitResponse struct {
	Schema       string `json:"schema"`
	Status       string `json:"status"`
	ReferenceURL string `json:"reference_url"`
	Deduplicated bool   `json:"deduplicated"`
}

func agentFeedbackDraftPreview(report appfeatures.FeedbackReport) AgentFeedbackDraftPreview {
	preview := AgentFeedbackDraftPreview{
		Description:    report.Description,
		CapabilityGaps: append([]string(nil), report.CapabilityGaps...),
		Environment: AgentFeedbackEnvironment{
			Product: report.Environment.Product,
			Version: report.Environment.Version,
			OS:      report.Environment.OS,
			Arch:    report.Environment.Arch,
			Agent:   report.Environment.Agent,
		},
	}
	if report.RecentError != nil {
		preview.RecentError = &AgentFeedbackRecentError{Text: report.RecentError.Text, OccurredAt: report.RecentError.At}
	}
	if report.Diagnostic != nil {
		data, _ := json.MarshalIndent(report.Diagnostic, "", "  ")
		preview.Description += "\n\nCaptured diagnostic snapshot:\n" + string(data)
	}
	preview.Description += "\n\nReport ID: " + report.ReportID + "\nDiagnostic protocol: 2"
	return preview
}

func (e *Engine) agentFeedbackTurnIdentity(credential string) (sessionKey, userID string, err error) {
	credential = strings.TrimSpace(credential)
	state, sessionKey := e.agentTurnStateForCredential(credential)
	if state == nil {
		return "", "", ErrAgentFeedbackCredentialInvalid
	}

	_, sessions := e.sessionContextForKey(sessionKey)
	if sessions == nil {
		return "", "", ErrAgentFeedbackNoActiveTurn
	}
	session := sessions.GetOrCreateActive(sessionKey)

	state.steerMu.Lock()
	defer state.steerMu.Unlock()
	state.mu.Lock()
	defer state.mu.Unlock()
	if !sameAgentTurnToken(credential, state.restartTurnToken) || state.currentSessionKey != sessionKey {
		return "", "", ErrAgentFeedbackCredentialInvalid
	}
	if !state.presentationOpen || state.stopped || state.agentSession == nil || !state.agentSession.Alive() || !session.Busy() {
		return "", "", ErrAgentFeedbackNoActiveTurn
	}
	userID = strings.TrimSpace(state.currentUserID)
	if userID == "" {
		return "", "", ErrAgentFeedbackNoActiveTurn
	}
	return sessionKey, userID, nil
}

// PreviewAgentFeedback creates the same immutable, redacted Foundation Draft
// used by /feedback. It performs no Relay request and returns a one-time token
// bound to the trusted initiating user/session and exact Draft. A later turn
// for that same user/session may submit it after the user reviews the preview.
func (e *Engine) PreviewAgentFeedback(credential, description string) (AgentFeedbackPreviewResponse, error) {
	description = strings.TrimSpace(description)
	if description == "" {
		return AgentFeedbackPreviewResponse{}, ErrAgentFeedbackDescription
	}
	sessionKey, userID, err := e.agentFeedbackTurnIdentity(credential)
	if err != nil {
		return AgentFeedbackPreviewResponse{}, err
	}
	if !e.feedbackActive() {
		return AgentFeedbackPreviewResponse{}, ErrAgentFeedbackDisabled
	}
	draft, err := e.buildFeedbackDraft(sessionKey, userID, description, nil)
	if err != nil {
		return AgentFeedbackPreviewResponse{}, err
	}
	token, expiresAt, err := e.rememberPendingFeedbackForCaller(sessionKey, userID, true, draft)
	if err != nil {
		return AgentFeedbackPreviewResponse{}, err
	}
	return AgentFeedbackPreviewResponse{
		Schema:        AgentFeedbackAPISchema,
		Status:        AgentFeedbackStatusApprovalRequired,
		ApprovalToken: token,
		ExpiresAt:     expiresAt,
		Draft:         agentFeedbackDraftPreview(draft.Report()),
	}, nil
}

func (e *Engine) takeAgentPendingFeedback(sessionKey, userID, token string) (appfeatures.FeedbackDraft, bool) {
	e.feedbackMu.Lock()
	defer e.feedbackMu.Unlock()
	e.prunePendingFeedbackLocked(time.Now())
	pending, exists := e.feedbackPending[strings.TrimSpace(token)]
	if !exists ||
		!pending.AgentOnly ||
		pending.SessionKey != e.feedbackBinding(sessionKey, "") ||
		pending.UserID != e.feedbackPendingUser(userID) {
		return appfeatures.FeedbackDraft{}, false
	}
	if err := e.rememberApprovedFeedbackLocked(sessionKey, userID, pending.Draft); err != nil {
		return appfeatures.FeedbackDraft{}, false
	}
	e.deletePendingFeedbackLocked(strings.TrimSpace(token))
	return pending.Draft, true
}

func (e *Engine) submitFeedbackDraft(ctx context.Context, draft appfeatures.FeedbackDraft, sessionKey, userID string) (appfeatures.FeedbackReceipt, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, feedbackSubmitTimeout)
	defer cancel()
	e.feedbackMu.Lock()
	if err := e.rememberApprovedFeedbackLocked(sessionKey, userID, draft); err != nil {
		e.feedbackMu.Unlock()
		return appfeatures.FeedbackReceipt{}, err
	}
	record := e.feedbackSubmissions[draft.Report().ReportID]
	if record.Receipt != nil {
		receipt := *record.Receipt
		e.feedbackMu.Unlock()
		return receipt, nil
	}
	if record.inFlight != nil {
		finished := record.inFlight
		e.feedbackMu.Unlock()
		select {
		case <-finished:
		case <-ctx.Done():
			return appfeatures.FeedbackReceipt{}, ctx.Err()
		}
		e.feedbackMu.Lock()
		defer e.feedbackMu.Unlock()
		if record.Receipt != nil {
			return *record.Receipt, nil
		}
		return appfeatures.FeedbackReceipt{}, fmt.Errorf("feedback submission did not complete")
	}
	record.inFlight = make(chan struct{})
	record.State = "dispatching"
	endpoint, submit := e.feedbackEndpoint, e.feedbackSubmitFn
	e.feedbackMu.Unlock()
	var receipt appfeatures.FeedbackReceipt
	err := e.saveFeedbackState()
	dispatched := false
	if err == nil {
		if submit == nil {
			relay := appfeatures.FeedbackRelay{Endpoint: endpoint}
			submit = relay.Submit
		}
		dispatched = true
		receipt, err = submit(ctx, draft, true)
	} else {
		err = fmt.Errorf("feedback approved report could not be saved")
	}
	e.feedbackMu.Lock()
	if err == nil {
		record.State = "submitted"
		copy := receipt
		record.Receipt = &copy
	} else {
		record.State = "outcome_unknown"
		if !dispatched || appfeatures.FeedbackDefinitelyRejected(err) {
			record.State = "definite_failed"
		}
	}
	close(record.inFlight)
	record.inFlight = nil
	e.scheduleFeedbackSaveLocked()
	e.feedbackMu.Unlock()
	if err != nil {
		slog.Warn("feedback: submission failed", "error", redactFeedbackText(err.Error()))
		return appfeatures.FeedbackReceipt{}, err
	}
	// A persisted dispatch intent plus the Relay receipt is sufficient to
	// recover even if the final local receipt write fails.
	if err := e.saveFeedbackState(); err != nil {
		slog.Warn("feedback: receipt could not be saved locally")
	}
	slog.Info("feedback: submitted", "reference_url", receipt.ReferenceURL, "deduplicated", receipt.Deduplicated)
	return receipt, nil
}

// SubmitAgentFeedback exchanges the one-use Agent approval for a stored exact
// report. The token cannot replay; bounded internal retries use that report ID
// and the Relay's durable receipt without a protocol fallback.
func (e *Engine) SubmitAgentFeedback(ctx context.Context, credential, approvalToken string) (AgentFeedbackSubmitResponse, error) {
	sessionKey, userID, err := e.agentFeedbackTurnIdentity(credential)
	if err != nil {
		return AgentFeedbackSubmitResponse{}, err
	}
	if !e.feedbackActive() {
		return AgentFeedbackSubmitResponse{}, ErrAgentFeedbackDisabled
	}
	draft, ok := e.takeAgentPendingFeedback(sessionKey, userID, approvalToken)
	if !ok {
		return AgentFeedbackSubmitResponse{}, ErrAgentFeedbackApprovalInvalid
	}
	receipt, err := e.submitFeedbackDraft(ctx, draft, sessionKey, userID)
	if err != nil {
		return AgentFeedbackSubmitResponse{}, err
	}
	return AgentFeedbackSubmitResponse{
		Schema:       AgentFeedbackAPISchema,
		Status:       AgentFeedbackStatusSubmitted,
		ReferenceURL: receipt.ReferenceURL,
		Deduplicated: receipt.Deduplicated,
	}, nil
}
