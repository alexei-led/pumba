#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."
load() { :; }
bats_require_minimum_version() { :; }
# shellcheck source=tests/test_helper.bash
source tests/test_helper.bash
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

podman() {
    printf '%s\n' "$*" >>"$TMP/calls"
    case "$*" in
    "image exists test-image") return "$EXISTS_STATUS" ;;
    "pull test-image") return "$PULL_STATUS" ;;
    *)
        echo "unexpected Podman call: $*" >&2
        return 99
        ;;
    esac
}

for scenario in cached missing pull-failure; do
    : >"$TMP/calls"
    EXISTS_STATUS=0
    PULL_STATUS=0
    expected=0
    if [ "$scenario" != cached ]; then EXISTS_STATUS=1; fi
    if [ "$scenario" = pull-failure ]; then
        PULL_STATUS=23
        expected=23
    fi
    actual=0
    podman_pull_if_missing test-image || actual=$?
    [ "$actual" -eq "$expected" ]
    printf '%s\n' 'image exists test-image' >"$TMP/expected"
    if [ "$scenario" != cached ]; then printf '%s\n' 'pull test-image' >>"$TMP/expected"; fi
    diff -u "$TMP/expected" "$TMP/calls"
    printf 'PASS Podman image %s\n' "$scenario"
done
