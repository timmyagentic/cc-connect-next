package codex

// SupportsAutomaticSessionRecovery exposes transport eligibility only. It does
// not read local history or promise that a particular failed turn is recoverable.
func (s *appServerSession) SupportsAutomaticSessionRecovery() bool {
	return localAppServer(s.url)
}
