"use strict";

const path = require("node:path");

function buildInstallPlan(environment, { channel = "stable", dryRun = false } = {}) {
  const dataDirectory = path.dirname(environment.config.path);
  const steps = [
    { id: "preflight", description: "Validate the supported OS, architecture, native config, and current service." },
    { id: "download", description: `Download the signed ${channel} release asset for ${environment.platform}/${environment.arch}.` },
    { id: "verify", description: "Verify the release manifest and SHA-256 before extraction." },
    { id: "install", description: "Install into an immutable version directory and atomically update the current pointer." },
    { id: "configure", description: "Preserve native settings and enable explicit Feishu Plus feature flags only." },
    { id: "activate", description: "Back up and switch the existing service executable without changing its data directory." },
    { id: "health-check", description: "Verify process, socket, platform readiness, and Feishu connectivity; roll back on failure." },
  ];
  const blockers = [];
  if (!environment.supported) blockers.push(`unsupported platform: ${environment.platform}/${environment.arch}`);
  if (!environment.config.exists) blockers.push(`native config is missing: ${environment.config.path}`);
  if (!environment.binary.path) blockers.push("native cc-connect binary could not be resolved");

  return {
    schemaVersion: 1,
    product: "CC Connect Feishu Plus",
    channel,
    dryRun,
    changesApplied: false,
    ready: blockers.length === 0,
    blockers,
    target: {
      installRoot: environment.paths.installRoot,
      platform: environment.platform,
      arch: environment.arch,
    },
    preserve: {
      config: environment.config.path,
      dataDirectory,
      currentBinary: environment.binary.path,
      currentService: environment.service.path,
    },
    steps,
  };
}

module.exports = { buildInstallPlan };
