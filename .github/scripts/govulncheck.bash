#!/usr/bin/env bash
# Fail-closed symbol gate for govulncheck v1.9.0. See docs/security.md.
set -euo pipefail
trap 'echo "govulncheck gate failed" >&2' ERR
SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)

status=0
govulncheck -json -scan=symbol ./... >govulncheck.json || status=$?
printf 'govulncheck raw JSON scanner exit: %s\n' "$status"
# JSON success exits 0 even with findings; text-mode exit 3 is not valid here.
if [ "$status" -ne 0 ]; then
    echo 'govulncheck gate failed: JSON mode requires exit 0; scanner failed or exit contract changed' >&2
    exit 1
fi

# jq also accepts non-JSON numeric spellings (NaN, +1, 01, etc.). Check the
# lexical grammar first; jq's parser then checks structure and serialization.
jq -Rne '
    all(inputs; test("^(?:\"(?:[^\"\\\\\\x00-\\x1f]|\\\\(?:[\"\\\\/bfnrt]|u[0-9a-fA-F]{4}))*\"|(?:true|false|null|-?(?:0|[1-9][0-9]*)(?:\\.[0-9]+)?(?:[eE][+-]?[0-9]+)?)(?![A-Za-z0-9_.+-])|[{}\\[\\],: \\t\\r\\n])*$"))
' govulncheck.json >/dev/null
# jq normally overwrites duplicate object keys. Reject them before that can
# hide an inconsistent finding. A closed path may not be visited again.
jq --stream -ne '
    reduce inputs as $event ({seen: {}, open: {}};
      if ($event | length) == 1 then
        if ($event[0] | length) <= 1 then {seen: {}, open: {}}
        else ($event[0][0:-1] | tojson) as $path | .open |= del(.[$path]) end
      else
        reduce range(1; ($event[0] | length) + 1) as $depth (.;
          ($event[0][0:$depth] | tojson) as $path
          | if .seen[$path] and (.open[$path] | not) then error("duplicate JSON key")
            else .seen[$path] = true
              | if $depth < ($event[0] | length) then .open[$path] = true else . end
            end)
      end) | true
' govulncheck.json >/dev/null
jq -ers -f "$SCRIPT_DIR/govulncheck.jq" govulncheck.json
