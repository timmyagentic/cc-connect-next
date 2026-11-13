package core

import (
	"errors"
	"os"
	"testing"
)

func TestCronServiceTierRejectsInvalidCatalogBeforePersistence(t *testing.T) {
	s := newCronReasoningScheduler(t, &cronPriorityAgent{&cronModelAgent{}})
	j := cronReasoningStorageJob("tier", "")
	j.ServiceTier = "fast" // Display names are not backend IDs.
	if err := s.AddJob(j); err == nil {
		t.Fatal("invalid catalog ID accepted before persistence")
	}
	if s.store.Snapshot(j.ID) != nil {
		t.Fatal("invalid job persisted")
	}
	j.ServiceTier = "priority"
	if err := s.AddJob(j); err != nil {
		t.Fatal(err)
	}
	before := s.store.Snapshot(j.ID)
	if err := s.UpdateJobFields(j.ID, map[string]any{"service_tier": "fast", "description": "must not persist"}); err == nil {
		t.Fatal("invalid edit accepted")
	}
	if got := s.store.Snapshot(j.ID); got.ServiceTier != before.ServiceTier || got.Description != before.Description {
		t.Fatalf("invalid edit persisted: %+v", got)
	}
}

func TestCronServiceTierEditNormalizesAndRollsBackSaveFailure(t *testing.T) {
	s := newCronReasoningScheduler(t, &cronPriorityAgent{&cronModelAgent{}})
	j := cronReasoningStorageJob("tier", "")
	if err := s.AddJob(j); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateJob(j.ID, "service_tier", " priority "); err != nil {
		t.Fatal(err)
	}
	snapshot := s.store.Snapshot(j.ID)
	if snapshot.ServiceTier != "priority" {
		t.Fatalf("tier not normalized: %q", snapshot.ServiceTier)
	}
	// A directory cannot be replaced by an atomic file rename.
	s.store.path = t.TempDir()
	if err := s.UpdateJob(j.ID, "service_tier", ""); err == nil {
		t.Fatal("failed persistence accepted")
	}
	if got := s.store.Snapshot(j.ID); got.ServiceTier != "priority" {
		t.Fatalf("failed save changed memory: %+v", got)
	}
}

type cronBrokenTierCatalog struct{ *cronModelAgent }

func (*cronBrokenTierCatalog) ServiceTierCapabilities(string) (ServiceTierCapabilities, error) {
	return ServiceTierCapabilities{}, errors.New("catalog unavailable")
}
func TestCronServiceTierUnavailableCatalogFailsClosed(t *testing.T) {
	for _, a := range []Agent{&cronBrokenTierCatalog{&cronModelAgent{}}} {
		s := newCronReasoningScheduler(t, a)
		j := cronReasoningStorageJob("tier", "")
		j.ServiceTier = "priority"
		if err := s.AddJob(j); err == nil {
			t.Fatal("unavailable catalog accepted")
		}
		if _, err := os.Stat(s.store.path); err == nil {
			t.Fatal("rejected job created persistent state")
		}
	}
}

type cronModelSpecificTierAgent struct{ *cronModelAgent }

func (*cronModelSpecificTierAgent) ServiceTierCapabilities(model string) (ServiceTierCapabilities, error) {
	caps := ServiceTierCapabilities{Model: model}
	if model == "chat-model" {
		caps.Options = []ServiceTierOption{{Name: "Fast", Value: "priority"}}
	}
	return caps, nil
}
func TestCronServiceTierValidationUsesChatModelBeforeProjectDefault(t *testing.T) {
	a := &cronModelSpecificTierAgent{&cronModelAgent{model: "project-model"}}
	s := newCronReasoningScheduler(t, a)
	e := s.engines["p"]
	e.SetProjectStateStore(NewProjectStateStore(t.TempDir() + "/project.json"))
	if err := e.sessions.UpdateChatKey("test:chat", "test:group"); err != nil {
		t.Fatal(err)
	}
	if err := e.projectState.SaveChatModelBinding("test:group", ChatModelBinding{Model: "chat-model", Provider: modelProvider(a)}); err != nil {
		t.Fatal(err)
	}
	j := cronReasoningStorageJob("chat-tier", "")
	j.ServiceTier = "priority"
	if err := s.AddJob(j); err != nil {
		t.Fatalf("inherited chat model rejected: %v", err)
	}
	if err := s.UpdateJob(j.ID, "model", "project-model"); err == nil {
		t.Fatal("explicit model should override chat model validation")
	}
}

func TestCronServiceTierRegistrationDefersSessionOnlyCatalog(t *testing.T) {
	s := newCronReasoningScheduler(t, &stubAgent{})
	j := cronReasoningStorageJob("session-catalog", "")
	j.ServiceTier = "priority"
	if err := s.AddJob(j); err != nil {
		t.Fatalf("registration must defer session capability: %v", err)
	}
}
