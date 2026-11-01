#!/usr/bin/env bash
# Offline boundary regression: source only the real webhook fixture helpers.
# All runtime and process-control calls are stubs; no containers or signals.
# Fixture state is consumed by dynamically sourced helpers.
# shellcheck disable=SC2034
set -euo pipefail
cd "$(dirname "$0")/../.."
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
# Shell functions do not intercept xargs or command-prefixed subprocesses.
mkdir "$TMP/bin"
for boundary in docker podman ctr sudo pkill killall; do
    cat >"$TMP/bin/$boundary" <<'EOF'
#!/usr/bin/env bash
printf 'external %s' "${0##*/}" >>"$LOG"
printf ' %s' "$@" >>"$LOG"
printf '\n' >>"$LOG"
exit 99
EOF
    chmod +x "$TMP/bin/$boundary"
done
export PATH="$TMP/bin:$PATH" LOG
BATS_TEST_NUMBER=1

# Do not load Bats libraries or execute any integration test bodies.
# shellcheck source=/dev/null
source /dev/stdin <<<"$(sed -n '/^setup() {/,/^}/p; /^teardown() {/,/^}/p; /^create_webhook_target() {/,/^}/p; /^start_receiver() {/,/^}/p' tests/webhook.bats)"

skip() { exit 77; }
fail() {
    echo "$*" >&2
    return 1
}
kill() { echo "kill $*" >>"$LOG"; }
wait() { echo "wait $*" >>"$LOG"; }
pkill() { echo "pkill $*" >>"$LOG"; }
killall() { echo "killall $*" >>"$LOG"; }
# Even a regressed prerequisite check must not start a real HTTP receiver.
python3() {
    echo 'unexpected python3 call' >>"$LOG"
    return 99
}
command() {
    if [ "$*" = '-v python3' ] && [ "$SCENARIO" = missing-python ]; then return 1; fi
    builtin command "$@"
}
docker() {
    echo "docker $*" >>"$LOG"
    case "$1" in
    run)
        [ "$SCENARIO" != failed-create ] || return 23
        echo fixture-id
        ;;
    ps) printf '%s\n' protected unrelated fixture-id ;;
    rm) : ;;
    *) return 99 ;;
    esac
}

for SCENARIO in normal empty skipped failed-create receiver-only missing-python; do
    LOG="$TMP/$SCENARIO.log"
    : >"$LOG"
    BATS_TEST_TMPDIR="$TMP/$SCENARIO"
    mkdir "$BATS_TEST_TMPDIR"
    set +e
    (
        set -e
        # Prove setup clears inherited values, including on an early skip.
        WEBHOOK_TARGET_ID=protected
        RECEIVER_PID=unrelated
        setup
        trap teardown EXIT
        case "$SCENARIO" in
        normal)
            create_webhook_target
            [ "$WEBHOOK_TARGET_ID" = fixture-id ]
            RECEIVER_PID=receiver-child
            ;;
        skipped) skip ;;
        failed-create) create_webhook_target ;;
        receiver-only) RECEIVER_PID=receiver-child ;;
        missing-python) start_receiver ;;
        esac
    ) >"$TMP/$SCENARIO.out" 2>"$TMP/$SCENARIO.err"
    result=$?
    set -e
    expected=0
    case "$SCENARIO" in
    skipped) expected=77 ;;
    failed-create) expected=23 ;;
    missing-python) expected=1 ;;
    esac
    [ "$result" = "$expected" ] || {
        echo "$SCENARIO: expected exit $expected, got $result" >&2
        cat "$TMP/$SCENARIO.err" >&2
        exit 1
    }
    # Match every boundary call, not just the expected removal. Broad names,
    # label queries, extra removals, process scans or empty IDs must all fail.
    case "$SCENARIO" in
    normal | failed-create)
        [ "$(grep -c '^docker run ' "$LOG")" = 1 ]
        sed '/^docker run /d' "$LOG" >"$TMP/cleanup.log"
        ;;
    *) cp "$LOG" "$TMP/cleanup.log" ;;
    esac
    expected_calls=''
    case "$SCENARIO" in
    normal) expected_calls=$(printf '%s\n' 'kill receiver-child' 'wait receiver-child' 'docker rm -f fixture-id') ;;
    receiver-only) expected_calls=$(printf '%s\n' 'kill receiver-child' 'wait receiver-child') ;;
    esac
    [ "$(cat "$TMP/cleanup.log")" = "$expected_calls" ] || {
        echo "$SCENARIO: unexpected fixture cleanup" >&2
        cat "$LOG" >&2
        exit 1
    }
    if [ "$SCENARIO" = missing-python ]; then
        grep -q 'python3 is required' "$TMP/$SCENARIO.err"
    fi
    echo "PASS webhook/$SCENARIO"
done
