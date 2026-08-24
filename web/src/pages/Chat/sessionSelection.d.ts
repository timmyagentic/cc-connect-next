export interface SessionKeyRecord {
  session_key: string;
}

export function selectDefaultWebSession<T extends SessionKeyRecord>(
  sessions: readonly T[],
  webSessionKey: string,
): T | null;
