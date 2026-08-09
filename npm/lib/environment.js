"use strict";

const { execFileSync } = require("node:child_process");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");

const SUPPORTED_PLATFORMS = new Set(["darwin", "linux", "win32"]);
const SUPPORTED_ARCHITECTURES = new Set(["arm64", "x64"]);

function resolveRuntimePaths({
  home = os.homedir(),
  platform = process.platform,
  env = process.env,
} = {}) {
  const configPath = env.CC_CONNECT_CONFIG || path.join(home, ".cc-connect", "config.toml");
  let installRoot;
  let servicePath;

  if (platform === "win32") {
    installRoot = path.join(
      env.LOCALAPPDATA || path.join(home, "AppData", "Local"),
      "cc-connect-feishu-plus",
    );
    servicePath = null;
  } else {
    installRoot = path.join(home, ".local", "share", "cc-connect-feishu-plus");
    servicePath = platform === "darwin"
      ? path.join(home, "Library", "LaunchAgents", "com.cc-connect.service.plist")
      : path.join(home, ".config", "systemd", "user", "cc-connect.service");
  }

  return { configPath, installRoot, servicePath };
}

function isFile(filePath) {
  if (!filePath) return false;
  try {
    return fs.statSync(filePath).isFile();
  } catch {
    return false;
  }
}

function decodeXML(value) {
  return value
    .replaceAll("&amp;", "&")
    .replaceAll("&lt;", "<")
    .replaceAll("&gt;", ">")
    .replaceAll("&quot;", '"')
    .replaceAll("&apos;", "'");
}

function binaryFromLaunchAgent(servicePath) {
  if (!isFile(servicePath)) return null;
  let source;
  try {
    source = fs.readFileSync(servicePath, "utf8");
  } catch {
    return null;
  }
  const argsBlock = source.match(
    /<key>\s*ProgramArguments\s*<\/key>\s*<array>([\s\S]*?)<\/array>/i,
  );
  if (!argsBlock) return null;

  const candidates = [];
  const stringPattern = /<string>([\s\S]*?)<\/string>/gi;
  let match;
  while ((match = stringPattern.exec(argsBlock[1])) !== null) {
    candidates.push(decodeXML(match[1].trim()));
  }
  return candidates.find((candidate) => {
    const base = path.basename(candidate).toLowerCase();
    return base.startsWith("cc-connect") && isFile(candidate);
  }) || null;
}

function binaryFromPath(env, platform) {
  const names = platform === "win32" ? ["cc-connect.exe", "cc-connect.cmd"] : ["cc-connect"];
  for (const directory of (env.PATH || "").split(path.delimiter).filter(Boolean)) {
    for (const name of names) {
      const candidate = path.join(directory, name);
      if (isFile(candidate)) return candidate;
    }
  }
  return null;
}

function resolveBinary({ env, platform, servicePath }) {
  if (isFile(env.CC_CONNECT_BINARY)) return env.CC_CONNECT_BINARY;
  return binaryFromPath(env, platform)
    || (platform === "darwin" ? binaryFromLaunchAgent(servicePath) : null);
}

function readBinaryVersion(binaryPath) {
  if (!binaryPath) return null;
  try {
    return execFileSync(binaryPath, ["--version"], {
      encoding: "utf8",
      timeout: 5000,
      stdio: ["ignore", "pipe", "pipe"],
    }).trim();
  } catch {
    return null;
  }
}

function inspectConfig(configPath) {
  if (!isFile(configPath)) {
    return { path: configPath, exists: false, mode: null, plusEnabled: false };
  }
  let mode = null;
  let plusEnabled = false;
  try {
    const stat = fs.statSync(configPath);
    mode = (stat.mode & 0o777).toString(8).padStart(4, "0");
    const source = fs.readFileSync(configPath, "utf8");
    plusEnabled = /^\s*plus_enabled\s*=\s*true\s*(?:#.*)?$/mi.test(source);
  } catch {
    // Existence is still useful; callers report unreadable metadata as null.
  }
  return { path: configPath, exists: true, mode, plusEnabled };
}

function inspectEnvironment({
  home = os.homedir(),
  platform = process.platform,
  arch = process.arch,
  env = process.env,
} = {}) {
  const paths = resolveRuntimePaths({ home, platform, env });
  const supported = SUPPORTED_PLATFORMS.has(platform) && SUPPORTED_ARCHITECTURES.has(arch);
  const binaryPath = resolveBinary({ env, platform, servicePath: paths.servicePath });
  const config = inspectConfig(paths.configPath);
  const service = {
    path: paths.servicePath,
    exists: paths.servicePath ? isFile(paths.servicePath) : false,
  };
  const warnings = [];

  if (!supported) warnings.push(`unsupported platform: ${platform}/${arch}`);
  if (!config.exists) warnings.push(`cc-connect config not found: ${config.path}`);
  if (!binaryPath) warnings.push("cc-connect binary not found in PATH, override, or known service metadata");
  if (config.mode && (Number.parseInt(config.mode, 8) & 0o077) !== 0) {
    warnings.push(`cc-connect config permissions are broader than 0600: ${config.mode}`);
  }

  return {
    product: "CC Connect Feishu Plus",
    platform,
    arch,
    supported,
    paths,
    config,
    service,
    binary: {
      path: binaryPath,
      version: readBinaryVersion(binaryPath),
    },
    status: supported && config.exists && binaryPath ? "ready" : supported ? "warning" : "unsupported",
    warnings,
  };
}

module.exports = {
  inspectEnvironment,
  resolveRuntimePaths,
};
