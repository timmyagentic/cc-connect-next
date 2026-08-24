package providerstate

import (
	"log/slog"
	"sync"

	"github.com/timmyagentic/cc-connect-next/core"
)

// Store owns the provider list and active selection shared by agent adapters.
// The zero value is ready to use and has no active provider.
type Store struct {
	mu         sync.RWMutex
	label      string
	providers  []core.ProviderConfig
	activeName string
}

func New(label string) Store {
	return Store{label: label}
}

// SetProviders replaces the configured providers while preserving the active
// selection by name. If that provider disappeared, the selection is cleared.
func (s *Store) SetProviders(providers []core.ProviderConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.providers = append([]core.ProviderConfig(nil), providers...)
	if s.activeName != "" && !containsName(s.providers, s.activeName) {
		s.activeName = ""
	}
}

func (s *Store) SetActiveProvider(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if name != "" && !containsName(s.providers, name) {
		return false
	}
	s.activeName = name
	if s.label != "" {
		if name == "" {
			slog.Info(s.label + ": provider cleared")
		} else {
			slog.Info(s.label+": provider switched", "provider", name)
		}
	}
	return true
}

func (s *Store) GetActiveProvider() *core.ProviderConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := range s.providers {
		if s.providers[i].Name == s.activeName && s.activeName != "" {
			provider := s.providers[i]
			return &provider
		}
	}
	return nil
}

func (s *Store) ListProviders() []core.ProviderConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]core.ProviderConfig(nil), s.providers...)
}

func (s *Store) Model(fallback string) string {
	if provider := s.GetActiveProvider(); provider != nil && provider.Model != "" {
		return provider.Model
	}
	return fallback
}

func (s *Store) Models() []core.ModelOption {
	if provider := s.GetActiveProvider(); provider != nil {
		return append([]core.ModelOption(nil), provider.Models...)
	}
	return nil
}

func containsName(providers []core.ProviderConfig, name string) bool {
	for i := range providers {
		if providers[i].Name == name {
			return true
		}
	}
	return false
}
