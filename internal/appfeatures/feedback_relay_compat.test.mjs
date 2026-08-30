import assert from "node:assert/strict";
import { afterEach, beforeEach, test } from "node:test";

import worker from "../../feedback-relay/src/compat.js";

let originalFetch;
let calls;

function relayEnv() {
  return {
    GITHUB_TOKEN: "test-token",
    GITHUB_REPO: "timmyagentic/cc-connect-next",
    GITHUB_LABEL: "user-feedback",
    RATE_LIMITER: {async limit() { return {success: true}; }},
  };
}

beforeEach(() => {
  originalFetch = globalThis.fetch;
  calls = [];
  globalThis.fetch = async (url, init = {}) => {
    calls.push({url: String(url), init});
    if (String(url).includes("/search/issues")) {
      return Response.json({items: []});
    }
    return Response.json({html_url: "https://github.com/timmyagentic/cc-connect-next/issues/7"});
  };
});

afterEach(() => {
  globalThis.fetch = originalFetch;
});

test("legacy schema-1 clients are translated before the strict Foundation relay", async () => {
  const response = await worker.fetch(new Request("https://relay.example/v1/feedback", {
    method: "POST",
    headers: {"content-type": "application/json"},
    body: JSON.stringify({
      schema: 1,
      install_id: "legacy-install-identifier",
      version: "v0.2.0",
      os: "darwin",
      arch: "arm64",
      agent: "codex",
      title: "Legacy startup failure",
      body: "The old client explicitly submitted this redacted report.",
    }),
  }), relayEnv());

  assert.equal(response.status, 200);
  const issueRequest = calls.find((call) => call.url.endsWith("/repos/timmyagentic/cc-connect-next/issues"));
  assert.ok(issueRequest, "translated request did not reach the configured repository");
  const issue = JSON.parse(issueRequest.init.body);
  assert.match(issue.title, /Legacy startup failure/);
  assert.doesNotMatch(issue.body, /legacy-install-identifier/);
});

test("structured clients pass through unchanged", async () => {
  const response = await worker.fetch(new Request("https://relay.example/v1/feedback", {
    method: "POST",
    headers: {"content-type": "application/json"},
    body: JSON.stringify({
      schema: 1,
      user_approved: true,
      environment: {product: "cc-connect-next", version: "v0.3.0", agent: "codex"},
      description: "Structured report",
    }),
  }), relayEnv());
  assert.equal(response.status, 200);
});

test("legacy compatibility remains bounded and rejects expanded client control", async () => {
  const attacker = await worker.fetch(new Request("https://relay.example/v1/feedback", {
    method: "POST",
    headers: {"content-type": "application/json"},
    body: JSON.stringify({
      schema: 1,
      title: "Client title",
      body: "Client body",
      repo: "attacker/repository",
    }),
  }), relayEnv());
  assert.equal(attacker.status, 400);

  const oversized = await worker.fetch(new Request("https://relay.example/v1/feedback", {
    method: "POST",
    headers: {"content-type": "application/json"},
    body: "x".repeat(100 * 1024),
  }), relayEnv());
  assert.equal(oversized.status, 413);
});
