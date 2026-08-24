/**
 * Pick the persistent session used by the default Web chat transport.
 * External IM sessions remain available in the drawer, but must never be
 * rendered as if they were the Web session unless the user selects one.
 *
 * @template {{ session_key: string }} T
 * @param {readonly T[]} sessions
 * @param {string} webSessionKey
 * @returns {T | null}
 */
export function selectDefaultWebSession(sessions, webSessionKey) {
  return sessions.find((session) => session.session_key === webSessionKey) ?? null;
}
