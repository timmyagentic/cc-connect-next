export interface ScheduledSessionTarget {
  key: string;
  supported: boolean;
  reason: string;
}

export interface ScheduledSessionRecord {
  session_key?: string;
  supports_scheduled_delivery?: boolean;
  scheduled_delivery_reason?: string;
}

export function buildScheduledSessionTargets(
  sessions: readonly ScheduledSessionRecord[],
): ScheduledSessionTarget[];
