package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/timmyagentic/cc-connect-next/internal/appfeatures"
)

func prepareAgentFeedbackTurn(t *testing.T, userID, sessionKey string) (*Engine, *interactiveState) {
	t.Helper()
	engine, state := prepareDeferredRestartTurn(t, userID, sessionKey)
	engine.SetFeedbackConfig(true, "https://relay.example/v1/feedback")
	return engine, state
}

func captureAgentFeedbackRelay(engine *Engine, receipt appfeatures.FeedbackReceipt, relayErr error) (*int, *[]appfeatures.FeedbackReport) {
	calls := 0
	reports := make([]appfeatures.FeedbackReport, 0, 1)
	engine.feedbackSubmitFn = func(_ context.Context, draft appfeatures.FeedbackDraft, approved bool) (appfeatures.FeedbackReceipt, error) {
		calls++
		if !approved {
			panic("Agent feedback reached Relay without explicit approval")
		}
		reports = append(reports, draft.Report())
		return receipt, relayErr
	}
	return &calls, &reports
}

func TestAgentFeedbackRequiresPreviewAndSubmitsExactDraftOnce(t *testing.T) {
	engine, state := prepareAgentFeedbackTurn(t, "ou_user", "feishu:d:chat:ou_user")
	calls, reports := captureAgentFeedbackRelay(engine, appfeatures.FeedbackReceipt{
		ReferenceURL: "https://github.com/timmyagentic/cc-connect-next/issues/101",
		Deduplicated: true,
	}, nil)

	if _, err := engine.SubmitAgentFeedback(context.Background(), state.restartTurnToken, "not-a-preview-token"); !errors.Is(err, ErrAgentFeedbackApprovalInvalid) {
		t.Fatalf("submit without preview error = %v, want ErrAgentFeedbackApprovalInvalid", err)
	}
	if *calls != 0 {
		t.Fatalf("submit without approval made %d Relay requests", *calls)
	}

	engine.SetFeedbackCapabilityGaps([]string{"first.gap"})
	preview, err := engine.PreviewAgentFeedback(state.restartTurnToken, "Agent needs a supported feedback tool")
	if err != nil {
		t.Fatalf("PreviewAgentFeedback() error = %v", err)
	}
	if preview.Schema != AgentFeedbackAPISchema || preview.Status != AgentFeedbackStatusApprovalRequired || preview.ApprovalToken == "" {
		t.Fatalf("preview envelope = %#v", preview)
	}
	if preview.Draft.Description != "Agent needs a supported feedback tool" ||
		preview.Draft.Environment.Product != "cc-connect-next" ||
		len(preview.Draft.CapabilityGaps) != 1 ||
		preview.Draft.CapabilityGaps[0] != "first.gap" {
		t.Fatalf("preview Draft = %#v", preview.Draft)
	}
	if *calls != 0 {
		t.Fatalf("preview made %d Relay requests", *calls)
	}

	engine.SetFeedbackCapabilityGaps([]string{"later.gap"})
	engine.recordFeedbackError("feishu:d:chat:ou_user", "later error")
	receipt, err := engine.SubmitAgentFeedback(context.Background(), state.restartTurnToken, preview.ApprovalToken)
	if err != nil {
		t.Fatalf("SubmitAgentFeedback() error = %v", err)
	}
	if receipt.Schema != AgentFeedbackAPISchema || receipt.Status != AgentFeedbackStatusSubmitted ||
		receipt.ReferenceURL != "https://github.com/timmyagentic/cc-connect-next/issues/101" || !receipt.Deduplicated {
		t.Fatalf("receipt = %#v", receipt)
	}
	if *calls != 1 || len(*reports) != 1 {
		t.Fatalf("Relay calls/reports = %d/%d", *calls, len(*reports))
	}
	report := (*reports)[0]
	if report.Description != preview.Draft.Description || len(report.CapabilityGaps) != 1 ||
		report.CapabilityGaps[0] != "first.gap" || report.RecentError != nil {
		t.Fatalf("submitted Draft drifted after preview: %#v", report)
	}

	if _, err := engine.SubmitAgentFeedback(context.Background(), state.restartTurnToken, preview.ApprovalToken); !errors.Is(err, ErrAgentFeedbackApprovalInvalid) {
		t.Fatalf("replay error = %v, want ErrAgentFeedbackApprovalInvalid", err)
	}
	if *calls != 1 {
		t.Fatalf("approval-token replay made %d Relay requests", *calls)
	}
}

