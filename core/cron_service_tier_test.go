package core

import (
	"testing"
)

type cronSpeedAgent struct{ *cronModelAgent }

func (a *cronSpeedAgent) ServiceTierCapabilities(model string) (ServiceTierCapabilities, error) {
	return ServiceTierCapabilities{Model: model, Default: "standard", Options: []ServiceTierOption{{Name: "standard", Value: "standard"}, {Name: "fast", Value: "fast"}}}, nil
}

func TestCronServiceTierPersistenceAndValidation(t *testing.T) {
	path := t.TempDir()
	store, err := NewCronStore(path)
	if err != nil {
		t.Fatal(err)
	}
	job := cronModelJob("speed", "model")
	job.ServiceTier = "fast"
	if err := store.Add(job); err != nil {
		t.Fatal(err)
	}
	loaded, err := NewCronStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.Get(job.ID).ServiceTier; got != "fast" {
		t.Fatalf("stored tier %q", got)
	}
	if !loaded.Update(job.ID, "service_tier", "") {
		t.Fatal("cannot restore inheritance")
	}
	if got := loaded.Get(job.ID).ServiceTier; got != "" {
		t.Fatalf("cleared tier %q", got)
	}
	job.Exec = "echo hi"
	if err := validateCronJob(job); err == nil {
		t.Fatal("shell task accepted service tier")
	}
}

func TestCronServiceTierRejectsUnsupportedValue(t *testing.T) {
	agent := &cronSpeedAgent{&cronModelAgent{model: "project-model"}}
	engine, _ := newCronModelEngine(t, agent, "")
	job := cronModelJob("invalid-tier", "task-model")
	job.ServiceTier = "unsupported"
	if err := engine.ExecuteCronJob(job); err == nil {
		t.Fatal("unsupported tier ran")
	}
	calls, _, _ := agent.snapshot()
	if len(calls) != 0 {
		t.Fatal("unsupported tier reached model")
	}
}
