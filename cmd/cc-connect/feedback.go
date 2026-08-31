package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/timmyagentic/cc-connect-next/core"
)

const agentFeedbackCLIResponseLimit int64 = 256 << 10

func runFeedback(args []string) {
	if code := runAgentFeedback(args, os.Stdout, os.Stderr, newLocalAPIClient); code != 0 {
		os.Exit(code)
	}
}

func printAgentFeedbackUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, `Usage:
  cc-connect-next feedback preview --description <text>
  cc-connect-next feedback submit --approval-token <opaque-token>

This local-agent tool is deliberately two-step. preview returns the complete
redacted Foundation Draft and performs no network request. Run submit only
when the current user explicitly authorized that exact preview.`)
}

func parseAgentFeedbackAction(args []string, stdout, stderr io.Writer) (action, description, approvalToken string, code int) {
	if len(args) == 0 {
		printAgentFeedbackUsage(stderr)
		return "", "", "", 2
	}
	action = strings.ToLower(strings.TrimSpace(args[0]))
	switch action {
	case "preview":
		flags := flag.NewFlagSet("feedback preview", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		flags.StringVar(&description, "description", "", "problem or missing capability")
		if err := flags.Parse(args[1:]); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				printAgentFeedbackUsage(stdout)
				return "", "", "", 0
			}
			_, _ = fmt.Fprintf(stderr, "feedback preview: %v\n", err)
			return "", "", "", 2
		}
		if flags.NArg() != 0 {
			_, _ = fmt.Fprintln(stderr, "feedback preview: unexpected positional arguments")
			return "", "", "", 2
		}
		description = strings.TrimSpace(description)
		if description == "" {
			_, _ = fmt.Fprintln(stderr, "feedback preview: --description is required")
			return "", "", "", 2
		}
	case "submit":
		flags := flag.NewFlagSet("feedback submit", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		flags.StringVar(&approvalToken, "approval-token", "", "one-time token returned by preview")
		var tokenAlias string
		flags.StringVar(&tokenAlias, "token", "", "alias for --approval-token")
		if err := flags.Parse(args[1:]); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				printAgentFeedbackUsage(stdout)
				return "", "", "", 0
			}
			_, _ = fmt.Fprintf(stderr, "feedback submit: %v\n", err)
			return "", "", "", 2
		}
		if flags.NArg() != 0 {
			_, _ = fmt.Fprintln(stderr, "feedback submit: unexpected positional arguments")
			return "", "", "", 2
		}
		if strings.TrimSpace(approvalToken) != "" && strings.TrimSpace(tokenAlias) != "" {
			_, _ = fmt.Fprintln(stderr, "feedback submit: use only one of --approval-token or --token")
			return "", "", "", 2
		}
		if approvalToken == "" {
			approvalToken = tokenAlias
		}
		approvalToken = strings.TrimSpace(approvalToken)
		if approvalToken == "" {
			_, _ = fmt.Fprintln(stderr, "feedback submit: --approval-token is required; run feedback preview first")
			return "", "", "", 2
		}
	case "help", "-h", "--help":
		printAgentFeedbackUsage(stdout)
		return "", "", "", 0
	default:
		_, _ = fmt.Fprintf(stderr, "feedback: unknown action %q\n", args[0])
		printAgentFeedbackUsage(stderr)
		return "", "", "", 2
	}
	return action, description, approvalToken, -1
}

func currentAgentTurnCredential() (string, error) {
	if strings.TrimSpace(os.Getenv(core.AgentTurnMarkerEnv)) != "1" {
		return "", fmt.Errorf("an active Agent turn is required")
	}
	sessionSecret := strings.TrimSpace(os.Getenv(core.AgentSessionSecretEnv))
	noncePath := strings.TrimSpace(os.Getenv(core.AgentTurnNonceFileEnv))
	if sessionSecret == "" || noncePath == "" {
		return "", fmt.Errorf("the active Agent turn does not provide credentials")
	}
	file, err := os.Open(noncePath)
	if err != nil {
		return "", fmt.Errorf("the active Agent turn nonce is unavailable")
	}
	data, readErr := io.ReadAll(io.LimitReader(file, 4<<10))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil || len(data) >= 4<<10 {
		return "", fmt.Errorf("the active Agent turn credential is invalid")
	}
	credential, err := core.BuildAgentTurnCredential(sessionSecret, strings.TrimSpace(string(data)))
	if err != nil {
		return "", fmt.Errorf("the active Agent turn credential is invalid")
	}
	return credential, nil
}

