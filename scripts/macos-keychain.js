"use strict";

const fs = require("node:fs");
const crypto = require("node:crypto");
const { execFileSync } = require("node:child_process");
const { teamID } = require("./macos-signing.js");

const [action, file] = process.argv.slice(2);
function security(args) {
  return execFileSync("security", args, { encoding: "utf8", stdio: ["ignore", "pipe", "pipe"] });
}

try {
  if (action === "save") {
    // security quotes paths; JSON parsing preserves spaces, quotes and backslashes.
    const list = security(["list-keychains", "-d", "user"]).split(/\r?\n/)
      .filter((line) => line.trim()).map((line) => JSON.parse(line.trim()));
    if (!list.every((value) => typeof value === "string")) throw new Error("Invalid keychain search list");
    fs.writeFileSync(file, JSON.stringify(list), { mode: 0o600 });
  } else if (action === "add" || action === "restore") {
    const original = JSON.parse(fs.readFileSync(file, "utf8"));
    const list = action === "add" ? [process.env.MACOS_SIGN_KEYCHAIN, ...original] : original;
    security(["list-keychains", "-d", "user", "-s", ...list]);
  } else if (action === "check-ca") {
    // Public Apple PKI certificate, not a new trust anchor. The system Apple root
    // still validates it. Source: https://www.apple.com/certificateauthority/DeveloperIDG2CA.cer
    const hash = crypto.createHash("sha256").update(fs.readFileSync(file)).digest("hex");
    if (hash !== "f16cd3c54c7f83cea4bf1a3e6a0819c8aaa8e4a1528fd144715f350643d2df3a") {
      throw new Error("Developer ID G2 intermediate certificate does not match Apple PKI");
    }
  } else if (action === "identity") {
    const requested = process.env.MACOS_SIGN_IDENTITY;
    const output = security(["find-identity", "-v", "-p", "codesigning", process.env.MACOS_SIGN_KEYCHAIN]);
    const identities = output.split(/\r?\n/).map((line) => line.match(/^\s*\d+\)\s+([A-Fa-f0-9]{40})\s+"(.+)"\s*$/))
      .filter(Boolean).filter(([, hash, name]) => name.startsWith("Developer ID Application:") && name.endsWith(`(${teamID})`) &&
        (/^[a-f0-9]{40}$/i.test(requested) ? hash.toLowerCase() === requested.toLowerCase() : name.includes(requested)));
    if (identities.length !== 1) {
      // Identity metadata is public; include invalid-identity reasons without
      // exporting a key or dumping keychain contents.
      console.error(security(["find-identity", "-p", "codesigning", process.env.MACOS_SIGN_KEYCHAIN]).trim().slice(0, 4096));
      throw new Error(`Expected one valid Developer ID Application identity for team ${teamID}; found ${identities.length}`);
    }
    process.stdout.write(identities[0][1]);
  } else {
    throw new Error("Unknown macOS keychain operation");
  }
} catch (error) {
  console.error(`[macos-keychain] ${error.message}`);
  process.exitCode = 1;
}
