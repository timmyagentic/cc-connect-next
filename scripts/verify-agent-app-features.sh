#!/bin/sh
set -eu

workspace_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
lock_path="$workspace_root/agent-app-features.lock.json"

test -f "$lock_path"
source_repository=$(jq -r '.source.repository' "$lock_path")
source_commit=$(jq -r '.source.commit' "$lock_path")
module_version=$(jq -r '
  [.features[].deliveries[] | select(.mode == "go-module") | .version]
  | unique
  | if length == 1 then .[0] else error("lock must use one Go module version") end
' "$lock_path")

case "$source_repository" in
  timmyagentic/awesome-agent-app-features) ;;
  *) printf 'unexpected Foundation repository: %s\n' "$source_repository" >&2; exit 1 ;;
esac
printf '%s\n' "$source_commit" | grep -Eq '^[0-9a-f]{40}$'
printf '%s\n' "$module_version" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$'

temporary=$(mktemp -d "${TMPDIR:-/tmp}/ccn-agent-app-features.XXXXXX")
trap 'rm -rf -- "$temporary"' EXIT HUP INT TERM
archive="$temporary/source.tar.gz"
curl --fail --silent --show-error --location --proto '=https' \
  --header 'Accept: application/vnd.github+json' \
  --user-agent 'cc-connect-next-feature-lock/1' \
  "https://api.github.com/repos/$source_repository/tarball/$source_commit" \
  --output "$archive"

tar -tzf "$archive" | awk '
  substr($0, 1, 1) == "/" { exit 1 }
  {
    count = split($0, parts, "/")
    for (i = 1; i <= count; i++) {
      if (parts[i] == "..") exit 1
    }
  }
'
if tar -tvzf "$archive" | awk 'substr($1,1,1) == "l" || substr($1,1,1) == "h" { found=1 } END { exit found ? 0 : 1 }'; then
  printf 'Foundation archive contains a symlink or hardlink\n' >&2
  exit 1
fi

mkdir "$temporary/extracted"
tar -xzf "$archive" -C "$temporary/extracted"
source_root=$(find "$temporary/extracted" -mindepth 1 -maxdepth 1 -type d -print -quit)
test -n "$source_root"

(
  cd "$workspace_root/feedback-relay"
  node --input-type=module -e '
    import fs from "node:fs";
    import Ajv2020 from "ajv/dist/2020.js";
    import addFormats from "ajv-formats";
    const [schemaPath, dataPath] = process.argv.slice(1);
    const ajv = new Ajv2020({allErrors: true, strict: true});
    addFormats(ajv);
    const validate = ajv.compile(JSON.parse(fs.readFileSync(schemaPath, "utf8")));
    const value = JSON.parse(fs.readFileSync(dataPath, "utf8"));
    if (!validate(value)) throw new Error(ajv.errorsText(validate.errors, {separator: "\n"}));
  ' "$source_root/features/integration-lock.schema.json" "$lock_path"
)

module_json=$(cd "$workspace_root" && GOWORK=off go list -m -json github.com/timmyagentic/awesome-agent-app-features)
printf '%s' "$module_json" | jq -e --arg version "$module_version" '
  .Version == $version and (has("Replace") | not)
' >/dev/null

cd "$workspace_root"
GOWORK=off go run \
  "github.com/timmyagentic/awesome-agent-app-features/cmd/feature-lock@$module_version" \
  validate \
  --source "$source_root" \
  --source-commit "$source_commit" \
  --host "$workspace_root" \
  --lock "$lock_path"
