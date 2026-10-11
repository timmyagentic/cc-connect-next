#!/bin/sh
# Sourced by the cloud entrypoints; never changes HOME or persistent config.
cd "$cloud_root"
if [ "${CLOUD_TOOLCHAIN_BIN+x}" = x ]; then
  case "$CLOUD_TOOLCHAIN_BIN" in
    /*) ;;
    *) echo 'CLOUD_TOOLCHAIN_BIN must be one absolute directory' >&2; exit 1 ;;
  esac
  case "$CLOUD_TOOLCHAIN_BIN" in
    *:*) echo 'CLOUD_TOOLCHAIN_BIN must not be a PATH list' >&2; exit 1 ;;
  esac
  [ -d "$CLOUD_TOOLCHAIN_BIN" ] || { echo 'CLOUD_TOOLCHAIN_BIN directory does not exist' >&2; exit 1; }
  for cloud_tool in go node npm pnpm; do
    [ -f "$CLOUD_TOOLCHAIN_BIN/$cloud_tool" ] && [ -x "$CLOUD_TOOLCHAIN_BIN/$cloud_tool" ] || {
      printf 'CLOUD_TOOLCHAIN_BIN requires executable %s\n' "$cloud_tool" >&2
      exit 1
    }
  done
  [ "$(GOTOOLCHAIN=local "$CLOUD_TOOLCHAIN_BIN/go" env GOVERSION)" = go1.25.13 ] || {
    echo 'Go 1.25.13 must be preinstalled in CLOUD_TOOLCHAIN_BIN' >&2
    exit 1
  }
  export PATH="$CLOUD_TOOLCHAIN_BIN:$PATH"
fi
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
