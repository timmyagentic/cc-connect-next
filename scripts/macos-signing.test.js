"use strict";

const assert = require("node:assert/strict");
const { execFileSync, spawnSync } = require("node:child_process");
const crypto = require("node:crypto");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const test = require("node:test");
const { tag } = require("./check-release-version.js").readReleaseVersion();

test("npm installs the exact signed release bytes without changing identity", {
  skip: process.platform !== "darwin" || !process.env.CCN_TEST_MACOS_RELEASE_DIR,
}, (t) => {
  const { verifyMacOSBinary } = require("./macos-signing.js");
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "ccn-npm-signed-"));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const arch = process.arch === "arm64" ? "arm64" : "amd64";
  const binaryName = `cc-connect-next-${tag}-darwin-${arch}`;
  const dist = path.resolve(process.env.CCN_TEST_MACOS_RELEASE_DIR);
  const packageDir = path.join(root, "package");
  fs.mkdirSync(packageDir);
  for (const file of ["install.js", "package.json"]) {
    fs.copyFileSync(path.join(__dirname, "..", "npm", file), path.join(packageDir, file));
  }
  // Replace only the external HTTP boundary; run the shipped installer unchanged.
  const preload = path.join(root, "local-downloads.cjs");
  fs.writeFileSync(preload, `
    const fs = require("node:fs"), path = require("node:path");
    const { Readable } = require("node:stream"), { EventEmitter } = require("node:events");
    require("node:https").get = (url, options, callback) => {
      const name = path.basename(new URL(url).pathname);
      if (!["checksums.txt", ${JSON.stringify(binaryName + ".tar.gz")}].includes(name)) throw Error("Unexpected fixture URL");
      const response = Readable.from([fs.readFileSync(path.join(${JSON.stringify(dist)}, name))]);
      response.statusCode = 200; response.headers = {};
      setImmediate(() => callback(response));
      return new EventEmitter();
    };
  `);
  execFileSync(process.execPath, ["--require", preload, path.join(packageDir, "install.js")], { stdio: "pipe" });
  const installed = path.join(packageDir, "bin", "cc-connect-next");
  assert.deepEqual(fs.readFileSync(installed), fs.readFileSync(path.join(dist, binaryName)));
  verifyMacOSBinary(installed);
});

test("Issue166 release signing refuses missing and ad hoc identities", () => {
  for (const identity of ["", "-"]) {
    const result = spawnSync(process.execPath, [path.join(__dirname, "sign-macos-release.js")], {
      encoding: "utf8", env: { ...process.env, MACOS_SIGN_IDENTITY: identity },
    });
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /Developer ID Application/);
  }
});

test("temporary signing keychain is cleaned on success and import failure", (t) => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "ccn-keychain-test-"));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const bin = path.join(root, "tools");
  fs.mkdirSync(bin);
  fs.writeFileSync(path.join(bin, "uname"), "#!/bin/sh\necho Darwin\n", { mode: 0o755 });
  fs.writeFileSync(path.join(bin, "security"), '#!/bin/sh\nprintf "%s\\n" "$1" >> "$CCN_TEST_SECURITY_LOG"\nif [ "$1" = import ] && [ "$CCN_TEST_IMPORT_FAIL" = 1 ]; then exit 1; fi\n', { mode: 0o755 });
  for (const failImport of ["0", "1"]) {
    const log = path.join(root, `security-${failImport}.log`);
    const env = { ...process.env, PATH: `${bin}${path.delimiter}${process.env.PATH}`, RUNNER_TEMP: root,
      CCN_TEST_SECURITY_LOG: log, CCN_TEST_IMPORT_FAIL: failImport,
      MACOS_CERTIFICATE_P12_BASE64: Buffer.from("fixture certificate").toString("base64"),
      MACOS_CERTIFICATE_PASSWORD: "fixture password", MACOS_SIGN_IDENTITY: "fixture identity" };
    const command = "if (process.env.MACOS_CERTIFICATE_P12_BASE64 || process.env.MACOS_CERTIFICATE_PASSWORD) process.exit(2); if (!process.env.MACOS_SIGN_KEYCHAIN) process.exit(3)";
    const result = spawnSync("bash", [path.join(__dirname, "with-macos-signing-keychain.sh"), process.execPath, "-e", command], { env, encoding: "utf8" });
    assert.equal(result.status, failImport === "1" ? 1 : 0, result.stderr);
    assert.match(fs.readFileSync(log, "utf8"), /delete-keychain/);
    assert.deepEqual(fs.readdirSync(root).filter((name) => name.startsWith("ccn-signing.")), []);
    assert.doesNotMatch(result.stdout + result.stderr, /fixture certificate|fixture password/);
  }
});

