package codex

import "testing"

func TestAutomaticSessionRecoveryCapabilityIsLocalOnly(t *testing.T) {
	for _, tc := range []struct {
		url  string
		want bool
	}{{"", true}, {"wss://remote.example", false}} {
		s := &appServerSession{url: tc.url}
		if got := s.SupportsAutomaticSessionRecovery(); got != tc.want {
			t.Fatalf("url=%q support=%v", tc.url, got)
		}
	}
}
