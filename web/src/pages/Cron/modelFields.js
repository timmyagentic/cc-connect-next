// Omitted on create preserves inheritance; an explicit empty edit clears an override.
export function cronModelFields(type, model, previous) {
  const value = (model || '').trim();
  if (type === 'exec' && value) throw new Error('cron.modelPromptOnly');
  if (previous) return value === (previous.model || '') ? {} : { model: value };
  return value ? { model: value } : {};
}

// Unknown or older servers defer to save-time validation rather than guessing by Agent name.
export function cronModelAvailability(manifest) {
  return manifest?.runtime?.find(adapter => adapter.kind === 'agent')?.capabilities
    ?.find(feature => feature.id === 'per_turn_model')?.availability?.state;
}