test("Developer ID builds preserve identity and reject tampering", {
  skip: process.platform !== "darwin" || !process.env.CCN_TEST_MACOS_SIGN_IDENTITY,
}, (t) => {
  const { identifier, codesign, verifyMacOSBinary, verifyCompatibleMacOSBinaries } = require("./macos-signing.js");
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "ccn-signed-builds-"));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const binaries = ["A", "B"].map((version) => {
    const binary = path.join(root, version);
    execFileSync("cc", ["-x", "c", "-o", binary, "-"], {
      input: `#include <stdio.h>\nint main(void) { puts("version ${version}"); return 0; }\n`,
      stdio: ["pipe", "pipe", "pipe"],
    });
    codesign(["--force", "--sign", process.env.CCN_TEST_MACOS_SIGN_IDENTITY, "--identifier", identifier, "--timestamp", "--options", "runtime", binary]);
    return binary;
  });
  const [a, b] = verifyCompatibleMacOSBinaries(...binaries);
  assert.notEqual(a.cdhash, b.cdhash, "the test must cover different executable code hashes");
  assert.equal(execFileSync(binaries[0], { encoding: "utf8" }).trim(), "version A");
  assert.equal(execFileSync(binaries[1], { encoding: "utf8" }).trim(), "version B");
  // A rename (as used by installers) preserves the signed bytes and identity.
  const installed = path.join(root, "cc-connect-next");
  fs.renameSync(binaries[1], installed);
  verifyCompatibleMacOSBinaries(binaries[0], installed);
  const bytes = fs.readFileSync(installed);
  bytes[4096] ^= 1;
  fs.writeFileSync(installed, bytes);
  assert.throws(() => verifyMacOSBinary(installed), /signature/);
});

test("Issue166 release gate rejects checksum-valid ad hoc macOS archives", {
  skip: process.platform !== "darwin",
}, (t) => {
  const dist = fs.mkdtempSync(path.join(os.tmpdir(), "ccn-issue166-"));
  t.after(() => fs.rmSync(dist, { recursive: true, force: true }));
  const manifest = [];
  for (const platform of ["linux", "darwin", "windows"]) {
    for (const arch of ["amd64", "arm64"]) {
      const binary = `cc-connect-next-${tag}-${platform}-${arch}${platform === "windows" ? ".exe" : ""}`;
      const binaryPath = path.join(dist, binary);
      if (platform === "darwin") {
        fs.copyFileSync("/usr/bin/true", binaryPath);
        execFileSync("codesign", ["--force", "--sign", "-", "--identifier", "a.out", binaryPath], { stdio: "pipe" });
      } else {
        fs.writeFileSync(binaryPath, "fixture", { mode: 0o755 });
      }
      const archive = `${binary.replace(/\.exe$/, "")}${platform === "windows" ? ".zip" : ".tar.gz"}`;
      if (platform === "windows") {
        execFileSync("zip", ["-q", archive, binary], { cwd: dist });
      } else {
        execFileSync("tar", ["czf", archive, binary], { cwd: dist });
      }
      const hash = crypto.createHash("sha256").update(fs.readFileSync(path.join(dist, archive))).digest("hex");
      manifest.push(`${hash}  ${archive}`);
    }
  }
  fs.writeFileSync(path.join(dist, "checksums.txt"), manifest.join("\n") + "\n");
  const result = spawnSync(process.execPath, [path.join(__dirname, "check-release-assets.js"), dist, "--require-macos-signatures"], { encoding: "utf8" });
  assert.notEqual(result.status, 0, "unsigned/ad hoc archives must never pass the publication gate");
  assert.match(result.stderr, /signature|Developer ID|ad hoc/i);
});
