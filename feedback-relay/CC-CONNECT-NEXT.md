# CC Connect Next host mapping

This directory is copied from `awesome-agent-app-features v0.1.1`
(`2e30c73ee6c3192f057ef24fa5bb4f77b8346c81`) and remains independently
testable. `wrangler.jsonc` and its generated `worker-configuration.d.ts` are
the only Foundation files intentionally owned by this host.

- Worker name: `cc-connect-feedback` (preserves the existing deployment name).
- GitHub destination: server-side `timmyagentic/cc-connect-next`.
- Secret: `GITHUB_TOKEN`, set only with Wrangler secret management.
- Rate Limiting namespace: `1001` is a local/dry-run placeholder. An operator
  must replace it with a unique positive integer in the target Cloudflare
  account before a separately authorized deployment.
- Client contract: structured Feedback v1 at exact `POST /v1/feedback`.

Local verification runs from this directory:

```bash
npm ci --ignore-scripts
npm test
npm run check
npm run typecheck
npm run types:check
npm run validate:worker
npm audit --audit-level=high
```

This integration does not deploy the Worker, set secrets, or prove the current
public endpoint has been cut over to the new structured contract.

## Existing-client migration

The host-owned `wrangler.jsonc` selects the extra `src/compat.js` entrypoint.
It preserves the strict Foundation Relay for new structured requests while
translating the exact legacy CC Connect schema-1 shape into the structured
request before validation and server-side rendering. The legacy `install_id`
is discarded, and legacy clients cannot select the repository or credential.
This permits an in-place Worker rollout before a new CC Connect binary is
released; remove the compatibility entrypoint only after the supported legacy
client window has ended.
