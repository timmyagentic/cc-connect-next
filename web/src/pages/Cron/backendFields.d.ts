export function cronBackendFields(
  type: 'prompt' | 'exec',
  backend: string,
  previous?: { backend?: string },
): { backend?: string };

export function cronBackendAvailability(manifest?: {
  runtime?: { kind: string; capabilities: { id: string; availability: { state: string } }[] }[];
}): string | undefined;
