#!/usr/bin/env node
"use strict";

const fs = require("node:fs");
const path = require("node:path");
const { tag } = require("./check-release-version.js").readReleaseVersion();
const { identifier, codesign, verifyCompatibleMacOSBinaries } = require("./macos-signing.js");

function main() {
  const identity = process.env.MACOS_SIGN_IDENTITY?.trim();
  if (!identity || identity === "-") {
    throw new Error("MACOS_SIGN_IDENTITY must select a Developer ID Application identity; ad hoc releases are forbidden");
  }
  const dist = path.resolve(process.argv[2] || "dist");
  const binaries = ["amd64", "arm64"].map((arch) => path.join(dist, `cc-connect-next-${tag}-darwin-${arch}`));
  for (const binary of binaries) {
    const info = fs.lstatSync(binary);
    if (!info.isFile() || info.isSymbolicLink()) {
      throw new Error("macOS release binary must be a regular file");
    }
    fs.chmodSync(binary, 0o755);
    const args = ["--force", "--sign", identity, "--identifier", identifier, "--timestamp", "--options", "runtime"];
    if (process.env.MACOS_SIGN_KEYCHAIN) args.push("--keychain", process.env.MACOS_SIGN_KEYCHAIN);
    codesign([...args, binary]);
  }
  verifyCompatibleMacOSBinaries(...binaries);
  console.log("[macos-signing] verified both architectures and mutually compatible designated requirements");
}

try { main(); } catch (err) { console.error(`[macos-signing] ${err.message}`); process.exitCode = 1; }
