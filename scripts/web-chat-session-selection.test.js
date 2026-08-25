const test = require('node:test');
const assert = require('node:assert/strict');

test('Web chat selects only its own default session', async () => {
  const { selectDefaultWebSession } = await import('../web/src/pages/Chat/sessionSelection.js');
  const sessions = [
    { id: 'external-newest', session_key: 'feishu:group:user' },
    { id: 'web-older', session_key: 'bridge:web-admin:demo' },
  ];

  assert.equal(selectDefaultWebSession(sessions, 'bridge:web-admin:demo')?.id, 'web-older');
});

test('Web chat starts empty when its own session does not exist', async () => {
  const { selectDefaultWebSession } = await import('../web/src/pages/Chat/sessionSelection.js');
  const sessions = [{ id: 'external', session_key: 'feishu:group:user' }];

  assert.equal(selectDefaultWebSession(sessions, 'bridge:web-admin:demo'), null);
});

test('Web chat reloads the active session when one key has multiple sessions', async () => {
  const { selectDefaultWebSession } = await import('../web/src/pages/Chat/sessionSelection.js');
  const sessions = [
    { id: 'newer-inactive', session_key: 'bridge:web-admin:demo', active: false },
    { id: 'older-active', session_key: 'bridge:web-admin:demo', active: true },
  ];

  assert.equal(selectDefaultWebSession(sessions, 'bridge:web-admin:demo')?.id, 'older-active');
});

test('Web chat activates a drawer selection before loading its history', async () => {
  const { loadSelectedSession } = await import('../web/src/pages/Chat/sessionSelection.js');
  const calls = [];
  const selected = { id: 'older', session_key: 'bridge:web-admin:demo' };

  const detail = await loadSelectedSession(
    'demo',
    selected,
    async (...args) => { calls.push(['switch', ...args]); },
    async (...args) => {
      calls.push(['load', ...args]);
      return { ...selected, history: [] };
    },
  );

  assert.deepEqual(calls, [
    ['switch', 'demo', selected.session_key, selected.id],
    ['load', 'demo', selected.id, 200],
  ]);
  assert.equal(detail.id, selected.id);
});
