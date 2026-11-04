#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
for ghcr_count in 1 2; do
    for hub_count in 0 1 2 3; do
        dir="$TMP/$ghcr_count-$hub_count"
        mkdir -p "$dir/ghcr" "$dir/hub"
        for ((i = 1; i <= ghcr_count; i++)); do touch "$dir/ghcr/$(printf '%064d' "$i")"; done
        for ((i = 1; i <= hub_count; i++)); do touch "$dir/hub/$(printf '%064d' "$i")"; done
        actual=0
        GITHUB_OUTPUT="$dir/output" bash .github/scripts/check-release-digests.bash "$dir" >/dev/null 2>&1 || actual=$?
        if [ "$ghcr_count" = 1 ]; then
            [ "$actual" != 0 ]
            continue
        fi
        [ "$actual" = 0 ]
        expected=false
        if [ "$hub_count" = 2 ]; then expected=true; fi
        [ "$(cat "$dir/output")" = "hub_ready=$expected" ]
    done
done
touch "$TMP/2-2/ghcr/not-a-digest"
if GITHUB_OUTPUT="$TMP/bad-output" bash .github/scripts/check-release-digests.bash "$TMP/2-2" >/dev/null 2>&1; then exit 1; fi
echo 'Release requires both architectures and never publishes a partial Hub manifest'
