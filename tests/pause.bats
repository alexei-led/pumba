#!/usr/bin/env bats

# Load the test helper
load test_helper

setup() {
    # Clean any leftover containers from previous test runs
    cleanup_containers "pause_victim"
}

teardown() {
    # Clean up containers after each test
    cleanup_containers "pause_victim"
}

@test "Should pause running container and resume after duration" {
    # Given a running container
    create_test_container "pause_victim"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} pause_victim
    assert_output "running"
    
    # When pausing the container with pumba (3 seconds duration)
    # Run pumba in background
    pumba pause --duration 3s pause_victim &
    PUMBA_PID=$!
    
    # Then wait for container to be paused
    wait_for 5 "docker inspect -f '{{.State.Status}}' pause_victim | grep -q paused" "container to be paused"
    
    # Verify container is paused
    run docker inspect -f {{.State.Status}} pause_victim
    echo "Container status after pause: $output"
    assert_output "paused"
    
    # And container is automatically resumed after duration
    wait_for 10 "docker inspect -f '{{.State.Status}}' pause_victim | grep -q running" "container to be running again"
    
    # Verify container is running again
    run docker inspect -f {{.State.Status}} pause_victim
    echo "Container status after resume: $output"
    assert_output "running"
    
    # Clean up the background pumba process if still running
    kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true
}
@test "Should respect --limit when pausing containers" {
    docker run -d --name pause_victim_1 alpine tail -f /dev/null
    docker run -d --name pause_victim_2 alpine tail -f /dev/null
    sleep 1

    assert_container_running "pause_victim_1"
    assert_container_running "pause_victim_2"

    pumba pause --limit 1 --duration 5s "re2:pause_victim_.*" &
    PUMBA_PID=$!

    sleep 2
    local paused=0
    docker inspect -f '{{.State.Status}}' pause_victim_1 2>/dev/null | grep -q paused && paused=$((paused+1))
    docker inspect -f '{{.State.Status}}' pause_victim_2 2>/dev/null | grep -q paused && paused=$((paused+1))
    echo "Paused containers after limit=1: $paused"
    [ "$paused" -eq 1 ]

    kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true
    docker rm -f pause_victim_1 pause_victim_2 2>/dev/null || true
}

@test "Should pause for approximately the specified duration" {
    create_test_container "pause_victim"
    assert_container_running "pause_victim"

    # Record start time
    local start_time
    start_time=$(date +%s)

    # Pause for 3 seconds
    pumba pause --duration 3s pause_victim &
    PUMBA_PID=$!

    # Wait for container to be paused
    wait_for 5 "docker inspect -f '{{.State.Status}}' pause_victim | grep -q paused" "container to be paused"

    # Wait for container to be unpaused
    wait_for 10 "docker inspect -f '{{.State.Status}}' pause_victim | grep -q running" "container to be running again"

    local end_time
    end_time=$(date +%s)
    local elapsed=$((end_time - start_time))

    echo "Pause duration: ${elapsed}s (expected ~3s)"
    # Allow ±2s tolerance (3s pause, expect 1-5s total)
    [ "$elapsed" -ge 1 ]
    [ "$elapsed" -le 6 ]

    kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true
}
