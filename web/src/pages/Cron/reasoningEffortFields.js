// Preserve untouched legacy values; explicit empty edits restore inheritance.
export function cronReasoningEffortFields(type, effort, previous) {
  const value = (effort || '').trim();
  if (type === 'exec' && value) throw new Error('cron.reasoningEffortPromptOnly');
  if (previous) {
    if (effort === (previous.reasoning_effort || '')) return {};
    return value === (previous.reasoning_effort || '') ? {} : { reasoning_effort: value };
  }
  return value ? { reasoning_effort: value } : {};
}

export function cronReasoningEffortCapability(manifest) {
  const state = manifest?.runtime?.find(adapter => adapter.kind === 'agent')?.capabilities
    ?.find(feature => feature.id === 'per_turn_reasoning_effort')?.availability?.state;
  const values = manifest?.tools?.find(tool => tool.id === 'cron')?.parameters
    ?.find(parameter => parameter.name === 'reasoning_effort')?.allowed_values;
  return {
    state,
    values: state === 'available' && Array.isArray(values)
      ? [...new Set(values.filter(value => typeof value === 'string' && value.trim()))]
      : [],
  };
}
