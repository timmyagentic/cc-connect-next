export interface SessionKeyRecord {
  session_key: string;
  active?: boolean;
}

export function selectDefaultWebSession<T extends SessionKeyRecord>(
  sessions: readonly T[],
  webSessionKey: string,
): T | null;

export function loadSelectedSession<T extends SessionKeyRecord & { id: string }, R>(
  projectName: string,
  session: T,
  switchSession: (project: string, sessionKey: string, sessionID: string) => Promise<unknown>,
  getSession: (project: string, sessionID: string, historyLimit: number) => Promise<R>,
): Promise<R>;
