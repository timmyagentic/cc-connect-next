package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/timmyagentic/cc-connect-next/core"
)

func TestAppServerSession_UsageLimitErrorNotificationIsClassified(t *testing.T) {
	s := &appServerSession{events: make(chan core.Event, 1)}
	s.handleNotification("error", json.RawMessage(`{"message":"No remaining credits for this account"}`))

	event := <-s.events
	if !errors.Is(event.Error, core.ErrUsageLimit) {
		t.Fatalf("error notification = %v, want usage-limit marker", event.Error)
	}
}

func TestAppServerSession_TurnCompletedUsageLimitIsClassified(t *testing.T) {
	s := &appServerSession{events: make(chan core.Event, 1)}
	s.threadID.Store("thread-1")
	s.currentTurn = "turn-1"
	s.handleNotification("turn/completed", json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"failed","error":{"message":"You've reached your usage limit"}}}`))

	event := <-s.events
	if !errors.Is(event.Error, core.ErrUsageLimit) {
		t.Fatalf("turn/completed error = %v, want usage-limit marker", event.Error)
	}
}

func TestAppServerSession_ApplyThreadRuntimeState(t *testing.T) {
	s := &appServerSession{}
	effort := "xhigh"

	s.applyThreadRuntimeState("/tmp/project", "gpt-5.4", &effort)

	if got := s.GetWorkDir(); got != "/tmp/project" {
		t.Fatalf("GetWorkDir() = %q, want /tmp/project", got)
	}
	if got := s.GetModel(); got != "gpt-5.4" {
		t.Fatalf("GetModel() = %q, want gpt-5.4", got)
	}
	if got := s.GetReasoningEffort(); got != "xhigh" {
		t.Fatalf("GetReasoningEffort() = %q, want xhigh", got)
	}
}

func TestAppServerSession_HandleRateLimitsUpdatedCachesUsage(t *testing.T) {
	s := &appServerSession{}
	raw, err := json.Marshal(appServerRateLimitsResponse{
		RateLimits: appServerRateLimitSnapshot{
			LimitID:   "codex",
			PlanType:  "pro",
			Primary:   &appServerRateLimitWindow{UsedPercent: 25, WindowDurationMins: 15, ResetsAt: 1730947200},
			Secondary: &appServerRateLimitWindow{UsedPercent: 42, WindowDurationMins: 60, ResetsAt: 1730950800},
			Credits:   &appServerCreditsSnapshot{HasCredits: true, Unlimited: false},
		},
	})
	if err != nil {
		t.Fatalf("marshal notification: %v", err)
	}

	s.handleNotification("account/rateLimits/updated", raw)

	report, err := s.GetUsage(context.Background())
	if err != nil {
		t.Fatalf("GetUsage() returned error: %v", err)
	}
	if report.Provider != "codex" {
		t.Fatalf("provider = %q, want codex", report.Provider)
	}
	if report.Plan != "pro" {
		t.Fatalf("plan = %q, want pro", report.Plan)
	}
	if len(report.Buckets) != 1 {
		t.Fatalf("buckets = %d, want 1", len(report.Buckets))
	}
	if got := report.Buckets[0].Name; got != "codex" {
		t.Fatalf("bucket name = %q, want codex", got)
	}
	if got := report.Buckets[0].Windows[0].WindowSeconds; got != 15*60 {
		t.Fatalf("primary window seconds = %d, want %d", got, 15*60)
	}
	if got := report.Buckets[0].Windows[1].UsedPercent; got != 42 {
		t.Fatalf("secondary used percent = %d, want 42", got)
	}
	if report.Credits == nil || !report.Credits.HasCredits {
		t.Fatalf("credits = %#v, want has credits", report.Credits)
	}
}

func TestAppServerSession_HandleThreadTokenUsageUpdatedCachesContextUsage(t *testing.T) {
	s := &appServerSession{}
	s.threadID.Store("thread-1")
	s.currentTurn = "turn-1"
	raw, err := json.Marshal(appServerThreadTokenUsageNotification{
		ThreadID: "thread-1",
		TurnID:   "turn-1",
		TokenUsage: struct {
			Total              codexTokenUsage `json:"total"`
			Last               codexTokenUsage `json:"last"`
			ModelContextWindow int             `json:"modelContextWindow"`
		}{
			Total: codexTokenUsage{
				TotalTokens:           52011395,
				InputTokens:           51847383,
				CachedInputTokens:     48187904,
				OutputTokens:          164012,
				ReasoningOutputTokens: 78910,
			},
			Last: codexTokenUsage{
				TotalTokens:           41061,
				InputTokens:           40849,
				CachedInputTokens:     36864,
				OutputTokens:          212,
				ReasoningOutputTokens: 32,
			},
			ModelContextWindow: 258400,
		},
	})
	if err != nil {
		t.Fatalf("marshal notification: %v", err)
	}

	s.handleNotification("thread/tokenUsage/updated", raw)

	usage := s.GetContextUsage()
	if usage == nil {
		t.Fatal("GetContextUsage() = nil, want cached context usage")
	}
	if usage.UsedTokens != 41061 {
		t.Fatalf("used tokens = %d, want 41061", usage.UsedTokens)
	}
	if usage.BaselineTokens != codexContextBaselineTokens {
		t.Fatalf("baseline tokens = %d, want %d", usage.BaselineTokens, codexContextBaselineTokens)
	}
	if usage.TotalTokens != 41061 {
		t.Fatalf("total tokens = %d, want 41061", usage.TotalTokens)
	}
	if usage.ContextWindow != 258400 {
		t.Fatalf("context window = %d, want 258400", usage.ContextWindow)
	}
	if usage.CachedInputTokens != 36864 {
		t.Fatalf("cached input tokens = %d, want 36864", usage.CachedInputTokens)
	}
	if usage.InputTokens != 40849 {
		t.Fatalf("input tokens = %d, want 40849", usage.InputTokens)
	}
}

func TestAppServerSession_Issue68IgnoresSubagentLifecycleNotifications(t *testing.T) {
	s := &appServerSession{
		events:      make(chan core.Event, 8),
		pendingMsgs: []string{"parent partial"},
		currentTurn: "parent-turn",
	}
	s.threadID.Store("parent-thread")

	s.handleNotification("turn/started", notificationProbe(t, map[string]any{
		"threadId": "child-thread",
		"turn":     map[string]any{"id": "child-turn", "status": "inProgress"},
	}))
	s.handleNotification("item/started", notificationProbe(t, map[string]any{
		"threadId": "child-thread",
		"turnId":   "child-turn",
		"item":     map[string]any{"type": "commandExecution", "command": "internal-child-command"},
	}))
	s.handleNotification("item/completed", notificationProbe(t, map[string]any{
		"threadId": "child-thread",
		"turnId":   "child-turn",
		"item":     map[string]any{"type": "agentMessage", "text": "child-only answer"},
	}))
	s.handleNotification("turn/completed", notificationProbe(t, map[string]any{
		"threadId": "child-thread",
		"turn":     map[string]any{"id": "child-turn", "status": "completed"},
	}))
	s.handleNotification("turn/completed", notificationProbe(t, map[string]any{
		"threadId": "failed-child-thread",
		"turn": map[string]any{
			"id":     "failed-child-turn",
			"status": "failed",
			"error":  map[string]any{"message": "child-only failure"},
		},
	}))
	s.handleNotification("thread/status/changed", notificationProbe(t, map[string]any{
		"threadId": "child-thread",
		"status":   map[string]any{"type": "idle"},
	}))

	if got := appServerCurrentTurn(s); got != "parent-turn" {
		t.Fatalf("current turn = %q after child lifecycle, want parent-turn", got)
	}
	if got := appServerPendingMessages(s); len(got) != 1 || got[0] != "parent partial" {
		t.Fatalf("pending messages = %v after child lifecycle, want parent partial", got)
	}
	assertNoAppServerEvent(t, s.events, "child lifecycle")

	s.handleNotification("item/completed", notificationProbe(t, map[string]any{
		"threadId": "parent-thread",
		"turnId":   "parent-turn",
		"item":     map[string]any{"type": "agentMessage", "text": " parent final"},
	}))
	s.handleNotification("turn/completed", notificationProbe(t, map[string]any{
		"threadId": "parent-thread",
		"turn":     map[string]any{"id": "parent-turn", "status": "completed"},
	}))

	wantEvents := []struct {
		type_   core.EventType
		content string
		done    bool
	}{
		{type_: core.EventText, content: "parent partial"},
		{type_: core.EventText, content: " parent final"},
		{type_: core.EventResult, done: true},
	}
	for i, want := range wantEvents {
		select {
		case event := <-s.events:
			if event.Type != want.type_ || event.Content != want.content || event.Done != want.done {
				t.Fatalf("event %d = %#v, want type=%s content=%q done=%v", i, event, want.type_, want.content, want.done)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for parent event %d", i)
		}
	}
	assertNoAppServerEvent(t, s.events, "completed parent turn")
}

func TestAppServerSession_Issue68IgnoresInactiveRootTurnNotifications(t *testing.T) {
	s := &appServerSession{
		events:      make(chan core.Event, 4),
		pendingMsgs: []string{"parent partial"},
		currentTurn: "parent-turn",
	}
	s.threadID.Store("parent-thread")

	s.handleNotification("item/started", notificationProbe(t, map[string]any{
		"threadId": "parent-thread",
		"turnId":   "stale-turn",
		"item":     map[string]any{"type": "commandExecution", "command": "stale command"},
	}))
	s.handleNotification("item/completed", notificationProbe(t, map[string]any{
		"threadId": "parent-thread",
		"turnId":   "stale-turn",
		"item":     map[string]any{"type": "agentMessage", "text": "stale answer"},
	}))
	s.handleNotification("turn/completed", notificationProbe(t, map[string]any{
		"threadId": "parent-thread",
		"turn":     map[string]any{"id": "stale-turn", "status": "completed"},
	}))

	if got := appServerCurrentTurn(s); got != "parent-turn" {
		t.Fatalf("current turn = %q after stale notifications, want parent-turn", got)
	}
	if got := appServerPendingMessages(s); len(got) != 1 || got[0] != "parent partial" {
		t.Fatalf("pending messages = %v after stale notifications, want parent partial", got)
	}
	assertNoAppServerEvent(t, s.events, "inactive root turn")
}

func TestAppServerSession_Issue68IgnoresSubagentTokenUsageNotifications(t *testing.T) {
	s := &appServerSession{currentTurn: "parent-turn"}
	s.threadID.Store("parent-thread")

	s.handleNotification("thread/tokenUsage/updated", tokenUsageNotificationProbe(t, "parent-thread", "parent-turn", 1234))
	s.handleNotification("thread/tokenUsage/updated", tokenUsageNotificationProbe(t, "child-thread", "child-turn", 9876))
	s.handleNotification("thread/tokenUsage/updated", tokenUsageNotificationProbe(t, "parent-thread", "stale-turn", 5555))

	usage := s.GetContextUsage()
	if usage == nil {
		t.Fatal("GetContextUsage() = nil, want parent usage")
	}
	if usage.UsedTokens != 1234 {
		t.Fatalf("used tokens = %d after child update, want parent value 1234", usage.UsedTokens)
	}
}

func TestAppServerSession_Issue68IgnoresSubagentErrorNotification(t *testing.T) {
	s := &appServerSession{
		events:      make(chan core.Event, 2),
		pendingMsgs: []string{"parent partial"},
		currentTurn: "parent-turn",
	}
	s.threadID.Store("parent-thread")

	s.handleNotification("error", notificationProbe(t, map[string]any{
		"threadId":  "child-thread",
		"turnId":    "child-turn",
		"message":   "No remaining credits for this account",
		"error":     map[string]any{"message": "No remaining credits for this account"},
		"willRetry": false,
	}))

	if got := appServerCurrentTurn(s); got != "parent-turn" {
		t.Fatalf("current turn = %q after child error, want parent-turn", got)
	}
	if got := appServerPendingMessages(s); len(got) != 1 || got[0] != "parent partial" {
		t.Fatalf("pending messages = %v after child error, want parent partial", got)
	}
	assertNoAppServerEvent(t, s.events, "child error")
}

func TestAppServerSession_Issue68ClassifiesScopedV2ErrorNotification(t *testing.T) {
	s := &appServerSession{events: make(chan core.Event, 1), currentTurn: "parent-turn"}
	s.threadID.Store("parent-thread")

	s.handleNotification("error", notificationProbe(t, map[string]any{
		"threadId":  "parent-thread",
		"turnId":    "parent-turn",
		"error":     map[string]any{"message": "No remaining credits for this account"},
		"willRetry": false,
	}))

	select {
	case event := <-s.events:
		if !errors.Is(event.Error, core.ErrUsageLimit) {
			t.Fatalf("v2 error notification = %v, want usage-limit marker", event.Error)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for scoped v2 error")
	}
}

func TestAppServerSession_Issue68IgnoresRetryableScopedV2Error(t *testing.T) {
	s := &appServerSession{events: make(chan core.Event, 1), currentTurn: "parent-turn"}
	s.threadID.Store("parent-thread")

	s.handleNotification("error", notificationProbe(t, map[string]any{
		"threadId":  "parent-thread",
		"turnId":    "parent-turn",
		"error":     map[string]any{"message": "temporary upstream disconnect"},
		"willRetry": true,
	}))

	if got := appServerCurrentTurn(s); got != "parent-turn" {
		t.Fatalf("current turn = %q after retryable error, want parent-turn", got)
	}
	assertNoAppServerEvent(t, s.events, "retryable root error")
}

func TestAppServerSession_Issue68RejectsSubagentInteractiveRequests(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		threadID   string
		turnID     string
		params     map[string]any
		resultKey  string
		resultText string
	}{
		{
			name:       "command approval",
			method:     "item/commandExecution/requestApproval",
			threadID:   "child-thread",
			turnID:     "child-turn",
			params:     map[string]any{"itemId": "cmd-1", "command": "internal-child-command", "cwd": "/tmp"},
			resultKey:  "decision",
			resultText: "decline",
		},
		{
			name:       "file approval",
			method:     "item/fileChange/requestApproval",
			threadID:   "child-thread",
			turnID:     "child-turn",
			params:     map[string]any{"itemId": "patch-1", "reason": "internal child patch"},
			resultKey:  "decision",
			resultText: "decline",
		},
		{
			name:      "permissions approval",
			method:    "item/permissions/requestApproval",
			threadID:  "child-thread",
			turnID:    "child-turn",
			params:    map[string]any{"itemId": "permissions-1", "permissions": map[string]any{"network": true}},
			resultKey: "permissions",
		},
		{
			name:     "request user input",
			method:   "item/tool/requestUserInput",
			threadID: "child-thread",
			turnID:   "child-turn",
			params: map[string]any{
				"itemId":    "question-1",
				"questions": []any{map[string]any{"id": "secret", "question": "Expose child-only data?"}},
			},
			resultKey: "answers",
		},
		{
			name:     "same thread wrong turn",
			method:   "item/tool/requestUserInput",
			threadID: "parent-thread",
			turnID:   "stale-turn",
			params: map[string]any{
				"itemId":    "question-2",
				"questions": []any{map[string]any{"id": "stale", "question": "Expose stale-turn data?"}},
			},
			resultKey: "answers",
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			stdin := &lockedWriteCloser{}
			s := &appServerSession{
				events:           make(chan core.Event, 2),
				ctx:              ctx,
				pendingApprovals: make(map[string]chan core.PermissionResult),
				stdin:            stdin,
				currentTurn:      "parent-turn",
			}
			s.threadID.Store("parent-thread")

			params := make(map[string]any, len(tt.params)+2)
			for key, value := range tt.params {
				params[key] = value
			}
			params["threadId"] = tt.threadID
			params["turnId"] = tt.turnID
			s.handleServerRequest(serverRequestProbe(t, fmt.Sprintf(`"child-%d"`, i), tt.method, params))

			assertNoAppServerEvent(t, s.events, tt.name)
			line := waitForWrittenJSONLine(t, stdin)
			var envelope struct {
				ID     string         `json:"id"`
				Result map[string]any `json:"result"`
			}
			if err := json.Unmarshal([]byte(line), &envelope); err != nil {
				t.Fatalf("decode response %q: %v", line, err)
			}
			if envelope.ID != fmt.Sprintf("child-%d", i) {
				t.Fatalf("response id = %q, want child-%d", envelope.ID, i)
			}
			got, ok := envelope.Result[tt.resultKey]
			if !ok {
				t.Fatalf("response result = %#v, want key %q", envelope.Result, tt.resultKey)
			}
			if tt.resultText != "" {
				if got != tt.resultText {
					t.Fatalf("response %s = %#v, want %q", tt.resultKey, got, tt.resultText)
				}
			} else if values, ok := got.(map[string]any); !ok || len(values) != 0 {
				t.Fatalf("response %s = %#v, want empty object", tt.resultKey, got)
			}

			s.approvalsMu.Lock()
			pendingCount := len(s.pendingApprovals)
			s.approvalsMu.Unlock()
			if pendingCount != 0 {
				t.Fatalf("pending approvals = %d, want none", pendingCount)
			}
		})
	}
}

func TestAppServerSession_Issue68AcceptsLegacyUnscopedRootNotifications(t *testing.T) {
	s := &appServerSession{events: make(chan core.Event, 2), currentTurn: "parent-turn"}
	s.threadID.Store("parent-thread")

	s.handleNotification("item/completed", notificationProbe(t, map[string]any{
		"item": map[string]any{"type": "agentMessage", "text": "legacy parent answer"},
	}))
	s.handleNotification("turn/completed", notificationProbe(t, map[string]any{
		"turn": map[string]any{"status": "completed"},
	}))

	textEvent := <-s.events
	if textEvent.Type != core.EventText || textEvent.Content != "legacy parent answer" {
		t.Fatalf("legacy text event = %#v, want parent answer", textEvent)
	}
	resultEvent := <-s.events
	if resultEvent.Type != core.EventResult || !resultEvent.Done {
		t.Fatalf("legacy result event = %#v, want completed result", resultEvent)
	}
}

func TestAppServerSession_RequestTimeoutIncludesBlockedStdinWrite(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stdin := newBlockingWriteCloser()
	defer func() { _ = stdin.Close() }()

	session := &appServerSession{
		ctx:     ctx,
		cancel:  cancel,
		events:  make(chan core.Event),
		stdin:   stdin,
		pending: make(map[int64]chan rpcResponseEnvelope),
	}

	done := make(chan error, 1)
	go func() {
		var out map[string]any
		done <- session.requestWithTimeout("turn/start", map[string]any{
			"input": strings.Repeat("x", 1024),
		}, &out, 25*time.Millisecond)
	}()

	select {
	case <-stdin.started:
	case <-time.After(time.Second):
		t.Fatal("request did not attempt to write to stdin")
	}

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("requestWithTimeout returned nil, want write timeout")
		}
		if !strings.Contains(err.Error(), "turn/start") || !strings.Contains(err.Error(), "write timed out") {
			t.Fatalf("error = %q, want turn/start write timeout", err.Error())
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("requestWithTimeout did not return while stdin write was blocked")
	}

	if !stdin.Closed() {
		t.Fatal("blocked stdin was not closed after timeout")
	}
}

func TestMapAppServerRateLimits_PrefersMultiBucketView(t *testing.T) {
	report := mapAppServerRateLimits(appServerRateLimitsResponse{
		RateLimits: appServerRateLimitSnapshot{
			LimitID:  "legacy",
			PlanType: "team",
			Primary:  &appServerRateLimitWindow{UsedPercent: 99, WindowDurationMins: 15},
		},
		RateLimitsByLimitID: map[string]appServerRateLimitSnapshot{
			"codex": {
				LimitID:   "codex",
				LimitName: "Codex",
				PlanType:  "team",
				Primary:   &appServerRateLimitWindow{UsedPercent: 10, WindowDurationMins: 15},
			},
			"codex_other": {
				LimitID:  "codex_other",
				PlanType: "team",
				Primary:  &appServerRateLimitWindow{UsedPercent: 20, WindowDurationMins: 60},
			},
		},
	})

	if report.Plan != "team" {
		t.Fatalf("plan = %q, want team", report.Plan)
	}
	if len(report.Buckets) != 2 {
		t.Fatalf("buckets = %d, want 2", len(report.Buckets))
	}
	if report.Buckets[0].Name != "Codex" {
		t.Fatalf("first bucket = %q, want Codex", report.Buckets[0].Name)
	}
	if report.Buckets[1].Name != "codex_other" {
		t.Fatalf("second bucket = %q, want codex_other", report.Buckets[1].Name)
	}
}

func TestAppServerSession_HandleRequestUserInputEmitsAskQuestion(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stdin := &lockedWriteCloser{}
	s := &appServerSession{
		events:           make(chan core.Event, 4),
		ctx:              ctx,
		pendingApprovals: make(map[string]chan core.PermissionResult),
		stdin:            stdin,
		currentTurn:      "turn-1",
	}
	s.threadID.Store("thread-1")

	s.handleServerRequest(serverRequestProbe(t, `"rui-1"`, "item/tool/requestUserInput", map[string]any{
		"threadId": "thread-1",
		"turnId":   "turn-1",
		"itemId":   "call-1",
		"questions": []any{
			map[string]any{
				"id":       "database",
				"header":   "Database",
				"question": "Which database should we use?",
				"isOther":  true,
				"isSecret": false,
				"options": []any{
					map[string]any{"label": "Postgres", "description": "Use the existing relational database"},
					map[string]any{"label": "SQLite", "description": "Keep it embedded"},
				},
			},
		},
	}))

	var event core.Event
	select {
	case event = <-s.events:
	case <-time.After(time.Second):
		t.Fatal("expected AskUserQuestion event")
	}
	if event.Type != core.EventPermissionRequest {
		t.Fatalf("event type = %s, want %s", event.Type, core.EventPermissionRequest)
	}
	if event.ToolName != "AskUserQuestion" {
		t.Fatalf("tool name = %q, want AskUserQuestion", event.ToolName)
	}
	if event.RequestID != `"rui-1"` {
		t.Fatalf("request id = %q, want raw JSON id", event.RequestID)
	}
	if len(event.Questions) != 1 {
		t.Fatalf("questions = %d, want 1", len(event.Questions))
	}
	q := event.Questions[0]
	if q.Question != "Which database should we use?" || q.Header != "Database" {
		t.Fatalf("question = %#v", q)
	}
	if len(q.Options) != 2 || q.Options[0].Label != "Postgres" || q.Options[1].Description != "Keep it embedded" {
		t.Fatalf("options = %#v", q.Options)
	}
	if stdin.String() != "" {
		t.Fatalf("request_user_input should not write before the answer, got %q", stdin.String())
	}
}

func TestAppServerSession_HandleRequestUserInputWritesCodexResponse(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stdin := &lockedWriteCloser{}
	s := &appServerSession{
		events:           make(chan core.Event, 4),
		ctx:              ctx,
		pendingApprovals: make(map[string]chan core.PermissionResult),
		stdin:            stdin,
		currentTurn:      "turn-1",
	}
	s.threadID.Store("thread-1")

	s.handleServerRequest(serverRequestProbe(t, `"rui-2"`, "item/tool/requestUserInput", map[string]any{
		"threadId": "thread-1",
		"turnId":   "turn-1",
		"itemId":   "call-2",
		"questions": []any{
			map[string]any{
				"id":       "database",
				"header":   "Database",
				"question": "Which database should we use?",
				"options": []any{
					map[string]any{"label": "Postgres", "description": "Use the existing relational database"},
					map[string]any{"label": "SQLite", "description": "Keep it embedded"},
				},
			},
		},
	}))

	var event core.Event
	select {
	case event = <-s.events:
	case <-time.After(time.Second):
		t.Fatal("expected AskUserQuestion event")
	}
	if err := s.RespondPermission(event.RequestID, core.PermissionResult{
		Behavior: "allow",
		UpdatedInput: map[string]any{
			"answers": map[string]any{
				"Which database should we use?": "Postgres",
			},
		},
	}); err != nil {
		t.Fatalf("RespondPermission() error = %v", err)
	}

	line := waitForWrittenJSONLine(t, stdin)
	var envelope struct {
		JSONRPC string `json:"jsonrpc"`
		ID      string `json:"id"`
		Result  struct {
			Answers map[string]struct {
				Answers []string `json:"answers"`
			} `json:"answers"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(line), &envelope); err != nil {
		t.Fatalf("decode response %q: %v", line, err)
	}
	if envelope.JSONRPC != "2.0" || envelope.ID != "rui-2" {
		t.Fatalf("envelope = %#v", envelope)
	}
	got := envelope.Result.Answers["database"].Answers
	if len(got) != 1 || got[0] != "Postgres" {
		t.Fatalf("answers[database] = %#v, want [Postgres]", got)
	}
}

