package updatechannel

import "testing"

func TestParseDefaultsToStableAndAcceptsOnlyExplicitChannels(t *testing.T) {
	for _, test := range []struct {
		raw  string
		want Channel
		ok   bool
	}{
		{raw: "", want: Stable, ok: true},
		{raw: " stable ", want: Stable, ok: true},
		{raw: "BETA", want: Beta, ok: true},
		{raw: "prerelease", ok: false},
	} {
		got, ok := Parse(test.raw)
		if got != test.want || ok != test.ok {
			t.Fatalf("Parse(%q) = %q, %t; want %q, %t", test.raw, got, ok, test.want, test.ok)
		}
	}
}

func TestChannelReleaseTypeCopyNeverCallsAPrereleaseStable(t *testing.T) {
	if got := Stable.ReleaseType(false); got != "stable" {
		t.Fatalf("stable release type = %q", got)
	}
	if got := Beta.ReleaseType(true); got != "prerelease" {
		t.Fatalf("beta release type = %q", got)
	}
	if got := Stable.ReleaseType(true); got != "prerelease" {
		t.Fatalf("prerelease on stable discovery was mislabeled %q", got)
	}
}
