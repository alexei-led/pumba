#!/usr/bin/env bats

load test_helper
load revert_helper
load revert_stress_helper

bats_require_minimum_version 1.5.0

setup() {
	export RV_RUNTIME="podman"
	revert_setup
}

teardown() { revert_teardown; }

@test "podman recovery discovers rules without sidecars, preserves duplicates and other runs" {
	revert_rules_case
}

@test "podman recovery removes exact orphan artifacts but reports unresolved coverage" {
	revert_orphan_case
}

@test "podman recovery never removes unattributable qdiscs" {
	revert_qdisc_case
}

@test "podman recovery deletes each rule through its exact nft or legacy backend" {
	revert_backend_case
}

@test "podman recovery kills a crashed run stress workload" {
	revert_stress_case false
}

@test "podman recovery serially verifies already-absent rules in shared namespaces" {
	revert_shared_namespace_case
}
