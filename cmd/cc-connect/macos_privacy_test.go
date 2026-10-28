package main

import (
	"runtime"
	"testing"

	"github.com/timmyagentic/cc-connect-next/config"
	"github.com/timmyagentic/cc-connect-next/internal/privacyguard"
)

func TestMacOSPrivacyBootstrapDefaultsAndExplicitOptOut(t *testing.T) {
	t.Cleanup(func() { _ = privacyguard.Configure(privacyguard.Settings{}) })
	if err := configureMacOSPrivacy(&config.Config{}); err != nil {
		t.Fatal(err)
	}
	if privacyguard.Active() != (runtime.GOOS == "darwin") {
		t.Fatal("omitted configuration must enable privacy only on macOS")
	}
	disabled := false
	if err := configureMacOSPrivacy(&config.Config{
		MacOSPrivacy: config.MacOSPrivacyConfig{Enabled: &disabled},
	}); err != nil {
		t.Fatal(err)
	}
	if privacyguard.Active() {
		t.Fatal("explicit opt-out was ignored")
	}
}
