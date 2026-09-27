# Awesome Agent App Features integration

[中文](agent-app-features.zh-CN.md)

CC Connect Next consumes `github.com/timmyagentic/awesome-agent-app-features`
at immutable version `v0.1.3` and source commit
`c8650a6886031ac722b4e7dd6bf25279ff8c93bf`. There is no local `replace`,
submodule, or floating `main` dependency. The published v0.1.3 tag and its CI-verified source commit are immutable.
Feedback and Updater use the same module.

## Feedback

CC Connect Next owns the command, cards, text fallback, localization, recent
selection, capability-gap prompts, and public fallback URL. The Foundation
owns the versioned diagnostic report, allowlisted fields, redaction and bounds,
opaque approval value, and no-redirect HTTP client. The additive
`feedback/diagnostic` package and `/v2/feedback` preserve the original v1 API.

1. An explicit `/feedback <description>` command or Feedback card action builds
   a fully redacted Draft, calls `Approve(true)`, and submits it immediately.
   Chat never renders a Draft preview and never asks for a second confirmation.
2. Automatic offers prepare the exact Draft under an opaque token but make no
   request. Error offers bind to the initiating user, including in shared
   sessions; an unknown user suppresses the offer. Capability-gap notices contain
   only generic capability metadata, so a participant in the same session may
   submit them. Expiry, replay, or a mismatched session/required user fails closed.
   Tokens retain their existing ten-minute lifetime and one-use semantics.
3. The Manifest-declared local-Agent CLI uses the same builder and submit
   function. A live HMAC turn credential resolves trusted project/session/user
   state; `feedback preview` exposes a JSON-safe projection with no request,
   and `feedback submit` accepts only its one-time session/user-bound token.
   The CLI cannot supply routing, forge an inbound message, or select a schema
   fallback.
4. The host captures the initiating user's original text before enrichment or
   Agent startup. Failures freeze the request, phase, runtime options, bounded
   activity and transport facts before cleanup. Long turns no longer depend on
   a recent-history window. Queues and accepted steering preserve ownership;
   other participants' input and answers are excluded. Arbitrary history,
   tool arguments/results, raw protocol events, credentials and configuration
   maps are never collected. Unsupported backend fields are explicitly missing.
5. Local redacted snapshots retain up to 20 turns and 64 pending/approved
   submissions per project, bounded by 10 MiB. Frozen turns and approved records
   expire after 72 hours; active turns survive that age boundary. Atomic files
   under `data_dir/run/feedback` use mode 0600 and hashed routing identities.
   Restart restores exact pending drafts, marks unfinished turns as interrupted
   with an unknown backend outcome, and never resumes a network submission.
6. An approved report retains its random ID and exact payload across bounded
   retries and restart. The Relay persists dispatch intent and confirmed receipts
   in a SQLite-backed Durable Object before responding. A lost create response
   permits receipt reconciliation but no second blind issue creation. Empty
   GitHub search results do not prove that a prior POST failed. Existing chat
   success/failure wording and card behavior remain unchanged.

The Relay owns repository selection, issue rendering, label, authentication and
rate limiting. Quoted JSON/config keys,
   escaped values, prefixed credentials, cookies, URL credentials, and host
identifiers are redacted before local storage, previews, truncation and submission.
The Codex adapter exposes cached request options, EOF/read state, queue/drop
counters and terminal-received versus terminal-delivered facts without extra
CLI or network calls. Other adapters degrade through the optional interface.

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

The host-owned Wrangler entrypoint is `src/worker.js`, which exports the
Foundation Durable Object and delegates HTTP authentication to `src/compat.js`.
It passes new structured
requests to the byte-identical Foundation Relay and translates only the exact
legacy CC Connect schema-1 shape first, discarding `install_id` and retaining
server-owned destination/rendering. This permits deploying the Worker before
releasing the new client without breaking existing installations. Invalid UTF-8
is rejected before token exchange. GitHub App token exchange and Foundation
GitHub API requests disable automatic redirects and reject redirect responses,
so authorization cannot follow a redirect to another origin.

Deploy the dual v1/v2 Relay and its `FEEDBACK_REPORTS` SQLite migration before
releasing a client that sends v2. Existing configured `/v1/feedback` URLs are
upgraded to the same origin's `/v2/feedback` internally; users need no new
configuration or commands. There is no schema downgrade or alternate POST on
rejection. Production deployment and real messaging-client validation remain
separate, unverified boundaries until explicitly exercised.

Run all Relay commands from `feedback-relay/`; do not use an external absolute
`npm --prefix` invocation as a substitute for testing the final target.

## Lock validation

`agent-app-features.lock.json` records the exact source, module deliveries,
subtree target, changed files, checks, and unverified production boundaries.
Validate it against a temporary full extraction of the same source commit:

```bash
GOWORK=off go run \
  github.com/timmyagentic/awesome-agent-app-features/cmd/feature-lock@c8650a6886031ac722b4e7dd6bf25279ff8c93bf \
  validate \
  --source "$EXACT_SOURCE_ROOT" \
  --source-commit c8650a6886031ac722b4e7dd6bf25279ff8c93bf \
  --host "$CC_CONNECT_NEXT_ROOT" \
  --lock "$CC_CONNECT_NEXT_ROOT/agent-app-features.lock.json"
```

The lock is maintenance metadata, not runtime configuration or proof that the
current public Relay endpoint has been deployed with this contract.
