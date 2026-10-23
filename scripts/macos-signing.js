"use strict";

const { spawnSync } = require("node:child_process");
const fs = require("node:fs");

// Product identity: keep these stable across versions and architectures.
const identifier = "com.timmyagentic.cc-connect-next";
const teamID = "4356B4HF9R";
const requirement = `anchor apple generic and identifier "${identifier}" and certificate 1[field.1.2.840.113635.100.6.2.6] exists and certificate leaf[field.1.2.840.113635.100.6.1.13] exists and certificate leaf[subject.OU] = "${teamID}"`;

function codesign(args) {
  if (process.platform !== "darwin") {
    throw new Error("macOS signature verification requires a macOS host");
  }
  const result = spawnSync("/usr/bin/codesign", args, { encoding: "utf8", timeout: 120000 });
  if (result.error || result.status !== 0) {
    throw new Error(`macOS signature check failed: ${result.error?.message || result.stderr.trim()}`);
  }
  return `${result.stdout}\n${result.stderr}`;
}

function verifyMacOSBinary(binary) {
  const info = fs.lstatSync(binary);
  if (!info.isFile() || info.isSymbolicLink()) {
    throw new Error("macOS signature target must be a regular file");
  }
  codesign(["--verify", "--strict", "--all-architectures", "-R", `=${requirement}`, binary]);
  const details = codesign(["--display", "--verbose=4", binary]);
  if (!details.includes(`Identifier=${identifier}\n`) || !details.includes(`TeamIdentifier=${teamID}\n`) ||
      !/^Authority=Developer ID Application:/m.test(details) || !/flags=.*\bruntime\b/.test(details) ||
      !/^Timestamp=/m.test(details)) {
    throw new Error("macOS signature must use the product Developer ID, Hardened Runtime, and a secure timestamp");
  }
  const output = codesign(["--display", "-r-", binary]);
  const designated = output.match(/^designated => (.+)$/m)?.[1];
  if (!designated || /\bcdhash\b/.test(designated) || !designated.includes("anchor apple generic") ||
      !designated.includes(identifier) || !designated.includes(teamID)) {
    throw new Error("macOS signature has no stable designated requirement");
  }
  return { identifier, teamID, designated, cdhash: details.match(/^CDHash=(.+)$/m)?.[1] };
}

function verifyCompatibleMacOSBinaries(first, second) {
  const a = verifyMacOSBinary(first);
  const b = verifyMacOSBinary(second);
  codesign(["--verify", "--strict", "--all-architectures", "-R", `=${a.designated}`, second]);
  codesign(["--verify", "--strict", "--all-architectures", "-R", `=${b.designated}`, first]);
  return [a, b];
}

module.exports = { identifier, teamID, codesign, verifyMacOSBinary, verifyCompatibleMacOSBinaries };
