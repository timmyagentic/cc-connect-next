# Awesome Agent App Features integration

[中文](agent-app-features.zh-CN.md)

CC Connect Next consumes `github.com/timmyagentic/awesome-agent-app-features`
at immutable version `v0.1.0` and source commit
`1634667face06c20ba1e71d1b1599c959e882376`. There is no local `replace`,
submodule, or floating `main` dependency.

## Feedback

CC Connect Next owns the command, cards, text fallback, localization, recent
error selection, capability-gap prompts, and public fallback URL. The
Foundation owns the structured report, allowlisted environment, redaction and
bounds, opaque approval value, and no-redirect HTTP client.

1. `/feedback <description>` builds a fully redacted Draft.
2. The host renders every `Draft.Report()` field and keeps that exact Draft for
   ten minutes under an opaque token bound to the session and initiating user.
3. A separate token-bearing button or unambiguous `/feedback confirm` calls
   `Approve(true)` and sends the same Draft. Stale cards, another group user,
   cancel, expiry, ambiguity, and missing approval make no request.
4. The Relay owns GitHub repository selection, title/body rendering, label,
   token, rate limiting, and best-effort deduplication.

## Updates

The daemon notice is discovery only. `/upgrade` prepares an immutable Plan,
shows the exact Release notes and selected artifact, and retains the Plan under
an opaque session/user token. The token-bearing action applies only that Plan
without resolving latest again; a generic confirmation is rejected when more
than one Plan is pending.

- Standalone macOS/Linux uses the Foundation checksum, staging, two version
  probes, per-target lock, no-clobber backup, replacement, and rollback.
- npm pins and installs the reviewed stable package version through the host
  adapter, then verifies package metadata and binary version.
- Windows remains an explicit host replacement adapter, but consumes the same
  stable release object, exact archive/checksum, staged and installed probes,
  no-clobber backup, and rollback boundaries.
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
releasing the new client without breaking existing installations.

Run all Relay commands from `feedback-relay/`; do not use an external absolute
`npm --prefix` invocation as a substitute for testing the final target.

## Lock validation

`agent-app-features.lock.json` records the exact source, module deliveries,
subtree target, changed files, checks, and unverified production boundaries.
Validate it against a temporary full extraction of the same source commit:

```bash
GOWORK=off go run \
  github.com/timmyagentic/awesome-agent-app-features/cmd/feature-lock@v0.1.0 \
  validate \
  --source "$EXACT_SOURCE_ROOT" \
  --source-commit 1634667face06c20ba1e71d1b1599c959e882376 \
  --host "$CC_CONNECT_NEXT_ROOT" \
  --lock "$CC_CONNECT_NEXT_ROOT/agent-app-features.lock.json"
```

The lock is maintenance metadata, not runtime configuration or proof that the
current public Relay endpoint has been deployed with this contract.
