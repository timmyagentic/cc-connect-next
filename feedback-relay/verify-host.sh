#!/bin/sh
set -eu

relay_root=$(CDPATH='' cd -- "$(dirname "$0")" && pwd)

sh "$relay_root/verify.sh"
cd "$relay_root/.."
node --test internal/appfeatures/feedback_contract.test.mjs
node --test internal/appfeatures/feedback_relay_compat.test.mjs
node --check feedback-relay/src/compat.js
node --check feedback-relay/src/github-app.js
cd "$relay_root"
npm exec -- vitest run --config vitest.host.config.js
