#!/usr/bin/env bash
# Offline boundary regression: source the real suite setup/teardown with a
# stateful Podman CLI stub. No runtime, sudo, signals or image pulls are used.
set -euo pipefail
cd "$(dirname "$0")/../.."
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

load() { :; }
bats_require_minimum_version() { :; }
# shellcheck source=tests/test_helper.bash
source tests/test_helper.bash
require_podman() { [ "$SCENARIO" != unavailable ] || skip; }
wait_for() { :; }
wait_for_running() { :; }
pkill() { echo pkill >>"$LOG"; }
kill() { echo kill >>"$LOG"; }
sudo() { "$@"; }
skip() { exit 77; }

podman() {
    echo "$*" >>"$LOG"
    case "$1" in
    info) echo "$ROOTLESS" ;;
    pull) : ;;
    run)
        [ "$SCENARIO" != failed-create ] || return 23
        local id=fixture-id
        if grep -q '^fixture-id ' "$STATE"; then id=fixture-peer; fi
        echo "$id false -" >>"$STATE"
        echo "$id"
        ;;
    ps)
        local skip_filter='' target_filter='' arg
        for arg in "$@"; do
            case "$arg" in
            label=com.gaiaadm.pumba.skip=*) skip_filter=${arg##*=} ;;
            label=com.gaiaadm.pumba.target=*) target_filter=${arg##*=} ;;
            esac
        done
        awk -v s="$skip_filter" -v t="$target_filter" \
            '(s == "" || $2 == s) && (t == "" || $3 == t) { print $1 }' "$STATE"
        ;;
    rm)
        local id
        shift
        for id in "$@"; do
            [ "$id" != -f ] || continue
            awk -v id="$id" '$1 != id' "$STATE" >"$STATE.next"
            mv "$STATE.next" "$STATE"
        done
        ;;
    *)
        echo "unexpected podman call: $*" >&2
        return 1
        ;;
    esac
}

for suite in status sidecar netem iptables combined stress stress_inject_cgroup; do
    file="tests/podman_$suite.bats"
    if [ "$suite" = stress_inject_cgroup ]; then file="tests/skip_ci/podman_$suite.bats"; fi
    for SCENARIO in ${SCENARIOS:-normal rootless unavailable failed-create}; do
        STATE="$TMP/$suite-$SCENARIO.state"
        LOG="$TMP/$suite-$SCENARIO.log"
        printf '%s\n' 'protected true -' 'unrelated true another-target' \
            'wrong-target false fixture-id' 'prefix-target true fixture-id-extra' >"$STATE"
        : >"$LOG"
        ROOTLESS=false
        [ "$SCENARIO" != rootless ] || ROOTLESS=true
        set +e
        (
            set -e
            # Extract only the actual setup and teardown; do not execute Bats
            # test bodies or library loads. A skip still runs teardown via EXIT.
            # shellcheck source=/dev/null
            source /dev/stdin <<<"$(sed '/^setup() {/,$d' "$file")"
            # shellcheck source=/dev/null
            source /dev/stdin <<<"$(sed -n '/^setup() {/,/^}/p; /^teardown() {/,/^}/p' "$file")"
            trap teardown EXIT
            setup
            echo 'fixture-sidecar true fixture-id' >>"$STATE"
            if declare -F podman_fixture_sidecars >/dev/null; then
                podman_fixture_run -d --name peer alpine top
                echo 'fixture-peer-sidecar true fixture-peer' >>"$STATE"
                [ "$(podman_fixture_sidecars)" = "$(printf '%s\n' fixture-sidecar fixture-peer-sidecar)" ]
            fi
        )
        result=$?
        set -e
        case "$SCENARIO" in
        normal) [ "$result" = 0 ] ;;
        rootless | unavailable) [ "$result" = 77 ] ;;
        failed-create) [ "$result" = 23 ] ;;
        esac
        for id in protected unrelated wrong-target prefix-target; do
            grep -q "^$id " "$STATE" || {
                echo "$suite/$SCENARIO deleted $id" >&2
                exit 1
            }
        done
        if [ "$SCENARIO" = normal ]; then
            if grep -q '^fixture-' "$STATE"; then
                echo 'fixtures leaked' >&2
                exit 1
            fi
            if grep '^ps ' "$LOG" | grep -Ev -- '--filter label=com.gaiaadm.pumba.skip=true --filter label=com.gaiaadm.pumba.target=fixture-(id|peer)$'; then
                echo 'unscoped sidecar listing' >&2
                exit 1
            fi
            # Sidecars must be enumerated before the recorded target is removed.
            awk '/^ps / { listed=1 } /^rm -f fixture-id$/ { if (!listed) exit 1; removed=1 } END { if (!removed) exit 1 }' "$LOG"
        else
            if grep -Eq '^(rm |ps |pkill$|kill$)' "$LOG"; then
                echo 'cleanup without fixtures' >&2
                exit 1
            fi
        fi
        echo "PASS $suite/$SCENARIO"
    done
done