func callAgentFeedbackAPI(ctx context.Context, client *http.Client, path string, payload, response any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://unix"+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	result, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("connect to running daemon: %w", err)
	}
	defer func() { _ = result.Body.Close() }()
	responseBody, err := io.ReadAll(io.LimitReader(result.Body, agentFeedbackCLIResponseLimit+1))
	if err != nil {
		return fmt.Errorf("read daemon response: %w", err)
	}
	if len(responseBody) > int(agentFeedbackCLIResponseLimit) {
		return fmt.Errorf("daemon response is too large")
	}
	if result.StatusCode != http.StatusOK {
		detail := strings.TrimSpace(string(responseBody))
		if detail == "" {
			detail = http.StatusText(result.StatusCode)
		}
		return fmt.Errorf("daemon returned HTTP %d: %s", result.StatusCode, detail)
	}
	if err := json.Unmarshal(responseBody, response); err != nil {
		return fmt.Errorf("decode daemon response: %w", err)
	}
	return nil
}

func writeAgentFeedbackJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

func runAgentFeedback(args []string, stdout, stderr io.Writer, clientFactory func(string) *http.Client) int {
	action, description, approvalToken, parseCode := parseAgentFeedbackAction(args, stdout, stderr)
	if parseCode >= 0 {
		return parseCode
	}
	credential, err := currentAgentTurnCredential()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "feedback %s failed closed: %v\n", action, err)
		return 1
	}
	client := clientFactory(resolveSocketPath(""))
	defer client.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	switch action {
	case "preview":
		request := core.AgentFeedbackPreviewRequest{
			Schema: core.AgentFeedbackAPISchema, Credential: credential, Description: description,
		}
		var response core.AgentFeedbackPreviewResponse
		if err := callAgentFeedbackAPI(ctx, client, "/feedback/preview", request, &response); err != nil {
			_, _ = fmt.Fprintf(stderr, "feedback preview failed closed: %v\n", err)
			return 1
		}
		if response.Schema != core.AgentFeedbackAPISchema || response.Status != core.AgentFeedbackStatusApprovalRequired ||
			strings.TrimSpace(response.ApprovalToken) == "" {
			_, _ = fmt.Fprintln(stderr, "feedback preview failed closed: runtime schema or status mismatch")
			return 1
		}
		if err := writeAgentFeedbackJSON(stdout, response); err != nil {
			_, _ = fmt.Fprintf(stderr, "feedback preview output failed: %v\n", err)
			return 1
		}
	case "submit":
		request := core.AgentFeedbackSubmitRequest{
			Schema: core.AgentFeedbackAPISchema, Credential: credential, ApprovalToken: approvalToken,
		}
		var response core.AgentFeedbackSubmitResponse
		if err := callAgentFeedbackAPI(ctx, client, "/feedback/submit", request, &response); err != nil {
			_, _ = fmt.Fprintf(stderr, "feedback submit failed closed: %v\n", err)
			return 1
		}
		if response.Schema != core.AgentFeedbackAPISchema || response.Status != core.AgentFeedbackStatusSubmitted ||
			strings.TrimSpace(response.ReferenceURL) == "" {
			_, _ = fmt.Fprintln(stderr, "feedback submit failed closed: runtime schema or status mismatch")
			return 1
		}
		if err := writeAgentFeedbackJSON(stdout, response); err != nil {
			_, _ = fmt.Fprintf(stderr, "feedback submit output failed: %v\n", err)
			return 1
		}
	}
	return 0
}
