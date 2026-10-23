import type { CronCapabilities } from '../../api/cron';

export function cronReasoningEffortFields(
  type: 'prompt' | 'exec',
  effort: string,
  previous?: { reasoning_effort?: string },
): { reasoning_effort?: string };

export function cronReasoningEffortCapability(manifest?: CronCapabilities): {
  state: string | undefined;
  values: string[];
};
