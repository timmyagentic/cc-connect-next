package core

// SetProviderModel returns a copy of providers with the named provider's model updated.
// The second return value indicates whether a provider matched the given name.
func SetProviderModel(providers []ProviderConfig, name, model string) ([]ProviderConfig, bool) {
	updated := make([]ProviderConfig, len(providers))
	copy(updated, providers)
	for i := range updated {
		if updated[i].Name == name {
			updated[i].Model = model
			return updated, true
		}
	}
	return updated, false
}
