/**
 * cc-connect-next feedback relay.
 *
 * Accepts anonymous in-app feedback submissions from cc-connect-next daemons
 * and files them as GitHub issues, so reporters need no GitHub account. The
 * GitHub token lives only here (Worker secret), never in the client binary.
 *
 * Deploy:  wrangler deploy
 * Secrets: wrangler secret put GITHUB_TOKEN   (fine-grained PAT, issues:write
 *          on the single target repo)
 * Vars:    REPO (default "timmyagentic/cc-connect-next")
 */

export interface Env {
  GITHUB_TOKEN: string;
  REPO?: string;
}

interface Submission {
  schema: number;
  install_id?: string;
  version?: string;
  os?: string;
  arch?: string;
  agent?: string;
  trigger?: string;
  title: string;
  body: string;
}

const MAX_TITLE = 200;
const MAX_BODY = 12000;
const RATE_LIMIT = 5; // submissions per window per client
const RATE_WINDOW_MS = 60_000;

// Per-isolate rate limiting: resets on isolate recycle, which is acceptable
// for a spam brake (the GitHub token's own scope is the hard boundary).
const buckets = new Map<string, { count: number; resetAt: number }>();

function rateLimited(key: string): boolean {
  const now = Date.now();
  const bucket = buckets.get(key);
  if (!bucket || now > bucket.resetAt) {
    buckets.set(key, { count: 1, resetAt: now + RATE_WINDOW_MS });
    return false;
  }
  bucket.count++;
  return bucket.count > RATE_LIMIT;
}

function json(status: number, payload: unknown): Response {
  return new Response(JSON.stringify(payload), {
    status,
    headers: { "content-type": "application/json" },
  });
}

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    const url = new URL(request.url);
    if (request.method !== "POST" || url.pathname !== "/v1/feedback") {
      return json(404, { error: "not found" });
    }

    let sub: Submission;
    try {
      sub = (await request.json()) as Submission;
    } catch {
      return json(400, { error: "invalid JSON" });
    }
    if (sub.schema !== 1 || typeof sub.title !== "string" || typeof sub.body !== "string") {
      return json(400, { error: "unsupported schema" });
    }
    if (sub.title.trim() === "" || sub.body.trim() === "") {
      return json(400, { error: "empty submission" });
    }
    if (sub.title.length > MAX_TITLE || sub.body.length > MAX_BODY) {
      return json(413, { error: "submission too large" });
    }

    const clientKey =
      (sub.install_id && String(sub.install_id).slice(0, 64)) ||
      request.headers.get("cf-connecting-ip") ||
      "unknown";
    if (rateLimited(clientKey)) {
      return json(429, { error: "rate limited" });
    }

    const repo = env.REPO || "timmyagentic/cc-connect-next";
    const labels = ["user-feedback"];
    if (sub.trigger === "config_keys") labels.push("config-gap");

    const resp = await fetch(`https://api.github.com/repos/${repo}/issues`, {
      method: "POST",
      headers: {
        authorization: `Bearer ${env.GITHUB_TOKEN}`,
        accept: "application/vnd.github+json",
        "content-type": "application/json",
        "user-agent": "cc-connect-feedback-relay",
      },
      body: JSON.stringify({ title: sub.title, body: sub.body, labels }),
    });

    if (!resp.ok) {
      const detail = await resp.text();
      console.error(`github issue create failed: ${resp.status} ${detail.slice(0, 300)}`);
      return json(502, { error: "issue creation failed" });
    }

    const issue = (await resp.json()) as { html_url?: string };
    if (!issue.html_url) {
      return json(502, { error: "unexpected github response" });
    }
    return json(200, { issue_url: issue.html_url });
  },
};
