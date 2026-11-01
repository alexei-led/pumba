#!/usr/bin/env bash
# Offline safety check: exercise the real plan suite setup/teardown with CLI
# stubs. No containers, sockets, image pulls, sudo or signals are used.
# sudo is an in-process stub here and intentionally calls shell functions.
# shellcheck disable=SC2032
set -euo pipefail
cd "$(dirname "$0")/../.."
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
# shellcheck source=tests/dry_run_plan_helper.bash
source tests/dry_run_plan_helper.bash

BATS_TEST_NUMBER=1
PUMBA_BIN=bash
require_podman() { :; }
require_containerd() { :; }
ctr_pull_image() { :; }
wait_for_running() { :; }
skip() { exit 77; }
sudo() { "$@"; }

remove_fixture() {
    local id=$1
    echo "remove $id" >>"$LOG"
    awk -v id="$id" '$1 != id' "$STATE" >"$STATE.next"
    mv "$STATE.next" "$STATE"
}

list_sidecars() {
    local target=$1
    awk -v target="$target" '$2 == "true" && $3 == target { print $1 }' "$STATE"
}

engine() {
    local arg target='' id has_skip=false
    case "$1" in
        info) echo "${ROOTLESS:-false}" ;;
        run)
            case " $* " in *" --entrypoint sh "*) ;; *) return 91 ;; esac
            case " $* " in *" --label com.gaiaadm.pumba.skip=false "*) ;; *) return 92 ;; esac
            [ "$SCENARIO" != failed-create ] || return 23
            echo 'fixture-target false -' >>"$STATE"
            echo fixture-target
            ;;
        exec) echo 'unchanged network state' ;;
        ps)
            for arg in "$@"; do
                case "$arg" in
                    label=com.gaiaadm.pumba.target=*) target=${arg#*=}; target=${target#*=} ;;
                    label=com.gaiaadm.pumba.skip=true) has_skip=true ;;
                esac
            done
            [ -n "$target" ] && [ "$has_skip" = true ] || { echo 'unscoped sidecar query' >&2; return 1; }
            list_sidecars "$target"
            ;;
        rm)
            shift
            for id in "$@"; do
                [ "$id" = -f ] || remove_fixture "$id"
            done
            ;;
        *) echo "unexpected engine call: $*" >&2; return 1 ;;
    esac
}
docker() { engine "$@"; }
podman() { engine "$@"; }

ctr() {
    shift 2 # namespace
    local arg target=''
    case "$1 $2" in
        'c create')
            [ "$SCENARIO" != failed-create ] || return 23
            [ "$3" = --privileged ] && [ "$4" = --label ] && [ "$5" = com.gaiaadm.pumba.skip=false ] || return 94
            printf '%s false -\n' "$7" >>"$STATE"
            ;;
        't start'|'t rm'|'snapshots rm') : ;;
        't exec') echo 'unchanged network state' ;;
        'c rm') remove_fixture "$3" ;;
        'c ls')
            # ctr ORs separate selectors; only comma-separated predicates AND.
            for arg in "$@"; do
                case "$arg" in
                    'labels."com.gaiaadm.pumba.skip"==true,labels."com.gaiaadm.pumba.target"=='*)
                        target=${arg##*==}
                        list_sidecars "$target"
                        ;;
                    'labels."com.gaiaadm.pumba.skip"==true')
                        awk '$2 == "true" { print $1 }' "$STATE"
                        ;;
                    'labels."com.gaiaadm.pumba.target"=='*)
                        target=${arg#*==}
                        awk -v target="$target" '$3 == target { print $1 }' "$STATE"
                        ;;
                esac
            done | sort -u
            ;;
        *) echo "unexpected ctr call: $*" >&2; return 1 ;;
    esac
}

for runtime in docker podman containerd; do
    for SCENARIO in normal failed-create rootless; do
        [ "$SCENARIO" != rootless ] || [ "$runtime" = podman ] || continue
        STATE="$TMP/$runtime-$SCENARIO.state"
        LOG="$TMP/$runtime-$SCENARIO.log"
        : >"$LOG"
        printf '%s\n' 'protected true -' 'unrelated true other-target' >"$STATE"
        ROOTLESS=false
        [ "$SCENARIO" != rootless ] || ROOTLESS=true
        set +e
        (
            set -e
            trap plan_teardown EXIT
            plan_setup "$runtime"
            printf '%s\n' "own-sidecar true $PLAN_TARGET_ID" \
                "wrong-label false $PLAN_TARGET_ID" "prefix-target true $PLAN_TARGET_ID-extra" >>"$STATE"
            plan_assert_untouched && exit 42 # a leftover must fail the assertion
            plan_teardown
            PLAN_TARGET_ID=''
        )
        result=$?
        set -e
        case "$SCENARIO" in
            normal) [ "$result" = 0 ] ;;
            failed-create) [ "$result" = 23 ] ;;
            rootless) [ "$result" = 77 ] ;;
        esac
        for id in protected unrelated; do grep -q "^$id " "$STATE"; done
        if [ "$SCENARIO" = normal ]; then
            for id in wrong-label prefix-target; do grep -q "^$id " "$STATE"; done
            if grep -q '^own-sidecar ' "$STATE"; then exit 1; fi
        else
            [ ! -s "$LOG" ]
        fi
    done
done
echo 'dry-run plan fixture boundaries passed'
