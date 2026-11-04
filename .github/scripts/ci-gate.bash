#!/usr/bin/env bash
set -euo pipefail
if [ "${CHANGES_RESULT:-}" != success ]; then
    echo 'Change classification did not succeed' >&2
    exit 1
fi
case "${DOCS_ONLY:-}" in
true) expected=skipped ;;
false) expected=success ;;
*)
    echo 'Missing or invalid change classification' >&2
    exit 1
    ;;
esac
for result in "${UNIT_RESULT:-}" "${INTEGRATION_RESULT:-}"; do
    if [ "$result" != "$expected" ]; then
        printf 'Required job result %s; expected %s\n' "$result" "$expected" >&2
        exit 1
    fi
done
