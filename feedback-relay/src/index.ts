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

    const gh = (path: string, init?: RequestInit) =>
      fetch(`https://api.github.com${path}`, {
        ...init,
        headers: {
          authorization: `Bearer ${env.GITHUB_TOKEN}`,
          accept: "application/vnd.github+json",
          "content-type": "application/json",
          "user-agent": "cc-connect-feedback-relay",
          ...(init?.headers || {}),
        },
      });

    // Dedup: identical titles thread onto one open issue as
    // "+1" comments instead of flooding the tracker — the comment count then
    // doubles as a frequency signal for triage. Best-effort only: the search
    // index lags a few seconds, so near-simultaneous duplicates may still
    // create two issues, and closed issues intentionally get a fresh one.
    const fp = await fingerprint(sub.title);
    const marker = `ccn-fp:${fp}`;
    const envLine = `${sub.version ?? "?"} · ${sub.os ?? "?"}/${sub.arch ?? "?"} · ${sub.agent ?? "?"}`;

    try {
      const q = encodeURIComponent(`repo:${repo} is:issue is:open label:user-feedback "${marker}"`);
      const searchResp = await gh(`/search/issues?q=${q}&per_page=1`);
      if (searchResp.ok) {
        const found = (await searchResp.json()) as {
          items?: { number: number; html_url: string }[];
        };
        const existing = found.items?.[0];
        if (existing) {
          const commentResp = await gh(`/repos/${repo}/issues/${existing.number}/comments`, {
            method: "POST",
            body: JSON.stringify({ body: `+1 — ${envLine}` }),
          });
          if (!commentResp.ok) {
            console.error(`dedup comment failed: ${commentResp.status}`);
          }
          return json(200, { issue_url: existing.html_url, deduplicated: true });
        }
      } else {
        console.error(`dedup search failed: ${searchResp.status}`);
      }
    } catch (err) {
      console.error(`dedup lookup error: ${err}`);
      // fall through to plain creation
    }

    const resp = await gh(`/repos/${repo}/issues`, {
      method: "POST",
      body: JSON.stringify({
        title: sub.title,
        body: `${sub.body}\n\n\`${marker}\``,
        labels,
      }),
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

async function fingerprint(input: string): Promise<string> {
  const digest = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(input));
  return [...new Uint8Array(digest)]
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("")
    .slice(0, 12);
}
