#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."
for docs in true false invalid; do
    for changes in success failure cancelled skipped; do
        for unit in success failure cancelled skipped; do
            for integration in success failure cancelled skipped; do
                expected=1
                if [ "$changes" = success ]; then
                    if [ "$docs" = true ] && [ "$unit" = skipped ] && [ "$integration" = skipped ]; then expected=0; fi
                    if [ "$docs" = false ] && [ "$unit" = success ] && [ "$integration" = success ]; then expected=0; fi
                fi
                actual=0
                CHANGES_RESULT="$changes" DOCS_ONLY="$docs" UNIT_RESULT="$unit" INTEGRATION_RESULT="$integration" \
                    bash .github/scripts/ci-gate.bash >/dev/null 2>&1 || actual=$?
                if [ "$actual" -ne "$expected" ]; then
                    printf 'Unexpected gate result: docs=%s changes=%s unit=%s integration=%s\n' "$docs" "$changes" "$unit" "$integration" >&2
                    exit 1
                fi
            done
        done
    done
done
echo 'CI gate result matrix passed'
