# Awesome Agent App Features integration

[中文](agent-app-features.zh-CN.md)

CC Connect Next consumes `github.com/timmyagentic/awesome-agent-app-features`
at immutable version `v0.1.2` and source commit
`9daaa15dcaf4512ce655c733264713d4d1eb72b6`. There is no local `replace`,
submodule, or floating `main` dependency. This version is the published Foundation `v0.1.2` patch release.

## Feedback

CC Connect Next owns the command, cards, text fallback, localization, recent
error selection, capability-gap prompts, and public fallback URL. The
Foundation owns the structured report, allowlisted environment, redaction and
bounds, opaque approval value, and no-redirect HTTP client.

1. An explicit `/feedback <description>` command or Feedback card action builds
   a fully redacted Draft, calls `Approve(true)`, and submits it immediately.
   Chat never renders a Draft preview and never asks for a second confirmation.
2. Automatic offers prepare the exact Draft under an opaque token but make no
   request. Error offers bind to the initiating user, including in shared
   sessions; an unknown user suppresses the offer. Capability-gap notices contain
   only generic capability metadata, so a participant in the same session may
   submit them. Expiry, replay, or a mismatched session/required user fails closed.
   Adjacent diagnostic messages must have a known timestamp within the recent
   context window; unknown, future, and stale timestamps are excluded.
3. The Manifest-declared local-Agent CLI uses the same builder and submit
   function. A live HMAC turn credential resolves trusted project/session/user
   state; `feedback preview` exposes a JSON-safe projection with no request,
   and `feedback submit` accepts only its one-time session/user-bound token.
   The CLI cannot supply routing, forge an inbound message, or select a schema
   fallback.
4. The Relay owns GitHub repository selection, title/body rendering, label,
   token, rate limiting, and best-effort deduplication. Quoted JSON/config keys,
   escaped values, prefixed credentials, cookies, URL credentials, and host
   identifiers are redacted before previews, truncation, and submission.

## Updates

The daemon notice is discovery only. `/upgrade` prepares an immutable Plan,
shows the exact Release notes and selected artifact, and retains the Plan under
an opaque session/user token. The token-bearing action applies only that Plan
without resolving latest again; a generic confirmation is rejected when more
than one Plan is pending.

- Stable standalone macOS/Linux uses the Foundation checksum, staging, two
  version probes, per-target lock, no-clobber backup, replacement, and rollback.
- Explicit Beta and Windows use the host replacement adapter with the same
  immutable release, archive/checksum, and probe boundaries. Unix Beta preserves
  existing recovery backups and executable permissions, creates backups without
  replacement, and syncs installation and rollback directory changes. Windows
  uses a no-clobber move and a cross-process exclusive file handle; only its
  verified stale running-image backup may be removed before an update.
- npm pins the reviewed exact package version for the selected Stable/Beta
  channel, then verifies package metadata and binary version.
- Restart, post-restart acknowledgement, cards, natural-language intent,
  authorization, and localization remain CC Connect Next responsibilities.

## Relay source subtree

`feedback-relay/` is copied from the same Foundation commit's
`relay/cloudflare` subtree. Only `wrangler.jsonc` and the generated
`worker-configuration.d.ts` may differ. The Worker name and server-side target
repository are host mappings; the Rate Limiting namespace remains a dry-run
placeholder until an operator performs a separately authorized deployment.

The host-owned Wrangler entrypoint is `src/compat.js`. It passes new structured
requests to the byte-identical Foundation Relay and translates only the exact
legacy CC Connect schema-1 shape first, discarding `install_id` and retaining
server-owned destination/rendering. This permits deploying the Worker before
releasing the new client without breaking existing installations. Invalid UTF-8
is rejected before token exchange. GitHub App token exchange and Foundation
GitHub API requests disable automatic redirects and reject redirect responses,
so authorization cannot follow a redirect to another origin.

Run all Relay commands from `feedback-relay/`; do not use an external absolute
`npm --prefix` invocation as a substitute for testing the final target.

## Lock validation

`agent-app-features.lock.json` records the exact source, module deliveries,
subtree target, changed files, checks, and unverified production boundaries.
Validate it against a temporary full extraction of the same source commit:

```bash
GOWORK=off go run \
  github.com/timmyagentic/awesome-agent-app-features/cmd/feature-lock@9daaa15dcaf4512ce655c733264713d4d1eb72b6 \
  validate \
  --source "$EXACT_SOURCE_ROOT" \
  --source-commit 9daaa15dcaf4512ce655c733264713d4d1eb72b6 \
  --host "$CC_CONNECT_NEXT_ROOT" \
  --lock "$CC_CONNECT_NEXT_ROOT/agent-app-features.lock.json"
```

The lock is maintenance metadata, not runtime configuration or proof that the
current public Relay endpoint has been deployed with this contract.