var _ interface {
	GetUsage(context.Context) (*core.UsageReport, error)
} = (*appServerSession)(nil)

var _ interface {
	GetContextUsage() *core.ContextUsage
} = (*appServerSession)(nil)

type lockedWriteCloser struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (w *lockedWriteCloser) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

func (w *lockedWriteCloser) Close() error { return nil }

func (w *lockedWriteCloser) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

var _ io.WriteCloser = (*lockedWriteCloser)(nil)

type blockingWriteCloser struct {
	started   chan struct{}
	closed    chan struct{}
	closeOnce sync.Once

	mu       sync.Mutex
	isClosed bool
}

func newBlockingWriteCloser() *blockingWriteCloser {
	return &blockingWriteCloser{
		started: make(chan struct{}),
		closed:  make(chan struct{}),
	}
}

func (w *blockingWriteCloser) Write([]byte) (int, error) {
	select {
	case <-w.started:
	default:
		close(w.started)
	}
	<-w.closed
	return 0, io.ErrClosedPipe
}

func (w *blockingWriteCloser) Close() error {
	w.closeOnce.Do(func() {
		w.mu.Lock()
		w.isClosed = true
		w.mu.Unlock()
		close(w.closed)
	})
	return nil
}

func (w *blockingWriteCloser) Closed() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.isClosed
}

