#!/usr/bin/env bats

# Podman 5.x only: the known 4.9 transient cgroup leaf race remains excluded.
# See podman_stress_inject_cgroup.bats in this directory.
load ../test_helper
load ../revert_helper
load ../revert_stress_helper

bats_require_minimum_version 1.5.0

setup() {
	export RV_RUNTIME="podman"
	revert_setup
}

teardown() { revert_teardown; }

@test "podman recovery kills inject-cgroup workers outside the original sidecar cgroup" {
	revert_stress_case true
}