func TestAgentFeedbackApprovalTokenCannotBeConsumedAsChatCommand(t *testing.T) {
	const sessionKey = "feishu:d:chat:ou_user"
	engine, state := prepareAgentFeedbackTurn(t, "ou_user", sessionKey)
	calls, _ := captureAgentFeedbackRelay(engine, appfeatures.FeedbackReceipt{
		ReferenceURL: "https://github.com/timmyagentic/cc-connect-next/issues/101",
	}, nil)
	preview, err := engine.PreviewAgentFeedback(state.restartTurnToken, "do not synthesize a chat command")
	if err != nil {
		t.Fatal(err)
	}
	message := &Message{SessionKey: sessionKey, Platform: "feishu", UserID: "ou_user"}
	engine.cmdFeedback(engine.platforms[0], message, "confirm "+preview.ApprovalToken)
	if *calls != 0 {
		t.Fatalf("Agent-only token was consumed through chat, Relay calls = %d", *calls)
	}
	if _, err := engine.SubmitAgentFeedback(context.Background(), state.restartTurnToken, preview.ApprovalToken); err != nil {
		t.Fatalf("Agent token was not preserved for the supported submit path: %v", err)
	}
	if *calls != 1 {
		t.Fatalf("supported Agent submit made %d Relay requests", *calls)
	}
}

func TestAgentFeedbackApprovalTokenBindsSessionAndUserAcrossTurns(t *testing.T) {
	const firstSession = "feishu:d:first:ou_user"
	engine, firstState := prepareAgentFeedbackTurn(t, "ou_user", firstSession)
	calls, _ := captureAgentFeedbackRelay(engine, appfeatures.FeedbackReceipt{
		ReferenceURL: "https://github.com/timmyagentic/cc-connect-next/issues/101",
	}, nil)
	preview, err := engine.PreviewAgentFeedback(firstState.restartTurnToken, "bind this exact session")
	if err != nil {
		t.Fatal(err)
	}

	otherCredential, err := newAgentTurnToken()
	if err != nil {
		t.Fatal(err)
	}
	const otherSession = "feishu:d:other:ou_user"
	otherState := &interactiveState{
		agentSession:      &stubAgentSession{},
		platform:          engine.platforms[0],
		currentUserID:     "ou_user",
		currentSessionKey: otherSession,
		restartTurnToken:  otherCredential,
		presentationOpen:  true,
	}
	engine.interactiveStates[otherSession] = otherState
	other := engine.sessions.GetOrCreateActive(otherSession)
	if !other.TryLock() {
		t.Fatal("failed to mark second session busy")
	}
	t.Cleanup(other.Unlock)

	if _, err := engine.SubmitAgentFeedback(context.Background(), otherCredential, preview.ApprovalToken); !errors.Is(err, ErrAgentFeedbackApprovalInvalid) {
		t.Fatalf("cross-session submit error = %v, want ErrAgentFeedbackApprovalInvalid", err)
	}
	if *calls != 0 {
		t.Fatalf("cross-session token made %d Relay requests", *calls)
	}

	firstState.mu.Lock()
	rotatedCredential, err := newAgentTurnToken()
	if err != nil {
		firstState.mu.Unlock()
		t.Fatal(err)
	}
	firstState.restartTurnToken = rotatedCredential
	firstState.currentUserID = "ou_other"
	firstState.mu.Unlock()
	if _, err := engine.SubmitAgentFeedback(context.Background(), rotatedCredential, preview.ApprovalToken); !errors.Is(err, ErrAgentFeedbackApprovalInvalid) {
		t.Fatalf("cross-user submit error = %v, want ErrAgentFeedbackApprovalInvalid", err)
	}
	if *calls != 0 {
		t.Fatalf("cross-user token made %d Relay requests", *calls)
	}

	firstState.mu.Lock()
	nextTurnCredential, err := newAgentTurnToken()
	if err != nil {
		firstState.mu.Unlock()
		t.Fatal(err)
	}
	firstState.restartTurnToken = nextTurnCredential
	firstState.currentUserID = "ou_user"
	firstState.mu.Unlock()
	if _, err := engine.SubmitAgentFeedback(context.Background(), nextTurnCredential, preview.ApprovalToken); err != nil {
		t.Fatalf("same user/session approval in a later turn failed: %v", err)
	}
	if *calls != 1 {
		t.Fatalf("same user/session approval made %d Relay requests, want 1", *calls)
	}
}

