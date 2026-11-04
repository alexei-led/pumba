#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
script="$root/examples/auth_outage_rehearsal.sh"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

cat >"$tmp/pumba" <<'STUB'
#!/bin/sh
printf '%s\n' "$@" >"$ARGS_FILE"
[ "${STUB_SIGNAL:-}" = 1 ] || exit "${STUB_EXIT:-0}"
printf ready >"$READY_FILE"
trap 'printf signal >"$SIGNAL_FILE"; exit 143' TERM INT
while :; do sleep 1; done
STUB
chmod +x "$tmp/pumba"
export PUMBA_BIN="$tmp/pumba" ARGS_FILE="$tmp/args" READY_FILE="$tmp/ready" SIGNAL_FILE="$tmp/signal"
export AUTH_IP=192.0.2.7 AUTH_PORTS=1812,1813 TARGET=staging-radius-1 RUN_ID=auth-test-1 PROTOCOL=udp

"$script"
cat >"$tmp/expected" <<'EXPECTED'
--dry-run
--run-id
auth-test-1
--max-duration
15m
--max-targets
1
--output
ndjson
iptables
--match-all
--duration
30s
--protocol
udp
--source
192.0.2.7
--src-port
1812,1813
loss
--probability
1.0
staging-radius-1
EXPECTED
diff -u "$tmp/expected" "$ARGS_FILE"

APPLY=1 DURATION=2m MAX_DURATION=5m MAX_TARGETS=2 "$script"
if grep -Fx -- '--dry-run' "$ARGS_FILE" >/dev/null; then
	echo 'apply mode unexpectedly included --dry-run' >&2
	exit 1
fi
for arg in --max-targets 2 --max-duration 5m 2m; do grep -Fx -- "$arg" "$ARGS_FILE" >/dev/null; done

for missing in AUTH_IP AUTH_PORTS TARGET RUN_ID; do
	if env -u "$missing" "$script" >/dev/null 2>&1; then
		echo "expected missing $missing to fail" >&2
		exit 1
	fi
done
if AUTH_IP=192.0.2.7 AUTH_PORTS=1812 TARGET=staging RUN_ID=bad! "$script" >/dev/null 2>&1; then
	echo 'expected invalid run ID to fail' >&2
	exit 1
fi
if AUTH_IP=192.0.2.7 AUTH_PORTS=1812 TARGET=staging RUN_ID=ok PROTOCOL=sctp "$script" >/dev/null 2>&1; then
	echo 'expected invalid protocol to fail' >&2
	exit 1
fi

for invalid_target in 're2:.*' $'staging\tother' $'staging\nother' '-flag'; do
	rm -f "$ARGS_FILE"
	if TARGET="$invalid_target" "$script" >/dev/null 2>&1; then
		echo "unsafe target accepted: $invalid_target" >&2
		exit 1
	fi
	[ ! -e "$ARGS_FILE" ] || {
		echo 'invalid target invoked Pumba' >&2
		exit 1
	}
done

DURATION=08m MAX_DURATION=09m "$script"
grep -Fx -- 08m "$ARGS_FILE" >/dev/null
grep -Fx -- 09m "$ARGS_FILE" >/dev/null

set +e
STUB_EXIT=17 "$script" >/dev/null 2>&1
rc=$?
set -e
[ "$rc" -eq 17 ] || {
	echo "wrong propagated status: $rc" >&2
	exit 1
}

STUB_SIGNAL=1 "$script" >/dev/null 2>&1 &
pid=$!
for _ in {1..50}; do
	[ -e "$READY_FILE" ] && break
	sleep 0.02
done
[ -e "$READY_FILE" ] || {
	echo 'fake Pumba did not start' >&2
	exit 1
}
kill -TERM "$pid"
set +e
wait "$pid"
rc=$?
set -e
[ "$rc" -eq 143 ] && [ -e "$SIGNAL_FILE" ] || {
	echo "signal did not propagate (exit=$rc)" >&2
	exit 1
}

echo 'auth outage rehearsal argv, validation, exit and signal tests passed'
