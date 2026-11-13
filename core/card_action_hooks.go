package core

// CardActionHookSource forwards admitted platform card actions to named hooks.
// The handler accepts or rejects dispatch immediately; operation results belong
// to the configured handler. Removing the handler disables this capability.
type CardActionHookSource interface {
	SetCardActionHandler(func(string, HookEvent) error)
}
