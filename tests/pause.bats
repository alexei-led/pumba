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
    [ "$output" = "running" ]
    
    # When pausing the container with pumba (3 seconds duration)
    # Run pumba in background
    pumba pause --duration 3s pause_victim &
    PUMBA_PID=$!
    
    # Then wait for container to be paused
    wait_for 5 "docker inspect -f '{{.State.Status}}' pause_victim | grep -q paused" "container to be paused"
    
    # Verify container is paused
    run docker inspect -f {{.State.Status}} pause_victim
    echo "Container status after pause: $output"
    [ "$output" = "paused" ]
    
    # And container is automatically resumed after duration
    wait_for 10 "docker inspect -f '{{.State.Status}}' pause_victim | grep -q running" "container to be running again"
    
    # Verify container is running again
    run docker inspect -f {{.State.Status}} pause_victim
    echo "Container status after resume: $output"
    [ "$output" = "running" ]
    
    # Clean up the background pumba process if still running
    kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true
}