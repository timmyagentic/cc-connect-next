# CC Connect Next host mapping

This directory is copied from `awesome-agent-app-features v0.1.0`
(`1634667face06c20ba1e71d1b1599c959e882376`) and remains independently
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
