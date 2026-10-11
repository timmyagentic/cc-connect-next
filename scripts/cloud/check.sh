#!/bin/sh
set -eu
cloud_root=$(CDPATH='' cd -- "$(dirname "$0")/../.." && pwd)
# Require a reviewable commit, not an inferred branch name or dirty working tree.
expected=${1:?usage: sh scripts/cloud/check.sh FULL_COMMIT_SHA}
printf '%s\n' "$expected" | grep -Eq '^[0-9a-f]{40}$' || exit 2
cd "$cloud_root"
[ "$(git rev-parse HEAD)" = "$expected" ] || { echo 'SHA mismatch' >&2; exit 1; }
[ -z "$(git status --porcelain --untracked-files=normal)" ] || { echo 'Clean checkout required' >&2; exit 1; }
. "$PWD/scripts/cloud/env.sh"
sh scripts/cloud/setup.sh
node scripts/check-release-version.js
node --test scripts/*.test.js
npm --prefix npm test
node --check npm/install.js
node --check npm/run.js
(cd npm && npm pack --dry-run)
go build ./...
go vet ./...
go test -race -count=1 ./...
go test -count=1 -tags=smoke,no_web ./tests/e2e/...
go test -count=1 -tags=regression,no_web ./tests/e2e/...
sh feedback-relay/verify-host.sh
sh scripts/verify-agent-app-features.sh
[ "$(git rev-parse HEAD)" = "$expected" ]
[ -z "$(git status --porcelain --untracked-files=normal)" ]
printf 'CHECK PASS sha=%s os=%s go=%s node=%s pnpm=%s\n' "$expected" "$(uname -s)" "$(go env GOVERSION)" "$(node --version)" "$(pnpm --version)"
