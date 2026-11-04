#!/bin/sh
# One bounded auth-reply outage rehearsal. See docs/auth-outage-rehearsal.md.
set -eu

fail() {
	printf 'auth rehearsal: %s\n' "$*" >&2
	exit 2
}

: "${AUTH_IP:?set AUTH_IP to the dedicated staging auth-server IPv4 address}"
: "${AUTH_PORTS:?set AUTH_PORTS to comma-separated auth reply ports}"
: "${TARGET:?set TARGET to one exact staging target name or ID}"
: "${RUN_ID:?set RUN_ID to a known 1-48 character run ID}"

case "$AUTH_IP" in *[!0-9./]* | '') fail 'AUTH_IP must be an IPv4 address or CIDR' ;; esac
case "$AUTH_IP" in *.*) ;; *) fail 'AUTH_IP must be IPv4' ;; esac
case "$AUTH_PORTS" in *[!0-9,]* | ,* | *, | *,,*) fail 'AUTH_PORTS must be comma-separated numeric ports' ;; esac
old_ifs=$IFS
IFS=,
for port in $AUTH_PORTS; do
	[ -n "$port" ] || fail 'empty port in AUTH_PORTS'
	[ "$port" -ge 1 ] 2>/dev/null && [ "$port" -le 65535 ] 2>/dev/null || fail "invalid port: $port"
done
IFS=$old_ifs
[ -n "$TARGET" ] || fail 'TARGET must not be empty'
case "$TARGET" in -* | *' '* | *'\n'*) fail 'TARGET must be one exact container name or ID' ;; esac
case "$RUN_ID" in *[!A-Za-z0-9_.-]* | '') fail 'RUN_ID must use 1-48 letters, digits, _, . or -' ;; esac
[ "${#RUN_ID}" -le 48 ] || fail 'RUN_ID is longer than 48 characters'

protocol=${PROTOCOL:-tcp}
case "$protocol" in tcp | udp) ;; *) fail 'PROTOCOL must be tcp or udp' ;; esac
duration=${DURATION:-30s}
case "$duration" in *[!0-9smh]* | '') fail 'DURATION must be a positive number followed by s, m or h' ;; esac
case "$duration" in *s | *m | *h) ;; *) fail 'DURATION needs s, m or h suffix' ;; esac
# Keep the scenario bounded to at most ten minutes.
case "$duration" in *h) fail 'DURATION must not exceed 10m' ;; *m)
	n=${duration%m}
	[ "$n" -ge 1 ] 2>/dev/null && [ "$n" -le 10 ] 2>/dev/null || fail 'DURATION must be between 1m and 10m'
	duration_seconds=$((n * 60))
	;;
*s)
	n=${duration%s}
	[ "$n" -ge 1 ] 2>/dev/null && [ "$n" -le 600 ] 2>/dev/null || fail 'DURATION must be between 1s and 600s'
	duration_seconds=$n
	;;
esac
max_targets=${MAX_TARGETS:-1}
case "$max_targets" in '' | *[!0-9]*) fail 'MAX_TARGETS must be a nonnegative integer' ;; esac
[ "$max_targets" -gt 0 ] 2>/dev/null || fail 'MAX_TARGETS must be greater than zero'
max_duration=${MAX_DURATION:-15m}
case "$max_duration" in *[!0-9smh]* | '') fail 'MAX_DURATION must be a positive number followed by s, m or h' ;; esac
case "$max_duration" in *m)
	n=${max_duration%m}
	[ "$n" -ge 1 ] 2>/dev/null && [ "$n" -le 15 ] 2>/dev/null || fail 'MAX_DURATION must be between 1m and 15m'
	max_duration_seconds=$((n * 60))
	;;
*) fail 'MAX_DURATION must use minutes (1m-15m)' ;; esac
[ "$max_duration_seconds" -gt "$duration_seconds" ] || fail 'MAX_DURATION must exceed DURATION'

pumba=${PUMBA_BIN:-pumba}
set -- --run-id "$RUN_ID" --max-duration "$max_duration" --max-targets "$max_targets" \
	--output ndjson
[ "${APPLY:-0}" = 1 ] || set -- --dry-run "$@"
exec "$pumba" "$@" iptables --match-all --duration "$duration" \
	--protocol "$protocol" --source "$AUTH_IP" --src-port "$AUTH_PORTS" \
	loss --probability 1.0 "$TARGET"
