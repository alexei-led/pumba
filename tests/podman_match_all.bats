#!/usr/bin/env bats

load test_helper
load match_all_helper

setup() {
	match_all_setup podman
}

teardown() {
	match_all_teardown
}

@test "AND iptables selectors affect only matching traffic on podman" {
	match_all_traffic iptables
}

@test "AND netem selectors affect only matching traffic on podman and clean grouped filters" {
	match_all_traffic netem
}

@test "AND netem cleanup verifies merged port keys on podman" {
	match_all_merged_cleanup
}

@test "AND iptables cleans the maximum ruleset within its cleanup budget on podman" {
	match_all_max_rules_cleanup
}
