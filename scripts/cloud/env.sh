#!/bin/sh
# Sourced by the cloud entrypoints; never changes HOME or persistent config.
cd "$cloud_root"
export GOWORK=off GOTOOLCHAIN=go1.25.13
export GOPATH="$cloud_root/.cloud-cache/go"
export GOCACHE="$cloud_root/.cloud-cache/go-build"
export GOMODCACHE="$GOPATH/pkg/mod"
export npm_config_cache="$cloud_root/.cloud-cache/npm"
export XDG_CACHE_HOME="$cloud_root/.cloud-cache/xdg"
export WRANGLER_SEND_METRICS=false
mkdir -p "$GOCACHE" "$GOMODCACHE" "$npm_config_cache" "$XDG_CACHE_HOME"
case "$(node --version)" in v22.*) ;; *) echo 'Node 22 is required' >&2; exit 1;; esac
case "$(pnpm --version)" in 10.*) ;; *) echo 'pnpm 10 is required' >&2; exit 1;; esac
[ "$(go env GOVERSION)" = go1.25.13 ] || { echo 'Go 1.25.13 is required' >&2; exit 1; }
