package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/timmyagentic/cc-connect-next/core"
)

func TestRunAgentFeedbackPreviewUsesOnlyTurnCredentialAndDescription(t *testing.T) {
	t.Setenv("CC_DATA_DIR", "/tmp/cc-feedback-data")
	t.Setenv("CC_PROJECT", "exposed-project")
	t.Setenv("CC_SESSION_KEY", "feishu:d:exposed-session")
	credential := setAgentTurnCredential(t, "a")
	var gotPath, gotSocket string
	var gotBody []byte
	factory := func(socketPath string) *http.Client {
		gotSocket = socketPath
		return &http.Client{Transport: capabilityRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			gotPath = req.URL.Path
			gotBody, _ = io.ReadAll(req.Body)
			response, _ := json.Marshal(core.AgentFeedbackPreviewResponse{
				Schema: core.AgentFeedbackAPISchema, Status: core.AgentFeedbackStatusApprovalRequired,
				ApprovalToken: "opaque-preview-token", ExpiresAt: time.Now().Add(time.Minute),
				Draft: core.AgentFeedbackDraftPreview{
					Description: "summarize this problem",
					Environment: core.AgentFeedbackEnvironment{Product: "cc-connect-next"},
				},
			})
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(response)), Header: make(http.Header)}, nil
		})}
	}

	var stdout, stderr bytes.Buffer
	code := runAgentFeedback([]string{"preview", "--description", "summarize this problem"}, &stdout, &stderr, factory)
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if gotSocket != "/tmp/cc-feedback-data/run/api.sock" || gotPath != "/feedback/preview" {
		t.Fatalf("socket/path = %q %q", gotSocket, gotPath)
	}
	var request map[string]any
	if err := json.Unmarshal(gotBody, &request); err != nil {
		t.Fatal(err)
	}
	if request["schema"] != core.AgentFeedbackAPISchema || request["credential"] != credential ||
		request["description"] != "summarize this problem" {
		t.Fatalf("preview request = %#v", request)
	}
	for _, forbidden := range []string{"project", "session_key", "user_id", "platform", "session_secret", "nonce"} {
		if _, exists := request[forbidden]; exists {
			t.Fatalf("preview client supplied untrusted field %q: %s", forbidden, gotBody)
		}
	}
	var preview core.AgentFeedbackPreviewResponse
	if err := json.Unmarshal(stdout.Bytes(), &preview); err != nil {
		t.Fatalf("stdout is not structured JSON: %v: %s", err, stdout.String())
	}
	if preview.Status != core.AgentFeedbackStatusApprovalRequired || preview.ApprovalToken != "opaque-preview-token" {
		t.Fatalf("preview output = %#v", preview)
	}
}

func TestRunAgentFeedbackSubmitPrintsStructuredReceipt(t *testing.T) {
	credential := setAgentTurnCredential(t, "b")
	var request core.AgentFeedbackSubmitRequest
	factory := func(string) *http.Client {
		return &http.Client{Transport: capabilityRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/feedback/submit" {
				t.Fatalf("path = %q", req.URL.Path)
			}
			if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
				t.Fatal(err)
			}
			response, _ := json.Marshal(core.AgentFeedbackSubmitResponse{
				Schema: core.AgentFeedbackAPISchema, Status: core.AgentFeedbackStatusSubmitted,
				ReferenceURL: "https://github.com/timmyagentic/cc-connect-next/issues/101", Deduplicated: true,
			})
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(response)), Header: make(http.Header)}, nil
		})}
	}

	var stdout, stderr bytes.Buffer
	code := runAgentFeedback([]string{"submit", "--approval-token", "opaque-preview-token"}, &stdout, &stderr, factory)
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if request.Schema != core.AgentFeedbackAPISchema || request.Credential != credential || request.ApprovalToken != "opaque-preview-token" {
		t.Fatalf("submit request = %#v", request)
	}
	var receipt core.AgentFeedbackSubmitResponse
	if err := json.Unmarshal(stdout.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.Status != core.AgentFeedbackStatusSubmitted || !receipt.Deduplicated ||
		!strings.Contains(receipt.ReferenceURL, "/issues/101") {
		t.Fatalf("receipt = %#v", receipt)
	}
}

func TestRunAgentFeedbackFailsClosedWithoutActiveTurn(t *testing.T) {
	t.Setenv(core.AgentTurnMarkerEnv, "")
	t.Setenv(core.AgentSessionSecretEnv, "")
	t.Setenv(core.AgentTurnNonceFileEnv, "")
	called := false
	factory := func(string) *http.Client {
		called = true
		return &http.Client{}
	}
	var stdout, stderr bytes.Buffer
	code := runAgentFeedback([]string{"preview", "--description", "anything"}, &stdout, &stderr, factory)
	if code == 0 || called || stdout.Len() != 0 || !strings.Contains(stderr.String(), "active Agent turn") {
		t.Fatalf("code=%d called=%v stdout=%q stderr=%q", code, called, stdout.String(), stderr.String())
	}
}

func TestRunAgentFeedbackRejectsRuntimeSchemaDrift(t *testing.T) {
	setAgentTurnCredential(t, "c")
	factory := func(string) *http.Client {
		return &http.Client{Transport: capabilityRoundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"schema":"cc-connect-next.agent-feedback/v2","status":"submitted","reference_url":"https://example.test/issue/1"}`)),
				Header:     make(http.Header),
			}, nil
		})}
	}
	var stdout, stderr bytes.Buffer
	code := runAgentFeedback([]string{"submit", "--approval-token", "opaque-preview-token"}, &stdout, &stderr, factory)
	if code == 0 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "schema") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestAgentFeedbackSubcommandSuppressesBackgroundUpdateCheck(t *testing.T) {
	for _, args := range [][]string{
		{"feedback", "preview", "--description", "anything"},
		{"feedback", "submit", "--approval-token", "opaque"},
	} {
		if shouldCheckUpdateAsync(args) {
			t.Fatalf("feedback args unexpectedly allow the background update network check: %v", args)
		}
	}
	if !shouldCheckUpdateAsync(nil) || !shouldCheckUpdateAsync([]string{"capabilities"}) {
		t.Fatal("ordinary runtime/CLI behavior must keep the existing update check")
	}
}
