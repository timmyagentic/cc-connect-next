const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');

test('backend fields preserve inheritance and untouched settings', async () => {
  const { cronBackendFields: fields } = await import('../web/src/pages/Cron/backendFields.js');
  assert.deepEqual(fields('prompt', ''), {});
  assert.deepEqual(fields('prompt', ' exec '), { backend: 'exec' });
  assert.deepEqual(fields('prompt', 'exec', { backend: 'exec' }), {});
  assert.deepEqual(fields('prompt', '', { backend: 'exec' }), { backend: '' });
  assert.deepEqual(fields('prompt', '', {}), {});
});

test('prompt-to-shell conversion clears backend and model together', async () => {
  const { cronBackendFields } = await import('../web/src/pages/Cron/backendFields.js');
  const { cronModelFields } = await import('../web/src/pages/Cron/modelFields.js');
  const { cronReasoningEffortFields } = await import('../web/src/pages/Cron/reasoningEffortFields.js');
  const previous = { backend: 'exec', model: 'model-a', reasoning_effort: 'low' };
  assert.deepEqual({
    exec: 'echo hi', prompt: '',
    ...cronBackendFields('exec', '', previous),
    ...cronModelFields('exec', '', previous),
    ...cronReasoningEffortFields('exec', '', previous),
  }, { exec: 'echo hi', prompt: '', backend: '', model: '', reasoning_effort: '' });
  assert.throws(() => cronBackendFields('exec', 'exec'), /backendPromptOnly/);
});

test('backend availability follows runtime capability rather than Agent name', async () => {
  const { cronBackendAvailability } = await import('../web/src/pages/Cron/backendFields.js');
  for (const state of ['available', 'unavailable']) {
    const manifest = { runtime: [{ kind: 'agent', capabilities: [{ id: 'cron_backend', availability: { state } }] }] };
    assert.equal(cronBackendAvailability(manifest), state);
  }
  assert.equal(cronBackendAvailability({}), undefined);
});

test('backend controls have translations in every locale', () => {
  const locales = path.join(__dirname, '../web/src/i18n/locales');
  for (const file of fs.readdirSync(locales).filter(file => file.endsWith('.json'))) {
    const values = JSON.parse(fs.readFileSync(path.join(locales, file), 'utf8')).cron;
    for (const key of ['backend', 'backendInherit', 'backendHint', 'backendPromptOnly', 'backendUnsupported']) {
      assert.equal(typeof values[key], 'string', `${file} ${key}`);
      assert.ok(values[key].trim(), `${file} ${key}`);
    }
  }
});
