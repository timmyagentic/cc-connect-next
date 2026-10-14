package feishu

import (
	"fmt"
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/timmyagentic/cc-connect-next/core"
)

func TestUnreadReminderConfiguredDelayAndRestart(t *testing.T) {
	for _, tc := range []struct {
		name  string
		opts  map[string]any
		delay time.Duration
	}{
		{"default", nil, time.Hour},
		{"shorter", map[string]any{"unread_reminder_delay_mins": int64(30)}, 30 * time.Minute},
		{"JSON integer", map[string]any{"unread_reminder_delay_mins": 30.0}, 30 * time.Minute},
		{"longer", map[string]any{"unread_reminder_delay_mins": 120}, 2 * time.Hour},
		{"minimum", map[string]any{"unread_reminder_delay_mins": 1}, time.Minute},
		{"maximum", map[string]any{"unread_reminder_delay_mins": 10079}, 10079 * time.Minute},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newReminderFixture(t, tc.opts)
			if err := f.notice("om_trigger", "omt_topic"); err != nil {
				t.Fatal(err)
			}
			if err := f.p.startUnreadReminders(); err != nil {
				t.Fatalf("cannot restore configured deadline: %v", err)
			}
			f.p.stopUnreadReminders()
			// Use a fresh queue owner to model a restart with a new configuration.
			raw, err := New(map[string]any{
				"app_id": f.p.appID, "app_secret": "test", "domain": f.p.domain,
				"enable_feishu_card": false, "unread_reminder": true,
				"unread_reminder_users": []string{"ou_owner"}, "unread_reminder_delay_mins": 180,
				"cc_data_dir": filepath.Dir(filepath.Dir(f.p.reminders.path)), "cc_project": "project",
			})
			if err != nil {
				t.Fatal(err)
			}
			f.p = raw.(*Platform)
			f.p.reminders.now = func() time.Time { return f.now }
			f.now = f.now.Add(tc.delay - time.Second)
			if err := f.p.startUnreadReminders(); err != nil {
				t.Fatal(err)
			}
			// Stop the worker before advancing the fixture clock; explicit ticks
			// below exercise the loaded records without a concurrent clock read.
			f.p.stopUnreadReminders()
			f.p.reminders.stopped = false
			f.tick()
			if f.private != 0 {
				t.Fatal("reminder sent before its persisted deadline")
			}
			f.now = f.now.Add(time.Second)
			f.tick()
			if f.private != 1 {
				t.Fatalf("configured/persisted delay %v ignored: private sends=%d", tc.delay, f.private)
			}
		})
	}
}

func TestUnreadReminderDelayRejectsInvalidValues(t *testing.T) {
	for _, value := range []any{0, -1, 10080, int64(1 << 62), "30", true, 1.5, math.NaN(), math.Inf(1), nil} {
		t.Run(fmt.Sprintf("%T_%v", value, value), func(t *testing.T) {
			opts := map[string]any{"app_id": "cli_test", "app_secret": "test", "unread_reminder_delay_mins": value}
			if err := core.ValidatePlatformOptions("feishu", opts); err == nil {
				t.Error("side-effect-free validation accepted invalid delay")
			}
			if _, err := New(opts); err == nil {
				t.Error("constructor accepted invalid delay")
			}
		})
	}
}
