package main

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/timmyagentic/cc-connect-next/core"
)

func setAgentTurnCredential(t *testing.T, character string) string {
	t.Helper()
	sessionSecret := testAgentTurnToken(character)
	nonce := testAgentTurnToken("f")
	path := filepath.Join(t.TempDir(), "turn-nonce")
	if err := os.WriteFile(path, []byte(nonce+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(core.AgentTurnMarkerEnv, "1")
	t.Setenv(core.AgentSessionSecretEnv, sessionSecret)
	t.Setenv(core.AgentTurnNonceFileEnv, path)
	credential, err := core.BuildAgentTurnCredential(sessionSecret, nonce)
	if err != nil {
		t.Fatal(err)
	}
	return credential
}

func testAgentTurnToken(character string) string {
	return strings.Repeat(character, 64)
}

func TestRunAgentDeferredRestartSchedulesCurrentSession(t *testing.T) {
	t.Setenv("CC_PROJECT", "demo")
	t.Setenv("CC_SESSION_KEY", "feishu:d:chat:user:thread:omt-1")
	t.Setenv("CC_DATA_DIR", "/tmp/cc-data")
	credential := setAgentTurnCredential(t, "a")
	var gotPath, gotBody, gotSocket string
	factory := func(socketPath string) *http.Client {
		gotSocket = socketPath
		return &http.Client{Transport: capabilityRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			gotPath = req.URL.Path
			body, _ := io.ReadAll(req.Body)
			gotBody = string(body)
			return &http.Response{
				StatusCode: http.StatusAccepted,
				Body:       io.NopCloser(strings.NewReader(`{"status":"scheduled","session_key":"feishu:d:chat:user:thread:omt-1","platform":"feishu"}`)),
				Header:     make(http.Header),
			}, nil
		})}
	}
	var stdout, stderr bytes.Buffer
	handled, code := runAgentDeferredRestart(nil, &stdout, &stderr, factory)
	if !handled || code != 0 || stderr.Len() != 0 {
		t.Fatalf("handled=%v code=%d stdout=%q stderr=%q", handled, code, stdout.String(), stderr.String())
	}
	if gotSocket != "/tmp/cc-data/run/api.sock" || gotPath != "/restart/defer" {
		t.Fatalf("socket/path = %q %q", gotSocket, gotPath)
	}
	for _, want := range []string{`"credential":"` + credential + `"`} {
		if !strings.Contains(gotBody, want) {
			t.Errorf("body %q missing %q", gotBody, want)
		}
	}
	for _, forbidden := range []string{"user_id", "platform", "project", "session_key", "session_secret", "nonce"} {
		if strings.Contains(gotBody, forbidden) {
			t.Fatalf("client supplied untrusted routing field %q: %s", forbidden, gotBody)
		}
	}
	if !strings.Contains(stdout.String(), "after the current Agent turn") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunAgentDeferredRestartFailsClosed(t *testing.T) {
	t.Setenv("CC_PROJECT", "demo")
	t.Setenv("CC_SESSION_KEY", "feishu:d:chat:user")
	setAgentTurnCredential(t, "b")
	factory := func(string) *http.Client {
		return &http.Client{Transport: capabilityRoundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader("restart endpoint unavailable")),
				Header:     make(http.Header),
			}, nil
		})}
	}
	var stdout, stderr bytes.Buffer
	handled, code := runAgentDeferredRestart(nil, &stdout, &stderr, factory)
	if !handled || code == 0 {
		t.Fatalf("handled=%v code=%d", handled, code)
	}
	if !strings.Contains(stderr.String(), "/restart") || strings.Contains(stdout.String(), "restarted") {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestRunAgentDeferredRestartRejectsForceWithoutCallingRuntime(t *testing.T) {
	t.Setenv("CC_PROJECT", "demo")
	t.Setenv("CC_SESSION_KEY", "feishu:d:chat:user")
	setAgentTurnCredential(t, "c")
	called := false
	factory := func(string) *http.Client {
		called = true
		return &http.Client{}
	}
	var stdout, stderr bytes.Buffer
	handled, code := runAgentDeferredRestart([]string{"--force"}, &stdout, &stderr, factory)
	if !handled || code == 0 || called || !strings.Contains(stderr.String(), "--force") {
		t.Fatalf("handled=%v code=%d called=%v stderr=%q", handled, code, called, stderr.String())
	}
}

func TestRunAgentDeferredRestartLeavesExternalTerminalBehaviorUntouched(t *testing.T) {
	t.Setenv("CC_PROJECT", "demo")
	t.Setenv("CC_SESSION_KEY", "documented-terminal-session-override")
	t.Setenv("CC_AGENT_TYPE", "")
	t.Setenv("CC_PLATFORM_TYPES", "")
	t.Setenv(core.AgentTurnMarkerEnv, "")
	t.Setenv(core.AgentSessionSecretEnv, "")
	t.Setenv(core.AgentTurnNonceFileEnv, "")
	called := false
	factory := func(string) *http.Client {
		called = true
		return &http.Client{}
	}
	var stdout, stderr bytes.Buffer
	handled, code := runAgentDeferredRestart(nil, &stdout, &stderr, factory)
	if handled || code != 0 || called || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("handled=%v code=%d called=%v stdout=%q stderr=%q", handled, code, called, stdout.String(), stderr.String())
	}
}

func TestRunAgentDeferredRestartFailsClosedForLegacyAgentEnvironment(t *testing.T) {
	t.Setenv("CC_PROJECT", "demo")
	t.Setenv("CC_SESSION_KEY", "feishu:d:chat:user")
	t.Setenv("CC_AGENT_TYPE", "codex")
	t.Setenv("CC_PLATFORM_TYPES", "feishu")
	t.Setenv(core.AgentTurnMarkerEnv, "")
	t.Setenv(core.AgentSessionSecretEnv, "")
	t.Setenv(core.AgentTurnNonceFileEnv, "")
	called := false
	factory := func(string) *http.Client {
		called = true
		return &http.Client{}
	}
	var stdout, stderr bytes.Buffer
	handled, code := runAgentDeferredRestart(nil, &stdout, &stderr, factory)
	if !handled || code == 0 || called || !strings.Contains(stderr.String(), "/restart") {
		t.Fatalf("handled=%v code=%d called=%v stderr=%q", handled, code, called, stderr.String())
	}
}

func TestRunAgentDeferredRestartReportsConnectionFailure(t *testing.T) {
	t.Setenv("CC_PROJECT", "demo")
	t.Setenv("CC_SESSION_KEY", "feishu:d:chat:user")
	setAgentTurnCredential(t, "d")
	factory := func(string) *http.Client {
		return &http.Client{Transport: capabilityRoundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("socket unavailable")
		})}
	}
	var stdout, stderr bytes.Buffer
	handled, code := runAgentDeferredRestart(nil, &stdout, &stderr, factory)
	if !handled || code == 0 || !strings.Contains(stderr.String(), "/restart") {
		t.Fatalf("handled=%v code=%d stderr=%q", handled, code, stderr.String())
	}
}
