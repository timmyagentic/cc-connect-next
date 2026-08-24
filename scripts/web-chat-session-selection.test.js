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
