# CC Connect Next host mapping

This directory is copied from `awesome-agent-app-features v0.1.1`
(`2e30c73ee6c3192f057ef24fa5bb4f77b8346c81`) and remains independently
testable. Foundation files remain byte-identical; the host adds
`CC-CONNECT-NEXT.md`, `src/compat.js`, `src/github-app.js`,
`test/host-auth.runtime.spec.js`, and `vitest.host.config.js`, and owns
`wrangler.jsonc` plus its generated `worker-configuration.d.ts`.

- Worker name: `cc-connect-feedback` (preserves the existing deployment name).
- GitHub destination: server-side `timmyagentic/cc-connect-next`.
- GitHub identity: the repository-selected `cc-connect-feedback` GitHub App,
  with only Issues read/write and implicit Metadata read.
- Non-secret vars: `GITHUB_APP_ID` and `GITHUB_APP_INSTALLATION_ID`.
- Secret: `GITHUB_APP_PRIVATE_KEY`, stored only with Wrangler secret
  management. The Relay dynamically exchanges a short-lived RS256 JWT for a
  repository-scoped installation token after request validation and rate
  limiting; no personal access token is used.
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

GitHub downloads App keys as PKCS#1. Convert the downloaded key to unencrypted
PKCS#8 before setting the Worker secret, then delete the temporary local key
files after the secret has been read back through a real App-authenticated
request:

```bash
openssl pkcs8 -topk8 -nocrypt -in github-app-private-key.pem -out github-app-private-key.pkcs8.pem
npm exec -- wrangler secret put GITHUB_APP_PRIVATE_KEY < github-app-private-key.pkcs8.pem
```

Repository changes do not deploy the Worker automatically. A production
cutover must deploy the reviewed exact head, verify a real feedback issue is
authored by the App bot, and only then remove the obsolete `GITHUB_TOKEN`
secret.

## Existing-client migration

The host-owned `wrangler.jsonc` selects the extra `src/compat.js` entrypoint.
It preserves the strict Foundation Relay for new structured requests while
translating the exact legacy CC Connect schema-1 shape into the structured
request before validation and server-side rendering. The legacy `install_id`
is discarded, and legacy clients cannot select the repository or credential.
This permits an in-place Worker rollout before a new CC Connect binary is
released; remove the compatibility entrypoint only after the supported legacy
client window has ended.
