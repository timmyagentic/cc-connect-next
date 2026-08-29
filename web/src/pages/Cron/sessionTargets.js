/**
 * Collapse session rows into stable cron/timer target options. A missing
 * support flag is treated as supported for compatibility with older servers;
 * if any row for one key is explicitly unsupported, the key is disabled.
 *
 * @template {{ session_key?: string, supports_scheduled_delivery?: boolean, scheduled_delivery_reason?: string }} T
 * @param {readonly T[]} sessions
 * @returns {{ key: string, supported: boolean, reason: string }[]}
 */
export function buildScheduledSessionTargets(sessions) {
  const targets = new Map();
  for (const session of sessions) {
    const key = String(session.session_key || '').trim();
    if (!key) continue;
    const supported = session.supports_scheduled_delivery !== false;
    const reason = String(session.scheduled_delivery_reason || '');
    const existing = targets.get(key);
    if (!existing) {
      targets.set(key, { key, supported, reason });
    } else if (!supported) {
      targets.set(key, { key, supported: false, reason: reason || existing.reason });
    }
  }
  return [...targets.values()];
}
