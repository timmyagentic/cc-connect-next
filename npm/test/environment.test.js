"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const test = require("node:test");

const {
  inspectEnvironment,
  resolveRuntimePaths,
} = require("../lib/environment");

test("resolveRuntimePaths keeps the existing cc-connect data directory", () => {
  const home = path.join(path.sep, "tmp", "feishu-plus-home");
  const paths = resolveRuntimePaths({ home, platform: "darwin", env: {} });

  assert.equal(paths.configPath, path.join(home, ".cc-connect", "config.toml"));
  assert.equal(
    paths.installRoot,
    path.join(home, ".local", "share", "cc-connect-feishu-plus"),
  );
  assert.equal(
    paths.servicePath,
    path.join(home, "Library", "LaunchAgents", "com.cc-connect.service.plist"),
  );
});

test("inspectEnvironment discovers the launchd binary without exposing config secrets", () => {
  const home = fs.mkdtempSync(path.join(os.tmpdir(), "feishu-plus-doctor-"));
  const configDir = path.join(home, ".cc-connect");
  const serviceDir = path.join(home, "Library", "LaunchAgents");
  const binaryPath = path.join(home, "bin", "cc-connect");
  fs.mkdirSync(configDir, { recursive: true });
  fs.mkdirSync(serviceDir, { recursive: true });
  fs.mkdirSync(path.dirname(binaryPath), { recursive: true });
  fs.writeFileSync(binaryPath, "#!/bin/sh\necho cc-connect 1.4.1\n", { mode: 0o755 });
  fs.writeFileSync(
    path.join(configDir, "config.toml"),
    'app_secret = "must-not-leak"\nplus_enabled = true\n',
    { mode: 0o600 },
  );
  fs.writeFileSync(
    path.join(serviceDir, "com.cc-connect.service.plist"),
    `<?xml version="1.0"?><plist><dict><key>ProgramArguments</key><array><string>${binaryPath}</string><string>daemon</string></array></dict></plist>`,
  );

  const result = inspectEnvironment({
    home,
    platform: "darwin",
    arch: "arm64",
    env: { PATH: "" },
  });

  assert.equal(result.binary.path, binaryPath);
  assert.match(result.binary.version, /1\.4\.1/);
  assert.equal(result.config.exists, true);
  assert.equal(result.config.plusEnabled, true);
  assert.equal(result.config.mode, "0600");
  assert.doesNotMatch(JSON.stringify(result), /must-not-leak/);
});
