const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

test('Cron reasoning effort inherits by omission, supports model-independent values, and clears explicitly', async () => {
  const { cronReasoningEffortFields: fields } = await import('../web/src/pages/Cron/reasoningEffortFields.js');
  assert.deepEqual(fields('prompt', ''), {});
  assert.deepEqual(fields('prompt', ' effort-a '), { reasoning_effort: 'effort-a' });
  assert.deepEqual(fields('prompt', '', { reasoning_effort: 'effort-a' }), { reasoning_effort: '' });
  assert.deepEqual(fields('prompt', '', {}), {});
  assert.deepEqual(fields('prompt', 'effort-a', { reasoning_effort: 'effort-a' }), {});
});

test('Unchanged legacy reasoning fields are omitted from unrelated patches', async () => {
  const { cronReasoningEffortFields: fields } = await import('../web/src/pages/Cron/reasoningEffortFields.js');
  for (const reasoning_effort of [undefined, '', 'legacy-effort', ' Legacy-Effort ', ' ']) {
    const previous = { reasoning_effort, model: 'legacy-model', timeout_mins: null, session_mode: 'reuse' };
    const patch = { description: 'new description', ...fields('prompt', reasoning_effort || '', previous) };
    assert.deepEqual(patch, { description: 'new description' });
    assert.equal(previous.reasoning_effort, reasoning_effort);
  }
});

test('Switching to shell clears both overrides in the same patch as the prompt and command', async () => {
  const { cronReasoningEffortFields } = await import('../web/src/pages/Cron/reasoningEffortFields.js');
  const { cronModelFields } = await import('../web/src/pages/Cron/modelFields.js');
  const previous = { model: 'model-a', reasoning_effort: 'effort-a', prompt: 'report', exec: '' };
  const patch = {
    ...cronModelFields('exec', '', previous),
    ...cronReasoningEffortFields('exec', '', previous),
    prompt: '', exec: 'echo report',
  };
  assert.deepEqual(patch, { model: '', reasoning_effort: '', prompt: '', exec: 'echo report' });
  assert.throws(() => cronReasoningEffortFields('exec', 'effort-a'), /reasoningEffortPromptOnly/);
  assert.deepEqual(cronReasoningEffortFields('exec', ''), {});
});

test('Reasoning availability uses the Agent feature and its advertised cron values', async () => {
  const { cronReasoningEffortCapability: capability } = await import('../web/src/pages/Cron/reasoningEffortFields.js');
  assert.deepEqual(capability(), { state: undefined, values: [] });
  assert.deepEqual(capability({}), { state: undefined, values: [] });
  const feature = { id: 'per_turn_reasoning_effort', availability: { state: 'available' } };
  const tools = [{ id: 'cron', parameters: [{ name: 'reasoning_effort', allowed_values: ['effort-a', 'effort-b'] }] }];
  assert.deepEqual(capability({ runtime: [{ kind: 'session', capabilities: [feature] }], tools }), { state: undefined, values: [] });
  const manifest = { runtime: [{ kind: 'agent', capabilities: [feature] }], tools };
  assert.deepEqual(capability(manifest), { state: 'available', values: ['effort-a', 'effort-b'] });
  assert.deepEqual(capability({ runtime: manifest.runtime }), { state: 'available', values: [] });
  for (const state of ['unavailable', 'conditional']) {
    assert.deepEqual(capability({ ...manifest, runtime: [{ kind: 'agent', capabilities: [{ ...feature, availability: { state } }] }] }), { state, values: [] });
  }
  const values = ['', 'effort-a', 'effort-a', ' ', null, 'effort-b'];
  const result = capability({ ...manifest, tools: [{ id: 'cron', parameters: [{ name: 'reasoning_effort', allowed_values: values }] }] });
  assert.deepEqual(result.values, ['effort-a', 'effort-b']);
  assert.equal(values.length, 6);
});

test('Reasoning effort translations cover every shipped Web locale', () => {
  const directory = path.join(__dirname, '../web/src/i18n/locales');
  for (const file of fs.readdirSync(directory).filter(name => name.endsWith('.json'))) {
    const locale = JSON.parse(fs.readFileSync(path.join(directory, file), 'utf8'));
    for (const key of ['reasoningEffort', 'reasoningEffortInherit', 'reasoningEffortHint', 'reasoningEffortPromptOnly', 'reasoningEffortUnsupported']) {
      assert.ok(locale.cron[key], `${file} missing cron.${key}`);
    }
  }
});

test('Cron editor fetches runtime and tool metadata and sends effort in the atomic patch', () => {
  const api = fs.readFileSync(path.join(__dirname, '../web/src/api/cron.ts'), 'utf8');
  const page = fs.readFileSync(path.join(__dirname, '../web/src/pages/Cron/CronList.tsx'), 'utf8');
  assert.match(api, /sections: 'runtime,tools'/);
  assert.match(api, /reasoning_effort\?: string/);
  assert.match(page, /getCronCapabilities\(form.project\)/);
  assert.match(page, /const updates: Record<string, any> = \{ \.\.\.modelFields, \.\.\.reasoningFields \}/);
  assert.match(page, /await updateCronJob\(editJob.id, updates\)/);
  assert.match(page, /_type: 'exec' as const, model: '', reasoning_effort: ''/);
  assert.match(page, /delete body.reasoning_effort/);
  assert.match(page, /reasoningCapability.values.length > 0/);
  assert.match(page, /job.reasoning_effort \|\| t\('cron.reasoningEffortInherit'\)/);
});
