#!/bin/sh
set -eu

relay_root=$(CDPATH='' cd -- "$(dirname "$0")" && pwd)

sh "$relay_root/verify.sh"
cd "$relay_root/.."
CCN_FEEDBACK_CUJ_PAYLOAD=$(mktemp "${TMPDIR:-/tmp}/ccn-feedback-cuj.XXXXXX")
export CCN_FEEDBACK_CUJ_PAYLOAD
trap 'rm -f "$CCN_FEEDBACK_CUJ_PAYLOAD"' EXIT HUP INT TERM
GOWORK=off go test ./core -run '^TestFeedbackLongTurnKeepsOriginalRequestAfterHistoryWindowExpires$' -count=1
node --test internal/appfeatures/feedback_contract.test.mjs
node --test internal/appfeatures/feedback_relay_compat.test.mjs
node --check feedback-relay/src/compat.js
node --check feedback-relay/src/github-app.js
node --check feedback-relay/src/worker.js
cd "$relay_root"
npm exec -- vitest run --config vitest.host.config.js
