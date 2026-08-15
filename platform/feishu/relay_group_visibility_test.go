package feishu

import "testing"

func TestPlatformRelayGroupVisibilityKey(t *testing.T) {
	p := &Platform{platformName: "feishu"}
	cases := []struct {
		name       string
		sessionKey string
		wantKey    string
		wantOK     bool
	}{
		{name: "root", sessionKey: "feishu:oc_chat:root:om_msg", wantKey: "feishu:oc_chat:root:om_msg", wantOK: true},
		{name: "thread", sessionKey: "feishu:oc_chat:thread:omt_msg", wantKey: "feishu:oc_chat:thread:omt_msg", wantOK: true},
		{name: "lark root", sessionKey: "lark:oc_chat:root:om_msg", wantKey: "lark:oc_chat:root:om_msg", wantOK: true},
		{name: "direct", sessionKey: "feishu:oc_chat:ou_user"},
		{name: "empty root", sessionKey: "feishu:oc_chat:root:"},
		{name: "foreign platform", sessionKey: "slack:C123:root:fake"},
		{name: "wrong configured platform", sessionKey: "lark:oc_chat:root:om_msg", wantOK: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "lark root" {
				p.platformName = "lark"
			} else {
				p.platformName = "feishu"
			}
			gotKey, gotOK := p.RelayGroupVisibilityKey(tc.sessionKey)
			if gotKey != tc.wantKey || gotOK != tc.wantOK {
				t.Fatalf("RelayGroupVisibilityKey(%q) = (%q, %v), want (%q, %v)", tc.sessionKey, gotKey, gotOK, tc.wantKey, tc.wantOK)
			}
		})
	}
}
