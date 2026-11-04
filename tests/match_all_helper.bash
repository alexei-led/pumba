#!/usr/bin/env bash

match_all_setup() {
	command -v jq >/dev/null || return
	MA_RUNTIME=$1
	MA_RUN="match-all-${MA_RUNTIME}-${BATS_TEST_NUMBER}-${BASHPID}"
	MA_IDS=()
	MA_PID=""
	MA_CLI=docker
	MA_PUMBA=(pumba --runtime "$MA_RUNTIME")
	case "$MA_RUNTIME" in
	docker) skip_if_dind ;;
	containerd)
		require_containerd
		MA_PUMBA=(sudo pumba --runtime containerd --containerd-namespace moby)
		;;
	podman)
		require_podman
		MA_CLI=podman
		if podman info --format '{{.Host.Security.Rootless}}' | grep -qi '^true$'; then
			skip "rootful Podman is required"
		fi
		;;
	esac
	"$MA_CLI" image inspect alpine:latest >/dev/null 2>&1 || "$MA_CLI" pull alpine:latest >/dev/null
	"$MA_CLI" image inspect ghcr.io/alexei-led/pumba-alpine-nettools:latest >/dev/null 2>&1 ||
		"$MA_CLI" pull ghcr.io/alexei-led/pumba-alpine-nettools:latest >/dev/null
	local id
	id=$("$MA_CLI" run -d --name "$MA_RUN-target" alpine:latest sh -ec 'mkdir -p /tmp/http; printf "match-all-ok\n" >/tmp/http/index.html; httpd -p 8080 -h /tmp/http; httpd -p 8081 -h /tmp/http; exec sleep infinity') || return
	MA_IDS+=("$id")
	MA_TARGET=$id
	id=$("$MA_CLI" run -d --name "$MA_RUN-peer" alpine:latest sleep infinity) || return
	MA_IDS+=("$id")
	MA_PEER=$id
	id=$("$MA_CLI" run -d --name "$MA_RUN-other" alpine:latest sleep infinity) || return
	MA_IDS+=("$id")
	MA_OTHER=$id
	MA_TARGET_IP=$("$MA_CLI" inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$MA_TARGET") || return
	MA_PEER_IP=$("$MA_CLI" inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$MA_PEER") || return
	[ -n "$MA_TARGET_IP" ] && [ -n "$MA_PEER_IP" ] || return
	MA_EVENTS="$BATS_TEST_TMPDIR/events.ndjson"
	MA_PUMBA+=(--run-id "$MA_RUN" --max-targets 1 --max-duration 30s -o ndjson)
	wait_for 5 "match_all_request '$MA_PEER' 8080 | grep -q match-all-ok" "HTTP fixture readiness"
	run match_all_request "$MA_PEER" 8080
	assert_success
	assert_output --partial "match-all-ok"
	run match_all_request "$MA_PEER" 8081
	assert_success
	assert_output --partial "match-all-ok"
	run match_all_request "$MA_OTHER" 8080
	assert_success
	assert_output --partial "match-all-ok"
}

match_all_request() {
	# The positional parameters are expanded by the container shell.
	# shellcheck disable=SC2016
	"$MA_CLI" exec "$1" sh -ec 'printf "GET / HTTP/1.0\r\n\r\n" | nc -w 1 "$1" "$2"' sh "$MA_TARGET_IP" "$2"
}

match_all_stop() {
	[ -n "$MA_PID" ] || return 0
	if [[ "$MA_RUNTIME" == containerd ]]; then
		sudo pkill -TERM -f "^pumba .*--run-id ${MA_RUN} " 2>/dev/null || true
	else
		kill -TERM "$MA_PID" 2>/dev/null || true
	fi
	local result=0
	wait "$MA_PID" || result=$?
	MA_PID=""
	return "$result"
}

match_all_teardown() {
	if [ -n "${MA_PID:-}" ]; then
		match_all_stop || true
	fi
	if [ "${#MA_IDS[@]}" -gt 0 ]; then
		"$MA_CLI" rm -f "${MA_IDS[@]}" >/dev/null
	fi
}

match_all_traffic() {
	local kind=$1
	case "$kind" in
	iptables)
		"${MA_PUMBA[@]}" iptables --duration 20s --pull-image=false --match-all --protocol tcp --source "$MA_PEER_IP" --dst-port 8080 loss --probability 1 "$MA_TARGET" >"$MA_EVENTS" 2>"$BATS_TEST_TMPDIR/pumba.log" 3>&- &
		;;
	netem)
		"${MA_PUMBA[@]}" netem --duration 20s --pull-image=false --match-all --target "$MA_PEER_IP" --egress-port 8080 loss --percent 100 "$MA_TARGET" >"$MA_EVENTS" 2>"$BATS_TEST_TMPDIR/pumba.log" 3>&- &
		;;
	esac
	MA_PID=$!
	wait_for 10 "jq -e 'select(.type == \"target.applied\")' '$MA_EVENTS' >/dev/null 2>&1" "fault application"

	run match_all_request "$MA_PEER" 8080
	refute_output --partial "match-all-ok"
	run match_all_request "$MA_PEER" 8081
	assert_success
	assert_output --partial "match-all-ok"
	run match_all_request "$MA_OTHER" 8080
	assert_success
	assert_output --partial "match-all-ok"

	match_all_stop
	run jq -es '.[-1].type == "run.end" and .[-1].status == "aborted" and .[-1].counts.cleanup_failed == 0' "$MA_EVENTS"
	assert_success
	assert_output "true"
	run match_all_request "$MA_PEER" 8080
	assert_success
	assert_output --partial "match-all-ok"
}