func TestAgentFeedbackAPISchemaMismatchFailsBeforeRelay(t *testing.T) {
	engine, state := prepareAgentFeedbackTurn(t, "ou_user", "feishu:d:chat:ou_user")
	calls, _ := captureAgentFeedbackRelay(engine, appfeatures.FeedbackReceipt{
		ReferenceURL: "https://github.com/timmyagentic/cc-connect-next/issues/101",
	}, nil)
	api := &APIServer{engines: map[string]*Engine{"demo": engine}}

	previewBody, _ := json.Marshal(AgentFeedbackPreviewRequest{
		Schema: AgentFeedbackAPISchema, Credential: state.restartTurnToken, Description: "schema-safe feedback",
	})
	previewRecorder := httptest.NewRecorder()
	api.handleAgentFeedbackPreview(previewRecorder, httptest.NewRequest(http.MethodPost, "/feedback/preview", bytes.NewReader(previewBody)))
	if previewRecorder.Code != http.StatusOK {
		t.Fatalf("preview status = %d: %s", previewRecorder.Code, previewRecorder.Body.String())
	}
	var preview AgentFeedbackPreviewResponse
	if err := json.Unmarshal(previewRecorder.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}

	wrongSchemaBody, _ := json.Marshal(AgentFeedbackSubmitRequest{
		Schema: "cc-connect-next.agent-feedback/v2", Credential: state.restartTurnToken, ApprovalToken: preview.ApprovalToken,
	})
	wrongSchemaRecorder := httptest.NewRecorder()
	api.handleAgentFeedbackSubmit(wrongSchemaRecorder, httptest.NewRequest(http.MethodPost, "/feedback/submit", bytes.NewReader(wrongSchemaBody)))
	if wrongSchemaRecorder.Code != http.StatusConflict {
		t.Fatalf("schema mismatch status = %d, want 409: %s", wrongSchemaRecorder.Code, wrongSchemaRecorder.Body.String())
	}
	if *calls != 0 {
		t.Fatalf("schema mismatch made %d Relay requests", *calls)
	}

	submitBody, _ := json.Marshal(AgentFeedbackSubmitRequest{
		Schema: AgentFeedbackAPISchema, Credential: state.restartTurnToken, ApprovalToken: preview.ApprovalToken,
	})
	submitRecorder := httptest.NewRecorder()
	api.handleAgentFeedbackSubmit(submitRecorder, httptest.NewRequest(http.MethodPost, "/feedback/submit", bytes.NewReader(submitBody)))
	if submitRecorder.Code != http.StatusOK {
		t.Fatalf("submit status = %d: %s", submitRecorder.Code, submitRecorder.Body.String())
	}
	var receipt AgentFeedbackSubmitResponse
	if err := json.Unmarshal(submitRecorder.Body.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.Status != AgentFeedbackStatusSubmitted || !strings.Contains(receipt.ReferenceURL, "/issues/101") {
		t.Fatalf("structured receipt = %#v", receipt)
	}
	if *calls != 1 {
		t.Fatalf("approved submit made %d Relay requests", *calls)
	}
}

func TestAgentFeedbackRelaySchemaRejectionIsNotRetried(t *testing.T) {
	engine, state := prepareAgentFeedbackTurn(t, "ou_user", "feishu:d:chat:ou_user")
	calls, _ := captureAgentFeedbackRelay(engine, appfeatures.FeedbackReceipt{}, errors.New("relay returned HTTP 400"))
	preview, err := engine.PreviewAgentFeedback(state.restartTurnToken, "do not fall back to an old schema")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.SubmitAgentFeedback(context.Background(), state.restartTurnToken, preview.ApprovalToken); err == nil ||
		!strings.Contains(err.Error(), "HTTP 400") {
		t.Fatalf("relay schema error = %v", err)
	}
	if *calls != 1 {
		t.Fatalf("schema rejection made %d requests, want exactly one", *calls)
	}
	if _, err := engine.SubmitAgentFeedback(context.Background(), state.restartTurnToken, preview.ApprovalToken); !errors.Is(err, ErrAgentFeedbackApprovalInvalid) {
		t.Fatalf("consumed approval replay error = %v", err)
	}
	if *calls != 1 {
		t.Fatalf("schema rejection was retried, calls = %d", *calls)
	}
}
