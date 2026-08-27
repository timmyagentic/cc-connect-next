package feishu

import (
	"context"
	"errors"
	"testing"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func TestGroupMentionFilterFailsClosedWhenBotIdentityUnavailable(t *testing.T) {
	p := &Platform{platformName: "feishu"}
	p.markBotIdentityUnavailable(errors.New("bot info unavailable"))

	chatType := "group"
	msg := &larkim.EventMessage{ChatType: &chatType}
	if p.shouldDispatchGroupMessage(msg, "text", "oc_chat", "feishu:oc_chat:ou_user", false) {
		t.Fatal("group message requiring @bot must be dropped while bot identity is unavailable")
	}
}

func TestGroupReplyAllBypassesIdentityDegradation(t *testing.T) {
	p := &Platform{platformName: "feishu"}
	p.markBotIdentityUnavailable(errors.New("bot info unavailable"))

	chatType := "group"
	msg := &larkim.EventMessage{ChatType: &chatType}
	if !p.shouldDispatchGroupMessage(msg, "text", "oc_chat", "feishu:oc_chat:ou_user", true) {
		t.Fatal("explicit group_reply_all must not depend on bot mention identity")
	}
}

func TestGroupMentionFilterKeepsWebhookCompatibilityBeforeIdentityDiscovery(t *testing.T) {
	p := &Platform{platformName: "feishu"}

	chatType := "group"
	msg := &larkim.EventMessage{ChatType: &chatType}
	if !p.shouldDispatchGroupMessage(msg, "text", "oc_chat", "feishu:oc_chat:ou_user", false) {
		t.Fatal("an unstarted/webhook platform without a recorded identity failure must keep legacy admission")
	}
}

func TestBotIdentityRecoveryClearsPlatformHealth(t *testing.T) {
	p := &Platform{platformName: "feishu"}
	p.markBotIdentityUnavailable(errors.New("temporary proxy failure"))
	if p.ConnectionError() == nil {
		t.Fatal("identity degradation must make platform health unavailable")
	}

	p.botOpenIDFetcher = func() (string, error) { return "ou_recovered", nil }
	if err := p.recoverBotIdentity(context.Background()); err != nil {
		t.Fatalf("recoverBotIdentity() error = %v", err)
	}
	if got := p.getBotOpenID(); got != "ou_recovered" {
		t.Fatalf("botOpenID = %q, want ou_recovered", got)
	}
	if err := p.ConnectionError(); err != nil {
		t.Fatalf("ConnectionError() = %v, want nil after identity recovery", err)
	}
}

func TestBotIdentityHealthSurvivesFreshWebSocketAttempt(t *testing.T) {
	p := &Platform{platformName: "feishu"}
	p.markBotIdentityUnavailable(errors.New("bot info unavailable"))

	// startWebSocketMode clears only the transport error before launching a
	// fresh connection. Identity degradation must remain visible until the bot
	// info call itself succeeds.
	p.recordConnectionError(nil)
	if p.ConnectionError() == nil {
		t.Fatal("fresh WebSocket attempt must not erase unresolved bot identity health")
	}
}
