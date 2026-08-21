# cc-connect-next feedback relay

A Cloudflare Worker that lets cc-connect-next installations file feedback as
GitHub issues **without the reporter needing a GitHub account**. The daemon
POSTs a confirmed, redacted submission here; the Worker creates the issue
using a token that never leaves the author's infrastructure.

## Contract

`POST /v1/feedback`, JSON schema 1:

```json
{
  "schema": 1,
  "install_id": "…", "version": "…", "os": "…", "arch": "…",
  "agent": "codex", "trigger": "user | config_keys",
  "title": "…", "body": "…"
}
```

Success: `200 {"issue_url": "https://github.com/…/issues/N"}`. Errors use
4xx/5xx with `{"error": …}`; the daemon degrades to pointing the user at the
public issue tracker.

Abuse brakes: 5 submissions/min per install (falls back to IP), title ≤ 200
chars, body ≤ 12000 chars, `user-feedback` label on every issue
(`config-gap` added for unsupported-config reports) so triage and bulk
cleanup stay easy.

## Deploy (author only)

```bash
cd feedback-relay
wrangler deploy
wrangler secret put GITHUB_TOKEN   # fine-grained PAT: issues:write on this repo only
```

The client's default endpoint is `core.DefaultFeedbackEndpoint`; deployments
can override it via `[feedback] endpoint` in config.toml.

## Local development

```bash
GITHUB_TOKEN=$(gh auth token) wrangler dev
# then point a daemon at it:
# [feedback]
# endpoint = "http://localhost:8787/v1/feedback"
```
