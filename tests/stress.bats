#!/usr/bin/env bats

load test_helper

STRESS_IMAGE="ghcr.io/alexei-led/stress-ng:0.20.01"

setup() {
    cleanup_containers "stress_victim"
}

teardown() {
    cleanup_containers "stress_victim"
}

@test "Should display stress help" {
    run pumba stress --help
    assert_success
    assert_output --partial "duration"
    assert_output --partial "stressors"
}

@test "Should handle gracefully when stress targets non-existent container" {
    run pumba stress --duration 1s --stressors="--cpu 1 --cpu-method loop --timeout 1s" nonexistent_xyz
    assert_success
    assert_output --partial "no containers found"
}

@test "Should stress container with CPU stressor" {
    create_test_container "stress_victim"
    assert_container_state "stress_victim" "running"

    # stress-ng runs in a sidecar, using the image preloaded by the workflow.
    run pumba --log-level debug stress --duration 10s --pull-image=false --stress-image "${STRESS_IMAGE}" --stressors="--cpu 1 --cpu-method loop --timeout 3s" stress_victim
    echo "Stress output: $output"
    assert_success
}

@test "Should run stress in dry-run mode" {
    create_test_container "stress_victim"
    assert_container_state "stress_victim" "running"

    run pumba --dry-run stress --duration 5s --stressors="--cpu 1 --cpu-method loop --timeout 3s" stress_victim
    assert_success
}
