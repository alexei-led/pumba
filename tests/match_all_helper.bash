#!/usr/bin/env bash

match_all_setup() {
	MA_RUNTIME=$1
	MA_RUN="match-all-${MA_RUNTIME}-${BATS_TEST_NUMBER}-${BASHPID}"
	MA_IDS=()
	MA_PID=""
	MA_EXEC_ID=0
	MA_CLI=docker
	MA_TARGET_IP=127.0.0.1
	MA_PEER_IP=127.0.0.2
	MA_OTHER_IP=127.0.0.3
	MA_PUMBA=(pumba --runtime "$MA_RUNTIME")
	command -v jq >/dev/null || return
	case "$MA_RUNTIME" in
	docker) skip_if_dind ;;
	containerd)
		require_containerd
		MA_PUMBA+=(--containerd-namespace moby)
		if [[ "$EUID" -ne 0 ]]; then
			MA_PUMBA=(sudo "${MA_PUMBA[@]}")
		fi
		ctr_pull_image moby docker.io/library/alpine:latest
		ctr_pull_image moby ghcr.io/alexei-led/pumba-alpine-nettools:latest
		;;
	podman)
		require_podman
		MA_CLI=podman
		if podman info --format '{{.Host.Security.Rootless}}' | grep -qi '^true$'; then
			skip "rootful Podman is required"
		fi
		;;
	esac
	# Namespace identity and positional parameters are checked inside the fixture.
	# shellcheck disable=SC2016
	local server='[ "$(readlink /proc/self/ns/net)" != "$1" ] || exit 1; ip link set lo up; nc -lk -p 8080 -e /bin/cat & nc -lk -p 8081 -e /bin/cat & wait'
	if [[ "$MA_RUNTIME" == containerd ]]; then
		MA_TARGET="$MA_RUN-target"
		sudo ctr -n moby run -d --privileged docker.io/library/alpine:latest "$MA_TARGET" sh -ec "$server" sh "$(readlink /proc/self/ns/net)" || return
	else
		"$MA_CLI" image inspect alpine:latest >/dev/null 2>&1 || "$MA_CLI" pull alpine:latest >/dev/null
		"$MA_CLI" image inspect ghcr.io/alexei-led/pumba-alpine-nettools:latest >/dev/null 2>&1 ||
			"$MA_CLI" pull ghcr.io/alexei-led/pumba-alpine-nettools:latest >/dev/null
		MA_TARGET=$("$MA_CLI" run -d --network none --cap-add NET_ADMIN --name "$MA_RUN-target" alpine:latest sh -ec "$server" sh "$(readlink /proc/self/ns/net)") || return
	fi
	MA_IDS+=("$MA_TARGET")
	local target_ns
	target_ns=$(match_all_exec readlink /proc/self/ns/net) || return
	# Never shape the host namespace, even if a runtime fixture default changes.
	[ "$target_ns" != "$(readlink /proc/self/ns/net)" ] || return 1
	MA_EVENTS="$BATS_TEST_TMPDIR/events.ndjson"
	MA_PUMBA+=(--run-id "$MA_RUN" --max-targets 1 --max-duration 30s -o ndjson)
	wait_for 5 "match_all_request '$MA_PEER_IP' 8080 | grep -q match-all-ok" "TCP echo fixture readiness"
	run match_all_request "$MA_PEER_IP" 8080
	assert_success
	assert_output --partial "match-all-ok"
	run match_all_request "$MA_PEER_IP" 8081
	assert_success
	assert_output --partial "match-all-ok"
	run match_all_request "$MA_OTHER_IP" 8080
	assert_success
	assert_output --partial "match-all-ok"
}

match_all_exec() {
	if [[ "$MA_RUNTIME" == containerd ]]; then
		MA_EXEC_ID=$((MA_EXEC_ID + 1))
		sudo ctr -n moby t exec --exec-id "$MA_RUN-$BASHPID-$MA_EXEC_ID" "$MA_TARGET" "$@"
	else
		"$MA_CLI" exec "$MA_TARGET" "$@"
	fi
}

match_all_request() {
	# The positional parameters are expanded by the container shell.
	# shellcheck disable=SC2016
	match_all_exec sh -ec 'printf "match-all-ok\n" | nc -s "$1" -w 1 "$2" "$3"' sh "$1" "$MA_TARGET_IP" "$2"
}

match_all_stop() {
	[ -n "$MA_PID" ] || return 0
	if [[ "$MA_RUNTIME" == containerd && "$EUID" -ne 0 ]]; then
		sudo pkill -TERM -f "^pumba .*--run-id ${MA_RUN} " 2>/dev/null || true
	else
		kill -TERM "$MA_PID" 2>/dev/null || true
	fi
	local result=0
	wait "$MA_PID" || result=$?
	MA_PID=""
	if [ "$result" -ne 0 ]; then
		cat "$BATS_TEST_TMPDIR/pumba.log" >&2 || true
	fi
	return "$result"
}

