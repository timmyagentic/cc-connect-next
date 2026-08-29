// Code generated from the compiled configuration catalog. DO NOT EDIT.

export const globalSettingsContract = {
  language: { defaultValue: "zh", allowedValues: ["zh","en","zh-TW","ja","es","auto"] },
  attachmentSend: { defaultValue: "on", allowedValues: ["on","off"] },
  logLevel: { defaultValue: "info", allowedValues: ["debug","info","warn","error"] },
  idleTimeoutMins: { defaultValue: 120 },
  thinkingMessages: { defaultValue: false },
  thinkingMaxLen: { defaultValue: 300 },
  toolMessages: { defaultValue: false },
  toolMaxLen: { defaultValue: 500 },
  streamPreviewEnabled: { defaultValue: true },
  streamPreviewIntervalMs: { defaultValue: 1500 },
  rateLimitMaxMessages: { defaultValue: 20 },
  rateLimitWindowSecs: { defaultValue: 60 },
} as const;
