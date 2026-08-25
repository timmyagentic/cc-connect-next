/**
 * Pick the persistent session used by the default Web chat transport.
 * External IM sessions remain available in the drawer, but must never be
 * rendered as if they were the Web session unless the user selects one.
 *
 * @template {{ session_key: string, active?: boolean }} T
 * @param {readonly T[]} sessions
 * @param {string} webSessionKey
 * @returns {T | null}
 */
export function selectDefaultWebSession(sessions, webSessionKey) {
  const matching = sessions.filter((session) => session.session_key === webSessionKey);
  return matching.find((session) => session.active === true) ?? matching[0] ?? null;
}

/**
 * Make a drawer selection authoritative before loading the history that the
 * user will see. The bridge sends only a session_key, so rendering an inactive
 * session without switching the Engine would send the next prompt elsewhere.
 *
 * @template {{ id: string, session_key: string }} T
 * @template R
 * @param {string} projectName
 * @param {T} session
 * @param {(project: string, sessionKey: string, sessionID: string) => Promise<unknown>} switchSession
 * @param {(project: string, sessionID: string, historyLimit: number) => Promise<R>} getSession
 * @returns {Promise<R>}
 */
export async function loadSelectedSession(projectName, session, switchSession, getSession) {
  await switchSession(projectName, session.session_key, session.id);
  return getSession(projectName, session.id, 200);
}