match_all_teardown() {
	if [ -n "${MA_PID:-}" ]; then
		match_all_stop || true
	fi
	local id
	for id in "${MA_IDS[@]}"; do
		if [[ "$MA_RUNTIME" == containerd ]]; then
			sudo ctr -n moby t kill -s SIGKILL "$id" >/dev/null 2>&1 || true
			sudo ctr -n moby t rm "$id" >/dev/null 2>&1 || true
			sudo ctr -n moby c rm "$id" >/dev/null 2>&1 || true
			sudo ctr -n moby snapshots rm "$id" >/dev/null 2>&1 || true
		else
			"$MA_CLI" rm -f "$id" >/dev/null
		fi
	done
}

match_all_traffic() {
	local kind=$1
	case "$kind" in
	iptables)
		"${MA_PUMBA[@]}" iptables --interface lo --duration 20s --pull-image=false --match-all --protocol tcp --source "$MA_PEER_IP" --dst-port 8080 loss --probability 1 "$MA_TARGET" >"$MA_EVENTS" 2>"$BATS_TEST_TMPDIR/pumba.log" 3>&- &
		;;
	netem)
		"${MA_PUMBA[@]}" netem --interface lo --duration 20s --pull-image=false --match-all --target "$MA_PEER_IP" --egress-port 8080 loss --percent 100 "$MA_TARGET" >"$MA_EVENTS" 2>"$BATS_TEST_TMPDIR/pumba.log" 3>&- &
		;;
	esac
	MA_PID=$!
	match_all_wait_applied
	run match_all_request "$MA_PEER_IP" 8080
	refute_output --partial "match-all-ok"
	run match_all_request "$MA_PEER_IP" 8081
	assert_success
	assert_output --partial "match-all-ok"
	run match_all_request "$MA_OTHER_IP" 8080
	assert_success
	assert_output --partial "match-all-ok"
	match_all_stop
	run jq -es '.[-1].type == "run.end" and .[-1].status == "aborted" and .[-1].counts.cleanup_failed == 0' "$MA_EVENTS"
	assert_success
	assert_output "true"
	run match_all_request "$MA_PEER_IP" 8080
	assert_success
	assert_output --partial "match-all-ok"
}

match_all_wait_applied() {
	if ! wait_for 40 "jq -e 'select(.type == \"target.applied\")' '$MA_EVENTS' >/dev/null 2>&1" "fault application"; then
		cat "$BATS_TEST_TMPDIR/pumba.log" >&2
		return 1
	fi
}

match_all_merged_cleanup() {
	"${MA_PUMBA[@]}" netem --interface lo --duration 20s --pull-image=false --match-all \
		--target "$MA_PEER_IP" --egress-port 8080 --ingress-port 12345 \
		loss --percent 100 "$MA_TARGET" >"$MA_EVENTS" 2>"$BATS_TEST_TMPDIR/pumba.log" 3>&- &
	MA_PID=$!
	match_all_wait_applied
	match_all_stop
	run "${MA_PUMBA[@]}" status --pull-image=false --check --run-id "$MA_RUN" "$MA_TARGET"
	assert_success
	assert_output --partial '"status":"clean"'
}

match_all_max_rules_cleanup() {
	local ports="" port rule_limit=256 snapshot started
	for ((port = 0; port < rule_limit; port++)); do
		ports="${ports}${ports:+,}$port"
	done
	"${MA_PUMBA[@]}" --max-duration 2m iptables --interface lo --duration 1m --pull-image=false --match-all \
		--protocol tcp --source "$MA_PEER_IP" --dst-port "$ports" \
		loss --probability 1 "$MA_TARGET" >"$MA_EVENTS" 2>"$BATS_TEST_TMPDIR/pumba.log" 3>&- &
	MA_PID=$!
	match_all_wait_applied
	snapshot=$("${MA_PUMBA[@]}" status --pull-image=false --run-id "$MA_RUN" "$MA_TARGET")
	[ "$(printf '%s\n' "$snapshot" | jq -s '[.[] | select(.type == "target.result") | .state.rules[]] | length')" -eq "$rule_limit" ]
	started=$SECONDS
	match_all_stop
	[ "$((SECONDS - started))" -le 35 ]
	run "${MA_PUMBA[@]}" status --pull-image=false --check --run-id "$MA_RUN" "$MA_TARGET"
	assert_success
	assert_output --partial '"status":"clean"'
}
