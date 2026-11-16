// Preserve omitted/untouched values; clearing the field restores project inheritance.
export function cronBackendFields(type, backend, previous) {
  const value = (backend || '').trim();
  if (type === 'exec' && value) throw new Error('cron.backendPromptOnly');
  if (previous) {
    if (backend === (previous.backend || '')) return {};
    return value === (previous.backend || '') ? {} : { backend: value };
  }
  return value ? { backend: value } : {};
}

export function cronBackendAvailability(manifest) {
  return manifest?.runtime?.find(adapter => adapter.kind === 'agent')?.capabilities
    ?.find(feature => feature.id === 'cron_backend')?.availability?.state;
}
