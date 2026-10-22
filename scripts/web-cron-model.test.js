const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

test('Cron model create inherits by omission and edit clears with an explicit empty string', async () => {
  const { cronModelFields } = await import('../web/src/pages/Cron/modelFields.js');
  assert.deepEqual(cronModelFields('prompt', ''), {});
  assert.deepEqual(cronModelFields('prompt', ' model-a '), { model: 'model-a' });
  assert.deepEqual(cronModelFields('prompt', '', { model: 'model-a' }), { model: '' });
  assert.deepEqual(cronModelFields('prompt', '', {}), {});
  assert.deepEqual(cronModelFields('prompt', 'model-a', { model: 'model-a' }), {});
});

test('Switching to a shell task clears a previous model and cannot submit a conflicting override', async () => {
  const { cronModelFields } = await import('../web/src/pages/Cron/modelFields.js');
  assert.deepEqual(cronModelFields('exec', '', { model: 'model-a' }), { model: '' });
  assert.throws(() => cronModelFields('exec', 'model-a'), /modelPromptOnly/);
});

test('Cron model controls have translations in every shipped Web locale', () => {
  const directory = path.join(__dirname, '../web/src/i18n/locales');
  for (const file of fs.readdirSync(directory).filter(name => name.endsWith('.json'))) {
    const locale = JSON.parse(fs.readFileSync(path.join(directory, file), 'utf8'));
    for (const key of ['model', 'modelInherit', 'modelHint', 'modelPromptOnly', 'modelUnsupported']) {
      assert.ok(locale.cron[key], `${file} missing cron.${key}`);
    }
  }
});

test('Cron model availability comes from the Agent capability, not a backend name or session feature', async () => {
  const { cronModelAvailability } = await import('../web/src/pages/Cron/modelFields.js');
  assert.equal(cronModelAvailability({}), undefined);
  assert.equal(cronModelAvailability({ runtime: [{ kind: 'session', capabilities: [
    { id: 'per_turn_model', availability: { state: 'available' } },
  ] }] }), undefined);
  for (const state of ['available', 'unavailable', 'conditional']) {
    assert.equal(cronModelAvailability({ runtime: [{ kind: 'agent', capabilities: [
      { id: 'per_turn_model', availability: { state } },
    ] }] }), state);
  }
});
