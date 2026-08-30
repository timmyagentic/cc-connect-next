import assert from "node:assert/strict";
import fs from "node:fs";
import { test } from "node:test";

import worker from "../../feedback-relay/src/index.js";

test("Go-approved Feedback v1 fixture is accepted by the copied Worker", async () => {
  const payload = JSON.parse(
    fs.readFileSync(new URL("./testdata/approved-feedback-v1.json", import.meta.url), "utf8"),
  );
  const originalFetch = globalThis.fetch;
  const calls = [];
  globalThis.fetch = async (url, init = {}) => {
    calls.push({ url: String(url), init });
    if (String(url).includes("/search/issues")) return Response.json({ items: [] });
    return Response.json({ html_url: "https://github.com/timmyagentic/cc-connect-next/issues/7" });
  };
  try {
    const response = await worker.fetch(
      new Request("https://relay.example/v1/feedback", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify(payload),
      }),
      {
        GITHUB_TOKEN: "test-token",
        GITHUB_REPO: "timmyagentic/cc-connect-next",
        GITHUB_LABEL: "user-feedback",
        RATE_LIMITER: { async limit() { return { success: true }; } },
      },
    );
    assert.equal(response.status, 200);
    assert.deepEqual(await response.json(), {
      reference_url: "https://github.com/timmyagentic/cc-connect-next/issues/7",
      deduplicated: false,
    });
    assert.equal(calls.length, 2);
    assert.match(calls[1].url, /\/repos\/timmyagentic\/cc-connect-next\/issues$/);
  } finally {
    globalThis.fetch = originalFetch;
  }
});
