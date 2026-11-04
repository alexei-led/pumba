#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."
load() { :; }
# shellcheck source=tests/test_helper.bash
source tests/test_helper.bash
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

# This in-process stub never runs through the unused xargs cleanup helper.
# shellcheck disable=SC2032
docker() {
    printf '%s\n' "$*" >>"$TMP/calls"
    case "$1" in
    run)
        [ "$SCENARIO" != failed-create ] || return 23
        local name
        while [ "$1" != --name ]; do shift; done
        name=$2
        if [ "$SCENARIO" = ready ]; then echo running >"$TMP/state-$name"; else echo created >"$TMP/state-$name"; fi
        echo fixture-id
        ;;
    inspect)
        local name=${*: -1}
        printf '%s\n' "$name" >"$TMP/last-inspected"
        cat "$TMP/state-$name"
        ;;
    *)
        echo "unexpected Docker call: $*" >&2
        return 99
        ;;
    esac
}

sleep() {
    printf '%s\n' "$*" >>"$TMP/sleeps"
    if [ "$SCENARIO" = delayed ]; then echo running >"$TMP/state-$(cat "$TMP/last-inspected")"; fi
}

for entrypoint in single labeled; do
    for SCENARIO in ready delayed failed-create timeout; do
        : >"$TMP/calls"
        : >"$TMP/sleeps"
        actual=0
        if [ "$entrypoint" = single ]; then
            create_test_container fixture alpine top || actual=$?
        else
            create_labeled_containers fixture key value 2 || actual=$?
        fi
        case "$SCENARIO" in
        ready)
            [ "$actual" = 0 ]
            grep -q '^inspect .* fixture' "$TMP/calls"
            if [ "$entrypoint" = labeled ]; then
                grep -q '^inspect .* fixture_1$' "$TMP/calls"
                grep -q '^inspect .* fixture_2$' "$TMP/calls"
            fi
            [ ! -s "$TMP/sleeps" ]
            ;;
        delayed)
            [ "$actual" = 0 ]
            grep -q '^inspect .* fixture' "$TMP/calls"
            if [ "$entrypoint" = labeled ]; then
                grep -q '^inspect .* fixture_1$' "$TMP/calls"
                grep -q '^inspect .* fixture_2$' "$TMP/calls"
                [ "$(wc -l <"$TMP/sleeps")" -eq 2 ]
            else
                [ "$(wc -l <"$TMP/sleeps")" -eq 1 ]
            fi
            ;;
        failed-create)
            [ "$actual" = 23 ]
            if grep -q '^inspect ' "$TMP/calls"; then exit 1; fi
            [ ! -s "$TMP/sleeps" ]
            ;;
        timeout)
            [ "$actual" = 1 ]
            grep -q '^inspect ' "$TMP/calls"
            [ -s "$TMP/sleeps" ]
            ;;
        esac
        printf 'PASS %s fixture %s\n' "$entrypoint" "$SCENARIO"
    done
done
