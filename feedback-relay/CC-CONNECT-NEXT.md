# CC Connect Next host mapping

This directory is copied from the CI-verified Foundation development commit
`0ab8c151f755080560eecf040f875a3e4413fd70` (Go version
`v0.1.3-0.20260927093303-0ab8c151f755`) and remains independently
testable. Foundation files remain byte-identical; the host adds
`CC-CONNECT-NEXT.md`, `src/worker.js`, `src/compat.js`, `src/github-app.js`,
`test/host-auth.runtime.spec.js`, and `vitest.host.config.js`, and owns
`wrangler.jsonc` plus its generated `worker-configuration.d.ts`.

- Worker name: `cc-connect-feedback` (preserves the existing deployment name).
- GitHub destination: server-side `timmyagentic/cc-connect-next`.
- GitHub identity: the repository-selected `cc-connect-feedback` GitHub App,
  with only Issues read/write and implicit Metadata read.
- Non-secret vars: `GITHUB_APP_ID` and `GITHUB_APP_INSTALLATION_ID`.
- Wrangler manages `GITHUB_APP_PRIVATE_KEY` as a secret binding.
  The Relay dynamically exchanges a short-lived RS256 JWT for a
  repository-scoped installation token after request validation and rate
  limiting; no personal access token is used.
- Rate Limiting namespace: `1001` is a local/dry-run placeholder. An operator
  must replace it with a unique positive integer in the target Cloudflare
  account before a separately authorized deployment.
- Client contract: legacy and structured v1 at `POST /v1/feedback`, and strict
  diagnostic v2 at `POST /v2/feedback`.
- Durable receipt binding: `FEEDBACK_REPORTS`, with a SQLite-backed
  `FeedbackReport` migration. Dispatch state persists no report body or token.

Local verification runs from this directory:

```bash
sh verify-host.sh
```

`verify-host.sh` composes the byte-identical Foundation gate with the CC legacy
compatibility, GitHub App authentication, and real workerd signing/receipt tests.
It also feeds the actual approved JSON from the two-hour host CUJ into the host
Relay and asserts the rendered GitHub issue at a mocked external boundary. The
Makefile and both tag-only CI/Release workflows call this same entrypoint.

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

The host-owned `wrangler.jsonc` selects `src/worker.js`, which exports the durable
class and delegates to the host `src/compat.js` authentication adapter.
It preserves the strict Foundation Relay for new structured requests while
translating the exact legacy CC Connect schema-1 shape into the structured
request before validation and server-side rendering. The legacy `install_id`
is discarded, and legacy clients cannot select the repository or credential.
This permits an in-place Worker rollout before a new CC Connect binary is
released; remove the compatibility entrypoint only after the supported legacy
client window has ended. Invalid UTF-8 is rejected before authentication;
replacement-character decoding cannot turn malformed bytes into a valid report.
Both installation-token exchange and Foundation GitHub API requests use manual
redirect handling and reject non-success responses. Native workerd Request tests
cover this boundary, including the short-lived App JWT.

Deploy this dual-protocol Worker and its migration before releasing the v2
client. Existing client endpoint configuration is mapped internally to the same
origin's v2 path, without a new setup step. v2 cannot translate legacy payloads
or downgrade after a protocol rejection. A lost GitHub create response enters
an unknown state; later approved retries may recover the exact receipt by its
ID/hash marker, but an empty search never permits another blind POST.

This change does not itself deploy the production Worker, restart a daemon,
publish a client, or prove a real messaging-platform flow.
