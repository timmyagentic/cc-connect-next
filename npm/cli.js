#!/usr/bin/env node

"use strict";

const packageJSON = require("./package.json");
const { inspectEnvironment } = require("./lib/environment");
const { buildInstallPlan } = require("./lib/install-plan");

function parseArgs(argv) {
  const command = argv[0] || "help";
  const options = { json: false, dryRun: false, channel: "stable" };
  for (let index = 1; index < argv.length; index += 1) {
    const arg = argv[index];
    switch (arg) {
      case "--json":
        options.json = true;
        break;
      case "--dry-run":
        options.dryRun = true;
        break;
      case "--config":
        options.config = argv[++index];
        if (!options.config) throw new Error("--config requires a path");
        break;
      case "--binary":
        options.binary = argv[++index];
        if (!options.binary) throw new Error("--binary requires a path");
        break;
      case "--channel":
        options.channel = argv[++index];
        if (!options.channel) throw new Error("--channel requires a value");
        break;
      default:
        throw new Error(`unknown option: ${arg}`);
    }
  }
  return { command, options };
}

function commandEnvironment(options) {
  const env = { ...process.env };
  if (options.config) env.CC_CONNECT_CONFIG = options.config;
  if (options.binary) env.CC_CONNECT_BINARY = options.binary;
  return inspectEnvironment({ env });
}

function printJSON(value) {
  process.stdout.write(`${JSON.stringify(value, null, 2)}\n`);
}

function printDoctor(result) {
  console.log(`CC Connect Feishu Plus doctor: ${result.status}`);
  console.log(`  system:  ${result.platform}/${result.arch}`);
  console.log(`  config:  ${result.config.exists ? result.config.path : "not found"}`);
  console.log(`  binary:  ${result.binary.path || "not found"}`);
  console.log(`  version: ${result.binary.version || "unknown"}`);
  console.log(`  plus:    ${result.config.plusEnabled ? "enabled" : "not enabled"}`);
  for (const warning of result.warnings) console.log(`  warning: ${warning}`);
}

function printPlan(plan) {
  console.log("CC Connect Feishu Plus installation plan");
  console.log(`  mode:    ${plan.dryRun ? "dry-run (no changes)" : "apply"}`);
  console.log(`  target:  ${plan.target.installRoot}`);
  console.log(`  config:  ${plan.preserve.config}`);
  console.log(`  binary:  ${plan.preserve.currentBinary || "not found"}`);
  for (const step of plan.steps) console.log(`  - ${step.id}: ${step.description}`);
  for (const blocker of plan.blockers) console.log(`  blocker: ${blocker}`);
}

function printHelp() {
  console.log(`CC Connect Feishu Plus ${packageJSON.version}

Usage:
  cc-connect-feishu-plus doctor [--json] [--config PATH] [--binary PATH]
  cc-connect-feishu-plus install --dry-run [--json] [--channel stable]
  cc-connect-feishu-plus version

The foundation release intentionally permits installation planning only. It
does not replace or restart the user's current service until signed release
assets and automatic rollback have passed the release gate.`);
}

function main(argv) {
  const { command, options } = parseArgs(argv);
  switch (command) {
    case "doctor": {
      const result = commandEnvironment(options);
      if (options.json) printJSON(result);
      else printDoctor(result);
      return result.supported ? 0 : 1;
    }
    case "install": {
      const environment = commandEnvironment(options);
      const plan = buildInstallPlan(environment, options);
      if (options.json) printJSON(plan);
      else printPlan(plan);
      if (!options.dryRun) {
        console.error("No changes were made: this foundation build requires --dry-run until signed release assets and rollback are enabled.");
        return 2;
      }
      return plan.ready ? 0 : 1;
    }
    case "version":
    case "--version":
    case "-v":
      console.log(packageJSON.version);
      return 0;
    case "help":
    case "--help":
    case "-h":
      printHelp();
      return 0;
    default:
      throw new Error(`unknown command: ${command}`);
  }
}

try {
  process.exitCode = main(process.argv.slice(2));
} catch (error) {
  console.error(`cc-connect-feishu-plus: ${error.message}`);
  process.exitCode = 2;
}
