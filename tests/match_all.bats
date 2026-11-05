#!/usr/bin/env bats

load test_helper
load match_all_helper

bats_require_minimum_version 1.5.0

setup() {
	match_all_setup docker
}

teardown() {
	match_all_teardown
}

# Start the owner first so this tests a proven startup refusal, not the
# separate inspect/add race in an otherwise empty namespace.
refused_sibling_preserves_fault() {
	local sibling
	local -a selectors=(--target "$MA_PEER_IP" --egress-port 8080)
	if [[ "$1" == AND ]]; then
		selectors=(--match-all "${selectors[@]}")
	fi
	sibling=$(docker run -d --network "container:$MA_TARGET" --name "$MA_RUN-sibling" alpine:latest sleep infinity)
	MA_IDS+=("$sibling")
	"${MA_PUMBA[@]}" netem --interface lo --duration 25s --pull-image=false \
		"${selectors[@]}" loss --percent 100 "$MA_TARGET" >"$MA_EVENTS" 2>"$BATS_TEST_TMPDIR/pumba.log" 3>&- &
	MA_PID=$!
	match_all_wait_applied

	run --separate-stderr "${MA_PUMBA[@]}" --run-id "$MA_RUN-refused" netem --interface lo --duration 1s --pull-image=false \
		"${selectors[@]}" loss --percent 100 "$sibling"
	assert_failure 1
	printf '%s\n' "$output" >"$BATS_TEST_TMPDIR/refused.ndjson"
	run jq -es 'all(.[]; .type != "target.applied") and .[-1].type == "run.end" and .[-1].status == "failed"' "$BATS_TEST_TMPDIR/refused.ndjson"
	assert_success
	assert_output "true"
	kill -0 "$MA_PID"
	run match_all_request "$MA_PEER_IP" 8080
	refute_output --partial "match-all-ok"
	# The active fault still belongs to the first action until its own stop.
	match_all_stop
	run match_all_request "$MA_PEER_IP" 8080
	assert_success
	assert_output --partial "match-all-ok"
	run jq -es '[.[] | select(.type == "target.result")] | length == 1' "$MA_EVENTS"
	assert_success
	assert_output "true"
}

@test "AND iptables selectors affect only matching traffic on docker" {
	match_all_traffic iptables
}

@test "AND netem selectors affect only matching traffic on docker and clean grouped filters" {
	match_all_traffic netem
}

@test "AND netem cleanup verifies merged port keys on docker" {
	match_all_merged_cleanup
}

@test "AND iptables cleans the maximum ruleset within its cleanup budget on docker" {
	match_all_max_rules_cleanup
}

@test "OR netem refused sibling leaves the owner's active fault intact" {
	refused_sibling_preserves_fault OR
}

@test "AND netem refused sibling leaves the owner's active fault intact" {
	refused_sibling_preserves_fault AND
}
