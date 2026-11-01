#!/usr/bin/env bats

load test_helper
load dry_run_plan_helper
bats_require_minimum_version 1.5.0

setup() { plan_setup docker; }
teardown() { plan_teardown; }

@test "docker: netem dry-run plans sidecar and direct exec without changing qdiscs" {
    plan_check_netem
}

@test "docker: iptables dry-run plans tagged add/delete argv without installing rules" {
    plan_check_iptables
}

@test "docker: dry-run commands are visible at the default log level" {
    plan_check_text
}
