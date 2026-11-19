"use strict";

const assert = require("node:assert/strict");
const { spawnSync } = require("node:child_process");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");

const { releasePolicyForVersion } = require("./check-release-version.js");
const {
  compareReleaseVersions,
  newestReleaseVersion,
  releaseChannelRelation,
} = require("./release-channel-policy.js");

const workflow = fs.readFileSync(
  path.join(__dirname, "..", ".github", "workflows", "release.yml"),
  "utf8"
);

test("release policy selects GitHub and npm channels from the version", () => {
  assert.deepEqual(releasePolicyForVersion("0.1.0-beta.1"), {
    prerelease: true,
    npmTag: "beta",
  });
  assert.deepEqual(releasePolicyForVersion("0.1.0"), {
    prerelease: false,
    npmTag: "latest",
  });
});

test("release reruns reuse immutable published assets", () => {
  assert.doesNotMatch(workflow, /gh release upload/);
  assert.doesNotMatch(workflow, /--clobber/);
  assert.match(workflow, /gh release download "\$tag"/);
  assert.match(workflow, /check-release-assets\.js "\$published_dir"/);
});

test("release workflow applies the derived channels", () => {
  assert.match(workflow, /id: release_policy/);
  assert.match(workflow, /--github-output/);
  assert.match(workflow, /--prerelease="\$prerelease"/);
  assert.match(workflow, /publish_tag="\$npm_tag"/);
  assert.match(workflow, /--tag "\$publish_tag"/);
  assert.doesNotMatch(workflow, /--tag beta/);
});

test("release channel ordering follows SemVer precedence", () => {
  assert.equal(compareReleaseVersions("0.1.0-beta.2", "0.1.0-beta.1"), 1);
  assert.equal(compareReleaseVersions("0.1.0", "0.1.0-beta.9"), 1);
  assert.equal(compareReleaseVersions("v1.2.3", "1.2.3"), 0);
  assert.equal(releaseChannelRelation("0.1.0-beta.1", "0.1.0-beta.2"), "older");
  assert.equal(releaseChannelRelation("0.1.0-beta.2", "0.1.0-beta.1"), "newer");
  assert.equal(releaseChannelRelation("0.1.0", ""), "missing");
  assert.equal(
    newestReleaseVersion(["v0.1.0", "v0.2.0-beta.3", "v0.1.1"]),
    "0.2.0-beta.3"
  );
});

test("release reruns preserve newer npm tags and GitHub Latest", () => {
  assert.match(
    workflow,
    /\n  publish:\n[\s\S]*?\n    concurrency:\n      group: release-channel-publication\n      cancel-in-progress: false/
  );
  assert.match(workflow, /release-channel-policy\.js --relation/);
  assert.match(workflow, /release-channel-policy\.js --max/);
  assert.match(
    workflow,
    /case "\$tag_relation" in[\s\S]*newer\|missing\)[\s\S]*npm dist-tag add/
  );
  assert.match(
    workflow,
    /if \[ "\$tag_relation" = "older" \]; then[\s\S]*publish_tag="\$staging_tag"/
  );
  assert.match(
    workflow,
    /case "\$latest_relation" in[\s\S]*older\)[\s\S]*latest_flag="--latest=false"/
  );
});

test("idempotent npm reruns remove stale staging tags", () => {
  const stagingAssignment = workflow.indexOf(
    'staging_tag="release-${safe_version}-${GITHUB_RUN_ID}"'
  );
  const publishedCheck = workflow.indexOf(
    'if npm view "cc-connect-next@${version}" version'
  );
  const stagingLookup = workflow.indexOf(
    "npm view cc-connect-next dist-tags --json"
  );
  const stagingRemoval = workflow.indexOf(
    'npm dist-tag rm cc-connect-next "$staging_tag"'
  );

  assert.ok(stagingAssignment >= 0, "workflow must derive a stable per-run staging tag");
  assert.ok(publishedCheck > stagingAssignment, "staging tag must exist before the idempotent published check");
  assert.ok(stagingLookup > publishedCheck, "reruns must query staging tags after reuse or publish");
  assert.ok(stagingRemoval > stagingLookup, "reruns must remove a stale staging tag when present");
});

function runNpmPublication({ existing = false, channel = "1.0.13-beta.4", converged = channel, staging = false } = {}) {
  const step = workflow
    .split("      - name: Publish npm package idempotently\n")[1]
    .split("      - name: Verify the public npm registry\n")[0]
    .split("        run: |\n")[1]
    .replaceAll(/^ {10}/gm, "")
    .replaceAll(/\$\{\{ steps\.release_policy\.outputs\.prerelease \}\}/g, "true");

  return spawnSync("bash", ["-c", `
    waited=false
    sleep() { waited=true; }
    node() {
      if [ "$1" = "-p" ]; then
        echo 1.0.13-beta.5
      else
        command node "$@"
      fi
    }
    npm() {
      case "$1:$2" in
        view:cc-connect-next@1.0.13-beta.5) [ "$EXISTING" = true ] ;;
        view:cc-connect-next@beta)
          if [ "$waited" = true ]; then echo "$CONVERGED"; else echo "$CHANNEL"; fi ;;
        view:cc-connect-next@latest) echo 0.3.6 ;;
        view:cc-connect-next)
          if [ "$STAGING" = true ]; then
            printf '{"release-1-0-13-beta-5-42":"1.0.13-beta.5"}'
          else
            echo '{}'
          fi ;;
        publish:*) echo "npm $*" ;;
        dist-tag:add|dist-tag:rm) echo "$*" ;;
        *) echo "unexpected npm command: $*" >&2; return 2 ;;
      esac
    }
    ${step}
  `], {
    cwd: path.join(__dirname, "..", "npm"),
    encoding: "utf8",
    env: {
      PATH: process.env.PATH,
      NPM_DIST_TAG: "beta",
      NODE_AUTH_TOKEN: "fixture",
      GITHUB_RUN_ID: "42",
      EXISTING: String(existing),
      CHANNEL: channel,
      CONVERGED: converged,
      STAGING: String(staging),
    },
  });
}

test("fresh npm publish leaves channel propagation to the public readback", () => {
  const result = runNpmPublication();
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /npm publish --access public --tag beta --provenance/);
  assert.doesNotMatch(result.stdout, /dist-tag add/);
});

test("npm reruns wait for propagation before attempting channel repair", () => {
  const lagging = runNpmPublication({ existing: true, converged: "1.0.13-beta.5" });
  assert.equal(lagging.status, 0, lagging.stderr);
  assert.doesNotMatch(lagging.stdout, /npm publish|dist-tag add/);

  const stale = runNpmPublication({ existing: true });
  assert.equal(stale.status, 0, stale.stderr);
  assert.match(stale.stdout, /dist-tag add cc-connect-next@1\.0\.13-beta\.5 beta/);
});

test("publishing an older beta uses a staging tag and preserves the newer channel", () => {
  const result = runNpmPublication({ channel: "1.0.13-beta.6", staging: true });
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /npm publish --access public --tag release-1-0-13-beta-5-42/);
  assert.match(result.stdout, /dist-tag rm cc-connect-next release-1-0-13-beta-5-42/);
  assert.doesNotMatch(result.stdout, /dist-tag add/);
});
