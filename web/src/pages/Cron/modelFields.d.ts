export function cronModelFields(
  type: 'prompt' | 'exec',
  model: string,
  previous?: { model?: string },
): { model?: string };

export function cronModelAvailability(manifest?: {
  runtime?: { kind: string; capabilities: { id: string; availability: { state: string } }[] }[];
}): string | undefined;
