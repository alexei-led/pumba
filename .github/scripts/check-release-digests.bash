#!/usr/bin/env bash
set -euo pipefail
: "${GITHUB_OUTPUT:?}"
root=${1:?digest directory}
expected_platforms=2
shopt -s nullglob
ghcr=("$root/ghcr/"*)
hub=("$root/hub/"*)
if [ "${#ghcr[@]}" -ne "$expected_platforms" ]; then
    echo 'Missing GHCR architecture digest' >&2
    exit 1
fi
for file in "${ghcr[@]}" "${hub[@]}"; do
    if [ ! -f "$file" ] || ! [[ "${file##*/}" =~ ^[0-9a-f]{64}$ ]]; then
        echo 'Invalid image digest artifact' >&2
        exit 1
    fi
done
if [ "${#hub[@]}" -eq "$expected_platforms" ]; then
    echo 'hub_ready=true' >>"$GITHUB_OUTPUT"
else
    echo 'hub_ready=false' >>"$GITHUB_OUTPUT"
    echo 'Docker Hub digest set incomplete; leave its tags unchanged' >&2
fi
