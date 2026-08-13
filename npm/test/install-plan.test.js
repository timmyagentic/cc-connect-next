"use strict";

const assert = require("node:assert/strict");
const path = require("node:path");
const test = require("node:test");

const { buildInstallPlan } = require("../lib/install-plan");

test("buildInstallPlan is non-mutating and preserves native data", () => {
  const home = path.join(path.sep, "tmp", "feishu-plus-home");
  const environment = {
    platform: "darwin",
    arch: "arm64",
    supported: true,
    config: {
      path: path.join(home, ".cc-connect", "config.toml"),
      exists: true,
      plusEnabled: false,
    },
    binary: { path: path.join(home, "bin", "cc-connect"), version: "cc-connect 1.4.1" },
    service: { path: path.join(home, "Library", "LaunchAgents", "com.cc-connect.service.plist"), exists: true },
    paths: {
      installRoot: path.join(home, ".local", "share", "cc-connect-feishu-plus"),
    },
  };

  const plan = buildInstallPlan(environment, { channel: "stable", dryRun: true });

  assert.equal(plan.dryRun, true);
  assert.equal(plan.changesApplied, false);
  assert.equal(plan.preserve.config, environment.config.path);
  assert.equal(plan.preserve.dataDirectory, path.join(home, ".cc-connect"));
  assert.deepEqual(
    plan.steps.map((step) => step.id),
    ["preflight", "download", "verify", "install", "configure", "activate", "health-check"],
  );
});
