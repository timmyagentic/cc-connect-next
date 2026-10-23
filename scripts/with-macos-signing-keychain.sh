#!/bin/bash
# Only the trusted tag-release job supplies these secrets. Never enable tracing.
set +x
set -euo pipefail

if [ "$(uname -s)" != Darwin ]; then
  echo 'macOS release signing requires a macOS runner' >&2
  exit 1
fi
: "${MACOS_CERTIFICATE_P12_BASE64:?Missing macOS signing certificate}"
: "${MACOS_CERTIFICATE_PASSWORD:?Missing macOS certificate password}"
: "${MACOS_SIGN_IDENTITY:?Missing Developer ID Application identity}"
if [ "$#" -eq 0 ]; then
  echo 'Usage: with-macos-signing-keychain.sh command [args...]' >&2
  exit 1
fi

signing_dir="$(mktemp -d "${RUNNER_TEMP:-${TMPDIR:-/tmp}}/ccn-signing.XXXXXX")"
export MACOS_SIGN_KEYCHAIN="$signing_dir/release.keychain-db"
cleanup() {
  security delete-keychain "$MACOS_SIGN_KEYCHAIN" >/dev/null 2>&1 || true
  rm -rf "$signing_dir"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
umask 077
keychain_password="$(openssl rand -hex 32)"
printf '%s' "$MACOS_CERTIFICATE_P12_BASE64" | base64 --decode > "$signing_dir/certificate.p12"
security create-keychain -p "$keychain_password" "$MACOS_SIGN_KEYCHAIN" >/dev/null
security set-keychain-settings -lut 21600 "$MACOS_SIGN_KEYCHAIN"
security unlock-keychain -p "$keychain_password" "$MACOS_SIGN_KEYCHAIN"
security import "$signing_dir/certificate.p12" -P "$MACOS_CERTIFICATE_PASSWORD" \
  -k "$MACOS_SIGN_KEYCHAIN" -T /usr/bin/codesign >/dev/null
security set-key-partition-list -S apple-tool:,apple:,codesign: -s \
  -k "$keychain_password" "$MACOS_SIGN_KEYCHAIN" >/dev/null
rm -f "$signing_dir/certificate.p12"
unset MACOS_CERTIFICATE_P12_BASE64 MACOS_CERTIFICATE_PASSWORD keychain_password
"$@"
