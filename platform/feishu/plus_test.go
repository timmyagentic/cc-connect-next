package feishu

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chenhg5/cc-connect/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func TestParsePlusConfigPreservesNativeBehaviorByDefault(t *testing.T) {
	cfg, err := parsePlusConfig(map[string]any{})
	if err != nil {
		t.Fatalf("parsePlusConfig() error = %v", err)
	}
	if cfg.enabled {
		t.Fatal("enabled = true, want false")
	}
	if cfg.identityMode != plusIdentityLegacy {
		t.Fatalf("identityMode = %q, want %q", cfg.identityMode, plusIdentityLegacy)
	}
}

func TestParsePlusConfigDefaultsToRecoveringFailClosedIdentity(t *testing.T) {
	cfg, err := parsePlusConfig(map[string]any{"plus_enabled": true})
	if err != nil {
		t.Fatalf("parsePlusConfig() error = %v", err)
	}
	if !cfg.enabled {
		t.Fatal("enabled = false, want true")
	}
	if cfg.identityMode != plusIdentityRetry {
		t.Fatalf("identityMode = %q, want %q", cfg.identityMode, plusIdentityRetry)
	}
}

func TestParsePlusConfigSupportsExplicitIdentityModes(t *testing.T) {
	for _, mode := range []plusIdentityMode{plusIdentityLegacy, plusIdentityFailClosed, plusIdentityRetry} {
		t.Run(string(mode), func(t *testing.T) {
			cfg, err := parsePlusConfig(map[string]any{
				"plus_enabled":       true,
				"plus_identity_mode": string(mode),
			})
			if err != nil {
				t.Fatalf("parsePlusConfig() error = %v", err)
			}
			if cfg.identityMode != mode {
				t.Fatalf("identityMode = %q, want %q", cfg.identityMode, mode)
			}
		})
	}
}

func TestParsePlusConfigRejectsInvalidOrInactiveIdentityMode(t *testing.T) {
	tests := []struct {
		name string
		opts map[string]any
		want string
	}{
		{
			name: "invalid mode",
			opts: map[string]any{"plus_enabled": true, "plus_identity_mode": "unsafe"},
			want: "invalid plus_identity_mode",
		},
		{
			name: "mode without plus",
			opts: map[string]any{"plus_identity_mode": "retry"},
			want: "requires plus_enabled=true",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parsePlusConfig(tt.opts)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestPlusIdentityPolicyFailsClosedOnlyForProtectedGroupMessages(t *testing.T) {
	p := &Platform{
		plus:          plusConfig{enabled: true, identityMode: plusIdentityRetry},
		groupReplyAll: false,
	}

	if !p.shouldDropForUnknownBotIdentity("group") {
		t.Fatal("unknown bot identity should drop protected group messages")
	}
	if p.shouldDropForUnknownBotIdentity("p2p") {
		t.Fatal("unknown bot identity must not block direct messages")
	}

	p.groupReplyAll = true
	if p.shouldDropForUnknownBotIdentity("group") {
		t.Fatal("group_reply_all=true should not require bot mention identity")
	}

	p.groupReplyAll = false
	p.botOpenID = "ou_bot"
	if p.shouldDropForUnknownBotIdentity("group") {
		t.Fatal("resolved bot identity should restore normal mention filtering")
	}
}

func TestPlusIdentityPolicyDropsInboundGroupEventBeforeDispatch(t *testing.T) {
	called := make(chan struct{}, 1)
	p := &Platform{
		platformName:  "feishu",
		plus:          plusConfig{enabled: true, identityMode: plusIdentityRetry},
		allowFrom:     "*",
		allowChat:     "*",
		groupReplyAll: false,
		dedup:         &core.MessageDedup{},
		handler: func(core.Platform, *core.Message) {
			called <- struct{}{}
		},
	}
	chatType := "group"
	messageType := "text"
	content := `{"text":"hello everyone"}`
	createTime := strconv.FormatInt(time.Now().UnixMilli(), 10)
	userID := "ou_user"
	messageID := "om_plus_identity_unknown"
	chatID := "oc_group"

	err := p.onMessage(context.Background(), &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Sender: &larkim.EventSender{SenderId: &larkim.UserId{OpenId: &userID}},
			Message: &larkim.EventMessage{
				MessageId:   &messageID,
				ChatId:      &chatID,
				ChatType:    &chatType,
				MessageType: &messageType,
				Content:     &content,
				CreateTime:  &createTime,
			},
		},
	})
	if err != nil {
		t.Fatalf("onMessage() error = %v", err)
	}

	select {
	case <-called:
		t.Fatal("handler was called while Plus bot identity was unknown")
	case <-time.After(25 * time.Millisecond):
	}
}

func TestPlusIdentityRetryRecoversWithoutRestart(t *testing.T) {
	p := &Platform{
		platformName:         "feishu",
		plus:                 plusConfig{enabled: true, identityMode: plusIdentityRetry},
		identityRetryInitial: time.Millisecond,
		identityRetryMax:     2 * time.Millisecond,
	}
	var calls atomic.Int32
	p.fetchBotOpenIDOverride = func() (string, error) {
		if calls.Add(1) < 2 {
			return "", errors.New("temporary network failure")
		}
		return "ou_recovered", nil
	}

	p.startBotIdentityRetry()
	defer p.stopBotIdentityRetry()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if p.getBotOpenID() == "ou_recovered" {
			if calls.Load() != 2 {
				t.Fatalf("calls = %d, want 2", calls.Load())
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("bot identity did not recover; calls = %d", calls.Load())
}

func TestFetchBotIdentityRejectsEmptySuccessfulResponse(t *testing.T) {
	p := &Platform{
		plus:                   plusConfig{enabled: true, identityMode: plusIdentityRetry},
		fetchBotOpenIDOverride: func() (string, error) { return "   ", nil },
	}
	_, err := p.fetchBotIdentity()
	if err == nil || !strings.Contains(err.Error(), "empty open_id") {
		t.Fatalf("error = %v, want empty open_id", err)
	}
}