var _ io.WriteCloser = (*blockingWriteCloser)(nil)

func serverRequestProbe(t *testing.T, idJSON, method string, params any) map[string]json.RawMessage {
	t.Helper()
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	methodJSON, err := json.Marshal(method)
	if err != nil {
		t.Fatalf("marshal method: %v", err)
	}
	return map[string]json.RawMessage{
		"id":     json.RawMessage(idJSON),
		"method": methodJSON,
		"params": paramsJSON,
	}
}

func notificationProbe(t *testing.T, params any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal notification: %v", err)
	}
	return raw
}

func tokenUsageNotificationProbe(t *testing.T, threadID, turnID string, totalTokens int) json.RawMessage {
	t.Helper()
	return notificationProbe(t, map[string]any{
		"threadId": threadID,
		"turnId":   turnID,
		"tokenUsage": map[string]any{
			"total": map[string]any{},
			"last": map[string]any{
				"totalTokens": totalTokens,
			},
			"modelContextWindow": 258400,
		},
	})
}

func appServerCurrentTurn(s *appServerSession) string {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	return s.currentTurn
}

func appServerPendingMessages(s *appServerSession) []string {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	return append([]string(nil), s.pendingMsgs...)
}

func assertNoAppServerEvent(t *testing.T, events <-chan core.Event, after string) {
	t.Helper()
	select {
	case event := <-events:
		t.Fatalf("%s emitted event %#v, want none", after, event)
	default:
	}
}

func waitForWrittenJSONLine(t *testing.T, w *lockedWriteCloser) string {
	t.Helper()
	deadline := time.After(time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for JSON response, buffer=%q", w.String())
		case <-ticker.C:
			for _, line := range strings.Split(w.String(), "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					return line
				}
			}
		}
	}
}
