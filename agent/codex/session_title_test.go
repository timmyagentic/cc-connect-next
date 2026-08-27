package codex

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/timmyagentic/cc-connect-next/core"
)

func TestInitialCodexThreadTitle(t *testing.T) {
	brief := core.BuildCapabilityBrief("codex", []string{"model", "reasoning_effort"})
	longTitle := strings.Repeat("测", 55)

	tests := []struct {
		name   string
		prompt string
		want   string
	}{
		{
			name:   "plain first line",
			prompt: "  你看看电脑有没有锁屏  \n第二行补充",
			want:   "你看看电脑有没有锁屏",
		},
		{
			name:   "capability brief",
			prompt: brief + "\n\n把 fast 模式关掉吧",
			want:   "把 fast 模式关掉吧",
		},
		{
			name: "capability and sender metadata",
			prompt: brief + "\n\n" +
				"[cc-connect-next sender_id=ou_private sender_name=\"Alice\" platform=feishu chat_id=oc_private]\n" +
				"检查流水线状态",
			want: "检查流水线状态",
		},
		{
			name: "quoted context uses actual request",
			prompt: brief + "\n\n" +
				"[Quoted message from User]:\nhttps://example.com/private 原消息\n\n\n@廷君的助手\n 看一下这个接口为什么失败",
			want: "看一下这个接口为什么失败",
		},
		{
			name:   "quote only does not expose quoted content",
			prompt: "[Quoted message from User]:\n另一位用户的敏感内容\n\n",
			want:   "New conversation",
		},
		{
			name:   "reply chain only does not expose quoted content",
			prompt: "--- Reply chain (2 messages) ---\n[1] user:\n第一条\n\n[2] assistant:\n第二条\n\n",
			want:   "New conversation",
		},
		{
			name:   "sensitive fragments",
			prompt: "给 person@example.com 检查 https://example.com/private/path 和 sk-secretvalue1234567890",
			want:   "给 [email] 检查 [link] 和 [secret]",
		},
		{
			name:   "assignment credential",
			prompt: "请使用 token=abcdefghijklmnop 和 api_key=zyxwvutsrqponmlk",
			want:   "请使用 token=[secret] 和 api_key=[secret]",
		},
		{
			name:   "quoted passphrase",
			prompt: `请使用 password="correct horse battery staple!" 连接服务`,
			want:   "请使用 password=[secret] 连接服务",
		},
		{
			name:   "json client secret",
			prompt: `检查 "client_secret": "value with spaces & punctuation!" 是否有效`,
			want:   "检查 client_secret=[secret] 是否有效",
		},
		{
			name:   "github pat",
			prompt: "检查 github_pat_11AA22BB33CC44DD55EE66FF77GG88HH",
			want:   "检查 [secret]",
		},
		{
			name:   "jwt",
			prompt: "检查 eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			want:   "检查 [secret]",
		},
		{
			name:   "aws access key",
			prompt: "检查 AKIAIOSFODNN7EXAMPLE",
			want:   "检查 [secret]",
		},
		{
			name:   "unicode length limit",
			prompt: longTitle,
			want:   strings.Repeat("测", 48) + "...",
		},
		{
			name:   "metadata only",
			prompt: brief,
			want:   "New conversation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := initialCodexThreadTitle(tt.prompt); got != tt.want {
				t.Fatalf("initialCodexThreadTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatSessionTitlePrefix(t *testing.T) {
	for _, tt := range []struct {
		name   string
		prefix string
		title  string
		want   string
	}{
		{name: "default", title: "检查锁屏状态", want: "[飞书] 检查锁屏状态"},
		{name: "blank uses default", prefix: "  ", title: "检查锁屏状态", want: "[飞书] 检查锁屏状态"},
		{name: "custom", prefix: " [Slack] ", title: "检查锁屏状态", want: "[Slack] 检查锁屏状态"},
		{name: "idempotent", prefix: "[飞书]", title: "[飞书] 检查锁屏状态", want: "[飞书] 检查锁屏状态"},
		{name: "partial text is not prefix", prefix: "A", title: "Apple", want: "A Apple"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatSessionTitle(tt.prefix, tt.title); got != tt.want {
				t.Fatalf("formatSessionTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNew_SessionTitleOptions(t *testing.T) {
	agentValue, err := New(map[string]any{
		"cmd":                  os.Args[0],
		"session_title_prefix": " [Slack] ",
		"session_title_model":  " gpt-5.3-codex-spark ",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	agent := agentValue.(*Agent)
	if agent.sessionTitlePrefix != "[Slack]" {
		t.Fatalf("sessionTitlePrefix = %q", agent.sessionTitlePrefix)
	}
	if agent.sessionTitleModel != "gpt-5.3-codex-spark" {
		t.Fatalf("sessionTitleModel = %q", agent.sessionTitleModel)
	}
	opts := agent.WorkspaceAgentOptions()
	if opts["session_title_prefix"] != "[Slack]" || opts["session_title_model"] != "gpt-5.3-codex-spark" {
		t.Fatalf("WorkspaceAgentOptions() = %#v", opts)
	}

	defaultValue, err := New(map[string]any{"cmd": os.Args[0]})
	if err != nil {
		t.Fatalf("New(defaults) error = %v", err)
	}
	defaults := defaultValue.(*Agent)
	if defaults.sessionTitlePrefix != "[飞书]" || defaults.sessionTitleModel != "" {
		t.Fatalf("defaults = prefix %q model %q", defaults.sessionTitlePrefix, defaults.sessionTitleModel)
	}
}

func TestAppServerSession_SetInitialSessionTitleNamesFreshThread(t *testing.T) {
	stdin := &steerFakeStdin{}
	s := newSteerTestSession(t, stdin, "thread-1", "")
	stdin.respond = respondSuccess(s, `{}`)

	prompt := core.BuildCapabilityBrief("codex", []string{"model"}) + "\n\n你看看电脑有没有锁屏"
	if err := s.SetInitialSessionTitle(prompt); err != nil {
		t.Fatalf("SetInitialSessionTitle() error = %v", err)
	}

	req := stdin.lastRequest()
	if req == nil || req["method"] != "thread/name/set" {
		t.Fatalf("request = %#v, want thread/name/set", req)
	}
	params, _ := req["params"].(map[string]any)
	if params["threadId"] != "thread-1" || params["name"] != "[飞书] 你看看电脑有没有锁屏" {
		t.Fatalf("thread/name/set params = %#v", params)
	}
}

func TestAppServerSession_OptionalTitleModel(t *testing.T) {
	for _, tt := range []struct {
		name          string
		model         string
		generated     string
		generateErr   error
		wantTitle     string
		wantCalls     int
		wantCandidate string
	}{
		{
			name:      "disabled by default",
			wantTitle: "[飞书] 你看看电脑有没有锁屏",
		},
		{
			name:          "spark success",
			model:         "gpt-5.3-codex-spark",
			generated:     "检查电脑锁屏状态",
			wantTitle:     "[飞书] 检查电脑锁屏状态",
			wantCalls:     1,
			wantCandidate: "你看看电脑有没有锁屏",
		},
		{
			name:          "spark failure falls back",
			model:         "gpt-5.3-codex-spark",
			generateErr:   errors.New("title generation unavailable"),
			wantTitle:     "[飞书] 你看看电脑有没有锁屏",
			wantCalls:     1,
			wantCandidate: "你看看电脑有没有锁屏",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			stdin := &steerFakeStdin{}
			s := newSteerTestSession(t, stdin, "thread-1", "")
			s.sessionTitleModel = tt.model
			calls := 0
			candidate := ""
			s.titleGenerator = func(_ context.Context, input string) (string, error) {
				calls++
				candidate = input
				return tt.generated, tt.generateErr
			}
			stdin.respond = respondSuccess(s, `{}`)

			prompt := core.BuildCapabilityBrief("codex", []string{"model"}) + "\n\n你看看电脑有没有锁屏"
			if err := s.SetInitialSessionTitle(prompt); err != nil {
				t.Fatalf("SetInitialSessionTitle() error = %v", err)
			}
			params, _ := stdin.lastRequest()["params"].(map[string]any)
			if params["name"] != tt.wantTitle {
				t.Fatalf("title = %v, want %q", params["name"], tt.wantTitle)
			}
			if calls != tt.wantCalls || candidate != tt.wantCandidate {
				t.Fatalf("generator calls = %d candidate = %q", calls, candidate)
			}
		})
	}
}

func TestAppServerSession_ExplicitRenameWinsDuringInitialGeneration(t *testing.T) {
	stdin := &steerFakeStdin{}
	s := newSteerTestSession(t, stdin, "thread-1", "")
	s.sessionTitleModel = "gpt-5.3-codex-spark"
	started := make(chan struct{})
	release := make(chan struct{})
	s.titleGenerator = func(context.Context, string) (string, error) {
		close(started)
		<-release
		return "自动生成标题", nil
	}
	stdin.respond = respondSuccess(s, `{}`)

	initialDone := make(chan error, 1)
	go func() {
		initialDone <- s.SetInitialSessionTitle("首个真实问题")
	}()
	<-started
	if err := s.SetSessionTitle("thread-1", "用户显式标题"); err != nil {
		t.Fatalf("SetSessionTitle() error = %v", err)
	}
	close(release)
	if err := <-initialDone; err != nil {
		t.Fatalf("SetInitialSessionTitle() error = %v", err)
	}

	stdin.mu.Lock()
	requests := append([]map[string]any(nil), stdin.requests...)
	stdin.mu.Unlock()
	if len(requests) != 1 {
		t.Fatalf("thread/name/set request count = %d, want only explicit rename", len(requests))
	}
	params, _ := requests[0]["params"].(map[string]any)
	if params["name"] != "[飞书] 用户显式标题" {
		t.Fatalf("final title request = %#v, want explicit rename", params)
	}
}

func TestAppServerSession_ContextDeadlineStopsTitleBeforeRPC(t *testing.T) {
	stdin := &steerFakeStdin{}
	s := newSteerTestSession(t, stdin, "thread-1", "")
	s.sessionTitleModel = "gpt-5.3-codex-spark"
	s.titleGenerator = func(ctx context.Context, _ string) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	err := s.SetInitialSessionTitleContext(ctx, "首个真实问题")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("SetInitialSessionTitleContext() error = %v, want context deadline exceeded", err)
	}
	stdin.mu.Lock()
	requestCount := len(stdin.requests)
	stdin.mu.Unlock()
	if requestCount != 0 {
		t.Fatalf("thread/name/set request count = %d, want 0 after deadline", requestCount)
	}
}

func TestSessionTitleGenerationArgsAreEphemeralAndPromptFree(t *testing.T) {
	s := &appServerSession{
		cliExtraArgs:      []string{"--profile", "work", "--dangerously-bypass-approvals-and-sandbox"},
		sessionTitleModel: "gpt-5.3-codex-spark",
	}
	args := s.sessionTitleGenerationArgs("/isolated/title-cwd")
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"exec",
		"--ephemeral",
		"--ignore-user-config",
		"--ignore-rules",
		"--skip-git-repo-check",
		"-C /isolated/title-cwd",
		"-m gpt-5.3-codex-spark",
		"-s read-only",
		"--disable plugins",
		"--disable apps",
		"--disable skill_search",
		"--disable memories",
		"--json -",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("args = %q, missing %q", joined, want)
		}
	}
	for _, forbidden := range []string{"--profile", "--dangerously-bypass-approvals-and-sandbox"} {
		if strings.Contains(joined, forbidden) {
			t.Errorf("isolated title args = %q, contains inherited %q", joined, forbidden)
		}
	}
	if strings.Contains(joined, "private user request") {
		t.Fatalf("user request leaked into argv: %q", joined)
	}
}

func TestParseSessionTitleGenerationOutput(t *testing.T) {
	input := strings.NewReader(strings.Join([]string{
		`{"type":"thread.started","thread_id":"ephemeral"}`,
		`not-json`,
		`{"type":"item.completed","item":{"type":"agent_message","text":"检查电脑锁屏状态"}}`,
		`{"type":"turn.completed","usage":{"input_tokens":10}}`,
	}, "\n"))
	title, err := parseSessionTitleGenerationOutput(input)
	if err != nil {
		t.Fatalf("parseSessionTitleGenerationOutput() error = %v", err)
	}
	if title != "检查电脑锁屏状态" {
		t.Fatalf("title = %q", title)
	}

	if _, err := parseSessionTitleGenerationOutput(strings.NewReader(`{"type":"turn.completed"}`)); err == nil {
		t.Fatal("missing agent message should fail")
	}
}

func TestAppServerSession_SendDoesNotOwnInitialTitle(t *testing.T) {
	stdin := &steerFakeStdin{}
	s := newSteerTestSession(t, stdin, "thread-1", "")
	stdin.respond = func(id int64, req map[string]any) {
		if req["method"] != "turn/start" {
			t.Errorf("Send() emitted %v, want only turn/start", req["method"])
		}
		s.handleResponse(rpcResponseEnvelope{ID: id, Result: json.RawMessage(`{"turn":{"id":"turn-1"}}`)})
	}

	if err := s.Send("处理创建后的首个问题", nil, nil); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	stdin.mu.Lock()
	requestCount := len(stdin.requests)
	stdin.mu.Unlock()
	if requestCount != 1 {
		t.Fatalf("request count = %d, want one turn/start request", requestCount)
	}
}

func TestAppServerSession_InitialTitleFailureIsReturnedToCreationBoundary(t *testing.T) {
	stdin := &steerFakeStdin{}
	s := newSteerTestSession(t, stdin, "thread-1", "")
	stdin.respond = respondError(s, "method unavailable")

	err := s.SetInitialSessionTitle("继续处理问题")
	if err == nil || !strings.Contains(err.Error(), "thread/name/set") {
		t.Fatalf("SetInitialSessionTitle() error = %v, want thread/name/set failure", err)
	}
}

func TestAppServerSession_HandledThreadKeepsExistingTitle(t *testing.T) {
	stdin := &steerFakeStdin{}
	s := newSteerTestSession(t, stdin, "thread-1", "")
	s.initialTitleHandled = true
	s.sessionTitleModel = "gpt-5.3-codex-spark"
	generatorCalls := 0
	s.titleGenerator = func(context.Context, string) (string, error) {
		generatorCalls++
		return "不应生成", nil
	}

	if err := s.SetInitialSessionTitle("不要覆盖已有标题"); err != nil {
		t.Fatalf("SetInitialSessionTitle() error = %v", err)
	}
	stdin.mu.Lock()
	requestCount := len(stdin.requests)
	stdin.mu.Unlock()
	if requestCount != 0 {
		t.Fatalf("request count = %d, want no rename request", requestCount)
	}
	if generatorCalls != 0 {
		t.Fatalf("generator calls = %d, want 0 for handled thread", generatorCalls)
	}
}

func TestAppServerSession_SetSessionTitleUsesOfficialRPC(t *testing.T) {
	stdin := &steerFakeStdin{}
	s := newSteerTestSession(t, stdin, "current-thread", "")
	stdin.respond = respondSuccess(s, `{}`)

	if err := s.SetSessionTitle("persisted-thread", "Readable title"); err != nil {
		t.Fatalf("SetSessionTitle() error = %v", err)
	}
	req := stdin.lastRequest()
	if req["method"] != "thread/name/set" {
		t.Fatalf("method = %v, want thread/name/set", req["method"])
	}
	params, _ := req["params"].(map[string]any)
	if params["threadId"] != "persisted-thread" || params["name"] != "[飞书] Readable title" {
		t.Fatalf("params = %#v", params)
	}

	if err := s.SetSessionTitle("", "Readable title"); err == nil {
		t.Fatal("empty thread id should be rejected")
	}
	if err := s.SetSessionTitle("persisted-thread", "  "); err == nil {
		t.Fatal("empty title should be rejected")
	}
}

var _ core.SessionTitleSetter = (*appServerSession)(nil)
var _ core.InitialSessionTitleSetter = (*appServerSession)(nil)
var _ core.ContextInitialSessionTitleSetter = (*appServerSession)(nil)
