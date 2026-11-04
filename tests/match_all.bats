#!/usr/bin/env bats

load test_helper
load match_all_helper

setup() {
	match_all_setup docker
}

teardown() {
	match_all_teardown
}

@test "AND iptables selectors affect only matching traffic on docker" {
	match_all_traffic iptables
}

@test "AND netem selectors affect only matching traffic on docker and clean grouped filters" {
	match_all_traffic netem
}
