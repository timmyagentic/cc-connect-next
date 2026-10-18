package core

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestParseDoctorProcess_PreservesExecutableWithSpaces(t *testing.T) {
	ppid, path, err := parseDoctorProcess(" 123 /Applications/Some App.app/Contents/MacOS/tool\n")
	if err != nil || ppid != 123 || path != "/Applications/Some App.app/Contents/MacOS/tool" {
		t.Fatalf("%d %q %v", ppid, path, err)
	}
	for _, malformed := range []string{"", "123", "x /bin/tool", "-1 /bin/tool", "0 ", "1 /bin/a\n2 /bin/b"} {
		if _, _, err := parseDoctorProcess(malformed); err == nil {
			t.Errorf("accepted %q", malformed)
		}
	}
}

func TestDoctorProcessChecks_ReportsAncestryWithoutClaimingPermissions(t *testing.T) {
	checks := doctorProcessChecks(context.Background(), 123, func(_ context.Context, pid int) (int, string, error) {
		if pid == 123 {
			return 1, "/bin/diagnostic", nil
		}
		return 0, "/sbin/launchd", nil
	})
	if checks[0].Status != DoctorPass || !strings.Contains(checks[0].Detail, "PID=123 PPID=1 executable=/bin/diagnostic") || !strings.Contains(checks[0].Detail, "PID=1 PPID=0 executable=/sbin/launchd") {
		t.Fatalf("%+v", checks)
	}
	if checks[1].Status != DoctorWarn || checks[1].DetailKey != MsgDoctorPrivacyUnknown {
		t.Fatalf("must not imply permission denial or grant: %+v", checks)
	}
	rendered := FormatDoctorResults(checks, NewI18n(LangEnglish))
	for _, want := range []string{"Screen Recording: unknown", "Accessibility: unknown", "Apple Events: unknown", "target application", "not the TCC-responsible"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("missing %q: %s", want, rendered)
		}
	}
}

func TestDoctorProcessChecks_UnavailableAncestryIsUnknownAndRedactsErrors(t *testing.T) {
	checks := doctorProcessChecks(context.Background(), 123, func(context.Context, int) (int, string, error) {
		return 0, "", errors.New("secret command argument")
	})
	if checks[0].Status != DoctorWarn || checks[0].DetailKey != MsgDoctorProcessIncomplete {
		t.Fatalf("%+v", checks)
	}
	rendered := FormatDoctorResults(checks, NewI18n(LangEnglish))
	if strings.Contains(rendered, "secret") || !strings.Contains(rendered, "unknown") {
		t.Fatal(rendered)
	}
}

func TestDoctorProcessChecks_BoundedAndCancellable(t *testing.T) {
	for _, tc := range []struct {
		name   string
		cycle  bool
		cancel bool
		calls  int
	}{
		{"depth", false, false, 8}, {"cycle", true, false, 1}, {"cancelled", false, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.cancel {
				cancel()
			}
			calls := 0
			checks := doctorProcessChecks(ctx, 123, func(_ context.Context, pid int) (int, string, error) {
				calls++
				if !tc.cycle {
					pid++
				}
				return pid, "/bin/tool", nil
			})
			if calls != tc.calls || checks[0].Status != DoctorWarn {
				t.Fatalf("calls=%d checks=%+v", calls, checks)
			}
		})
	}
}

func TestDoctorProcessChecks_LocalizedInAllSupportedLanguages(t *testing.T) {
	for _, lang := range []Language{LangEnglish, LangChinese, LangTraditionalChinese, LangJapanese, LangSpanish} {
		i18n := NewI18n(lang)
		for _, key := range []MsgKey{MsgDoctorProcessName, MsgDoctorPrivacyName, MsgDoctorProcessIncomplete, MsgDoctorPrivacyUnknown} {
			if i18n.T(key) == "" || i18n.T(key) == string(key) {
				t.Errorf("missing %s in %s", key, lang)
			}
		}
	}
}
