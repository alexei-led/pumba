#!/usr/bin/env bats

load test_helper

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
    run pumba stress --duration 1s --stressors="--cpu 1 --timeout 1s" nonexistent_xyz
    assert_success
    assert_output --partial "no containers to stress"
}

@test "Should stress container with CPU stressor" {
    # Create container with stress-ng pre-installed
    docker run -d --name stress_victim alpine sh -c "apk add --no-cache stress-ng >/dev/null 2>&1 && sleep infinity"
    sleep 5
    # Verify stress-ng is installed
    run docker exec stress_victim which stress-ng
    assert_success

    # Run stress for a short time
    run pumba --log-level debug stress --duration 10s --stressors="--cpu 1 --timeout 3s" stress_victim
    echo "Stress output: $output"
    assert_success
}

@test "Should run stress in dry-run mode" {
    create_test_container "stress_victim"
    assert_container_state "stress_victim" "running"

    run pumba --dry-run stress --duration 5s --stressors="--cpu 1 --timeout 3s" stress_victim
    assert_success
}
