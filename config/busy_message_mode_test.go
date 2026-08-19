package config

// Tests for queue.busy_message_mode / projects.busy_message_mode (issue #27).

import (
	"strings"
	"testing"
)

func TestNormalizeBusyMessageMode(t *testing.T) {
	cases := []struct {
		raw    string
		want   string
		wantOK bool
	}{
		{"", "", true},
		{"queue", BusyMessageModeQueue, true},
		{"steer", BusyMessageModeSteer, true},
		{" Steer ", BusyMessageModeSteer, true},
		{"QUEUE", BusyMessageModeQueue, true},
		{"inject", "", false},
	}
	for _, tc := range cases {
		got, ok := NormalizeBusyMessageMode(tc.raw)
		if got != tc.want || ok != tc.wantOK {
			t.Fatalf("NormalizeBusyMessageMode(%q) = (%q, %v), want (%q, %v)", tc.raw, got, ok, tc.want, tc.wantOK)
		}
	}
}

func TestResolveBusyMessageMode(t *testing.T) {
	steer := "steer"
	queue := "queue"

	// Default: queue.
	cfg := &Config{}
	proj := &ProjectConfig{}
	if got := cfg.ResolveBusyMessageMode(proj); got != BusyMessageModeQueue {
		t.Fatalf("default mode = %q, want queue", got)
	}

	// Global steer applies to projects without an override.
	cfg.Queue.BusyMessageMode = &steer
	if got := cfg.ResolveBusyMessageMode(proj); got != BusyMessageModeSteer {
		t.Fatalf("global steer = %q, want steer", got)
	}

	// Project override wins over global.
	proj.BusyMessageMode = "queue"
	if got := cfg.ResolveBusyMessageMode(proj); got != BusyMessageModeQueue {
		t.Fatalf("project override = %q, want queue", got)
	}
	proj.BusyMessageMode = "steer"
	cfg.Queue.BusyMessageMode = &queue
	if got := cfg.ResolveBusyMessageMode(proj); got != BusyMessageModeSteer {
		t.Fatalf("project steer over global queue = %q, want steer", got)
	}
}

func TestValidateBusyMessageMode(t *testing.T) {
	bad := "inject"

	cfg := Config{Projects: []ProjectConfig{validProject("demo")}}
	cfg.Queue.BusyMessageMode = &bad
	err := cfg.validate()
	if err == nil || !strings.Contains(err.Error(), `queue.busy_message_mode must be "queue" or "steer"`) {
		t.Fatalf("global validate() = %v, want busy_message_mode error", err)
	}

	cfg = Config{Projects: []ProjectConfig{validProject("demo")}}
	cfg.Projects[0].BusyMessageMode = "inject"
	err = cfg.validate()
	if err == nil || !strings.Contains(err.Error(), `projects[0].busy_message_mode must be "queue" or "steer"`) {
		t.Fatalf("project validate() = %v, want busy_message_mode error", err)
	}

	// Valid values pass.
	steer := "steer"
	cfg = Config{Projects: []ProjectConfig{validProject("demo")}}
	cfg.Queue.BusyMessageMode = &steer
	cfg.Projects[0].BusyMessageMode = "queue"
	if err := cfg.validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
}
