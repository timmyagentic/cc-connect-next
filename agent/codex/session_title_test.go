package codex

import (
	"encoding/json"
	"strings"
	"testing"

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
			name:   "sensitive fragments",
			prompt: "给 person@example.com 检查 https://example.com/private/path 和 sk-secretvalue1234567890",
			want:   "给 [email] 检查 [link] 和 [secret]",
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
	if params["threadId"] != "thread-1" || params["name"] != "你看看电脑有没有锁屏" {
		t.Fatalf("thread/name/set params = %#v", params)
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

	if err := s.SetInitialSessionTitle("不要覆盖已有标题"); err != nil {
		t.Fatalf("SetInitialSessionTitle() error = %v", err)
	}
	stdin.mu.Lock()
	requestCount := len(stdin.requests)
	stdin.mu.Unlock()
	if requestCount != 0 {
		t.Fatalf("request count = %d, want no rename request", requestCount)
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
	if params["threadId"] != "persisted-thread" || params["name"] != "Readable title" {
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
