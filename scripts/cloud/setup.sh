#!/bin/sh
set -eu
cloud_root=$(CDPATH='' cd -- "$(dirname "$0")/../.." && pwd)
. "$cloud_root/scripts/cloud/env.sh"
for tool in git curl jq tar; do command -v "$tool" >/dev/null; done
go mod download
go mod verify
pnpm --dir web install --frozen-lockfile --store-dir "$cloud_root/.cloud-cache/pnpm"
pnpm --dir web build
npm --prefix feedback-relay ci --ignore-scripts
printf 'SETUP PASS sha=%s os=%s\n' "$(git rev-parse HEAD)" "$(uname -s)"
