import assert from "node:assert/strict";
import { after, afterEach, before, beforeEach, test } from "node:test";

import worker from "../../feedback-relay/src/compat.js";

let originalFetch;
let calls;
let privateKeyPEM;
let publicKey;

function base64URLJSON(value) {
  return JSON.parse(Buffer.from(value, "base64url").toString("utf8"));
}

before(async () => {
  const pair = await crypto.subtle.generateKey(
    {
      name: "RSASSA-PKCS1-v1_5",
      modulusLength: 2048,
      publicExponent: new Uint8Array([1, 0, 1]),
      hash: "SHA-256",
    },
    true,
    ["sign", "verify"],
  );
  publicKey = pair.publicKey;
  const bytes = Buffer.from(await crypto.subtle.exportKey("pkcs8", pair.privateKey));
  privateKeyPEM = [
    "-----BEGIN PRIVATE KEY-----",
    ...bytes.toString("base64").match(/.{1,64}/g),
    "-----END PRIVATE KEY-----",
  ].join("\n");
});

after(() => {
  privateKeyPEM = undefined;
  publicKey = undefined;
});

function relayEnv() {
  return {
    GITHUB_APP_ID: "123",
    GITHUB_APP_INSTALLATION_ID: "456",
    GITHUB_APP_PRIVATE_KEY: privateKeyPEM,
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
    if (String(url).endsWith("/app/installations/456/access_tokens")) {
      return Response.json({
        token: "ghs_123_test_installation_token",
        expires_at: new Date(Date.now() + 60 * 60 * 1000).toISOString(),
      });
    }
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
  assert.equal(issueRequest.init.headers.authorization, "Bearer ghs_123_test_installation_token");
  const issue = JSON.parse(issueRequest.init.body);
  assert.match(issue.title, /Legacy startup failure/);
  assert.doesNotMatch(issue.body, /legacy-install-identifier/);
});

test("mints a repository-scoped GitHub App installation token with a valid RS256 JWT", async () => {
  const response = await worker.fetch(new Request("https://relay.example/v1/feedback", {
    method: "POST",
    headers: {"content-type": "application/json"},
    body: JSON.stringify({
      schema: 1,
      user_approved: true,
      environment: {product: "cc-connect-next", version: "v0.3.0", agent: "codex"},
      description: "GitHub App authentication",
    }),
  }), relayEnv());
  assert.equal(response.status, 200);

  const tokenRequest = calls.find((call) => call.url.endsWith("/app/installations/456/access_tokens"));
  assert.ok(tokenRequest, "installation token endpoint was not called");
  assert.equal(tokenRequest.init.method, "POST");
  const tokenBody = JSON.parse(tokenRequest.init.body);
  assert.deepEqual(tokenBody, {
    repositories: ["cc-connect-next"],
    permissions: {issues: "write"},
  });

  const jwt = tokenRequest.init.headers.authorization.replace(/^Bearer /, "");
  const [encodedHeader, encodedPayload, encodedSignature] = jwt.split(".");
  assert.deepEqual(base64URLJSON(encodedHeader), {alg: "RS256", typ: "JWT"});
  const payload = base64URLJSON(encodedPayload);
  assert.equal(payload.iss, "123");
  assert.equal(payload.exp - payload.iat, 600);
  assert.ok(payload.iat <= Math.floor(Date.now() / 1000));
  assert.ok(payload.exp <= Math.floor(Date.now() / 1000) + 10 * 60);
  assert.equal(
    await crypto.subtle.verify(
      "RSASSA-PKCS1-v1_5",
      publicKey,
      Buffer.from(encodedSignature, "base64url"),
      new TextEncoder().encode(`${encodedHeader}.${encodedPayload}`),
    ),
    true,
  );
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
  assert.equal(calls.filter((call) => call.url.includes("/access_tokens")).length, 0);
});

test("rate limits before minting an installation token", async () => {
  const env = relayEnv();
  env.RATE_LIMITER = {async limit() { return {success: false}; }};
  const response = await worker.fetch(new Request("https://relay.example/v1/feedback", {
    method: "POST",
    headers: {"content-type": "application/json", "cf-connecting-ip": "192.0.2.1"},
    body: JSON.stringify({
      schema: 1,
      user_approved: true,
      environment: {product: "cc-connect-next"},
      description: "Should be rate limited",
    }),
  }), env);
  assert.equal(response.status, 429);
  assert.equal(calls.length, 0);
});

test("maps GitHub App authentication failure to a bounded response without leaking credentials", async () => {
  const logs = [];
  const originalConsoleError = console.error;
  console.error = (value) => logs.push(JSON.parse(value));
  globalThis.fetch = async (url) => {
    calls.push({url: String(url)});
    return new Response("private upstream detail", {status: 401});
  };
  try {
    const response = await worker.fetch(new Request("https://relay.example/v1/feedback", {
      method: "POST",
      headers: {"content-type": "application/json"},
      body: JSON.stringify({
        schema: 1,
        user_approved: true,
        environment: {product: "cc-connect-next"},
        description: "Auth failure",
      }),
    }), relayEnv());
    assert.equal(response.status, 502);
    assert.deepEqual(await response.json(), {error: "github app authentication failed"});
    assert.equal(JSON.stringify(logs).includes("private upstream detail"), false);
    assert.equal(JSON.stringify(logs).includes(privateKeyPEM), false);
  } finally {
    console.error = originalConsoleError;
  }
});

test("rejects missing App configuration and malformed successful token responses", async () => {
  const originalConsoleError = console.error;
  console.error = () => {};
  try {
    const missing = relayEnv();
    delete missing.GITHUB_APP_ID;
    const missingResponse = await worker.fetch(new Request("https://relay.example/v1/feedback", {
      method: "POST",
      headers: {"content-type": "application/json"},
      body: JSON.stringify({
        schema: 1,
        user_approved: true,
        environment: {product: "cc-connect-next"},
        description: "Missing configuration",
      }),
    }), missing);
    assert.equal(missingResponse.status, 500);
    assert.equal(calls.length, 0);

    globalThis.fetch = async (url) => {
      calls.push({url: String(url)});
      return Response.json({token: "", expires_at: new Date(Date.now() + 60 * 60 * 1000).toISOString()});
    };
    const malformedResponse = await worker.fetch(new Request("https://relay.example/v1/feedback", {
      method: "POST",
      headers: {"content-type": "application/json"},
      body: JSON.stringify({
        schema: 1,
        user_approved: true,
        environment: {product: "cc-connect-next"},
        description: "Malformed token response",
      }),
    }), relayEnv());
    assert.equal(malformedResponse.status, 502);
    assert.deepEqual(await malformedResponse.json(), {error: "github app authentication failed"});
    assert.equal(calls.some((call) => call.url.includes("/search/issues")), false);
  } finally {
    console.error = originalConsoleError;
  }
});
