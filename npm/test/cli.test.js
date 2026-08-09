"use strict";

const assert = require("node:assert/strict");
const { spawnSync } = require("node:child_process");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const test = require("node:test");

const cliPath = path.join(__dirname, "..", "cli.js");

function fixture() {
  const home = fs.mkdtempSync(path.join(os.tmpdir(), "feishu-plus-cli-"));
  const configPath = path.join(home, ".cc-connect", "config.toml");
  const binaryPath = path.join(home, "bin", "cc-connect");
  fs.mkdirSync(path.dirname(configPath), { recursive: true });
  fs.mkdirSync(path.dirname(binaryPath), { recursive: true });
  fs.writeFileSync(configPath, 'app_secret = "never-print-this"\n', { mode: 0o600 });
  fs.writeFileSync(binaryPath, "#!/bin/sh\necho cc-connect 1.4.1\n", { mode: 0o755 });
  const env = {
    ...process.env,
    HOME: home,
    PATH: "",
    CC_CONNECT_CONFIG: configPath,
    CC_CONNECT_BINARY: binaryPath,
  };
  return { home, configPath, binaryPath, env };
}

test("install --dry-run reports a ready plan and makes no changes", () => {
  const f = fixture();
  const result = spawnSync(process.execPath, [cliPath, "install", "--dry-run", "--json"], {
    env: f.env,
    encoding: "utf8",
  });

  assert.equal(result.status, 0, result.stderr);
  const plan = JSON.parse(result.stdout);
  assert.equal(plan.ready, true);
  assert.equal(plan.changesApplied, false);
  assert.equal(plan.preserve.config, f.configPath);
  assert.equal(fs.existsSync(plan.target.installRoot), false);
  assert.doesNotMatch(result.stdout, /never-print-this/);
});

test("install without --dry-run refuses before writing", () => {
  const f = fixture();
  const result = spawnSync(process.execPath, [cliPath, "install", "--json"], {
    env: f.env,
    encoding: "utf8",
  });

  assert.equal(result.status, 2);
  assert.match(result.stderr, /No changes were made/);
  const plan = JSON.parse(result.stdout);
  assert.equal(plan.changesApplied, false);
  assert.equal(fs.existsSync(plan.target.installRoot), false);
});
