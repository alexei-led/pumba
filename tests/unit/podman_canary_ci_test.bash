#!/usr/bin/env bash
# Exercise the real workflow shell block offline; no runtime is contacted.
set -euo pipefail
cd "$(dirname "$0")/../.."
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
awk '
    /name: Run Podman Integration Tests/ { step=1; next }
    step && /run: \|/ { body=1; next }
    body && /^          / { sub(/^          /, ""); print; next }
    body { exit }
' .github/workflows/build.yaml >"$TMP/step.bash"
[ -s "$TMP/step.bash" ]

sudo() {
    if [ "$1" = env ]; then shift 2; fi
    "$@"
}
podman() {
    echo "$*" >>"$LOG"
    case "$1" in
    run)
        touch "$TMP/canary"
        echo canary-id
        ;;
    inspect)
        local id=${*: -1}
        [ "$id" = canary-id ] || return 44
        if [ "$CANARY" = missing ]; then return 125; fi
        echo "$CANARY"
        ;;
    rm)
        [ "$*" = 'rm -f canary-id' ] || return 42
        rm "$TMP/canary"
        ;;
    *) return 43 ;;
    esac
}
bats() {
    echo 'stub suite'
    return "$BATS_RC"
}

for BATS_RC in 0 17; do
    for CANARY in true false missing; do
        for TEE in success failure; do
            LOG="$TMP/calls.log"
            : >"$LOG"
            export BATS_LOG="$TMP/bats.log"
            [ "$TEE" != failure ] || BATS_LOG="$TMP"
            set +e
            (
                set -e
                # shellcheck source=/dev/null
                source "$TMP/step.bash"
            )
            result=$?
            set -e
            expected=$BATS_RC
            if [ "$TEE" = failure ] || { [ "$BATS_RC" = 0 ] && [ "$CANARY" != true ]; }; then expected=1; fi
            [ "$result" = "$expected" ] || {
                echo "unexpected status $result, wanted $expected" >&2
                exit 1
            }
            [ ! -e "$TMP/canary" ]
            [ "$(grep -c '^rm -f canary-id$' "$LOG")" = 1 ]
            [ "$(grep -c '^inspect ' "$LOG")" = 1 ]
            echo "PASS bats=$BATS_RC canary=$CANARY tee=$TEE exit=$result"
        done
    done
done
