const test = require('node:test');
const assert = require('node:assert/strict');

test('Cron target options disable non-persistent Bridge sessions', async () => {
  const { buildScheduledSessionTargets } = await import('../web/src/pages/Cron/sessionTargets.js');
  const targets = buildScheduledSessionTargets([
    { session_key: 'feishu:group:user', supports_scheduled_delivery: true },
    {
      session_key: 'bridge:web-admin:demo',
      supports_scheduled_delivery: false,
      scheduled_delivery_reason: 'depends on a live adapter',
    },
  ]);

  assert.deepEqual(targets, [
    { key: 'feishu:group:user', supported: true, reason: '' },
    { key: 'bridge:web-admin:demo', supported: false, reason: 'depends on a live adapter' },
  ]);
});

test('Cron target options preserve older servers and let unsupported duplicates win', async () => {
  const { buildScheduledSessionTargets } = await import('../web/src/pages/Cron/sessionTargets.js');
  const targets = buildScheduledSessionTargets([
    { session_key: 'legacy:chat:user' },
    { session_key: 'duplicate:key', supports_scheduled_delivery: true },
    { session_key: 'duplicate:key', supports_scheduled_delivery: false, scheduled_delivery_reason: 'not durable' },
    { session_key: '' },
  ]);

  assert.deepEqual(targets, [
    { key: 'legacy:chat:user', supported: true, reason: '' },
    { key: 'duplicate:key', supported: false, reason: 'not durable' },
  ]);
});
