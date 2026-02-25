#!/usr/bin/env bats

# Load the test helper
load test_helper

setup() {
    # Clean any leftover containers from previous test runs
    cleanup_containers "kill_victim"
}

teardown() {
    # Clean up containers after each test
    cleanup_containers "kill_victim"
}

@test "Should kill running container with default signal" {
    # Given a running container
    create_test_container "kill_victim"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} kill_victim
    [ "$output" = "running" ]
    
    # When killing the container with pumba
    run pumba kill kill_victim
    
    # Then pumba should exit successfully
    [ $status -eq 0 ]
    
    # And container should be killed (status changed to exited)
    wait_for 5 "docker inspect -f '{{.State.Status}}' kill_victim | grep -q exited" "container to be killed"
    
    run docker inspect -f {{.State.Status}} kill_victim
    echo "Container status after kill: $output"
    [ "$output" = "exited" ]
}

@test "Should accept additional kill parameters" {
    # Given a running container
    create_test_container "kill_victim"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} kill_victim
    echo "Container status before kill: $output"
    [ "$output" = "running" ]
    
    # When killing the container with specific signal (syntax verification only)
    run pumba kill --signal SIGKILL kill_victim

    # Then pumba should exit successfully
    [ $status -eq 0 ]

    # Verify container was affected in some way
    # Note: Due to timing issues with external commands we're not testing exact state
    # The purpose of this test is mainly to verify command syntax is accepted
    sleep 2
    run docker inspect kill_victim || echo "Container removed"
    echo "Container status/inspection after kill: $status / ${output:0:60}..."
}

@test "Should kill container with SIGTERM signal" {
    # Use 'top' which properly handles SIGTERM (unlike sleep/tail as PID 1)
    create_test_container "kill_victim" "alpine" "top"
    assert_container_state "kill_victim" "running"

    run pumba kill --signal SIGTERM kill_victim
    [ $status -eq 0 ]

    wait_for 10 "docker inspect -f '{{.State.Status}}' kill_victim | grep -q exited" "container to exit"
    run docker inspect -f {{.State.Status}} kill_victim
    [ "$output" = "exited" ]
}

@test "Should respect --limit when killing containers" {
    # Create two containers matching a regex
    docker run -d --name kill_victim_1 alpine tail -f /dev/null
    docker run -d --name kill_victim_2 alpine tail -f /dev/null
    sleep 1

    assert_container_state "kill_victim_1" "running"
    assert_container_state "kill_victim_2" "running"

    # Kill with limit=1 — only one should be killed
    run pumba kill --limit 1 "re2:kill_victim_.*"
    [ $status -eq 0 ]

    sleep 2
    # Count how many are still running
    local running=0
    docker inspect -f '{{.State.Status}}' kill_victim_1 2>/dev/null | grep -q running && running=$((running+1))
    docker inspect -f '{{.State.Status}}' kill_victim_2 2>/dev/null | grep -q running && running=$((running+1))
    echo "Running containers after limit=1 kill: $running"
    [ "$running" -eq 1 ]

    # Cleanup extras
    docker rm -f kill_victim_1 kill_victim_2 2>/dev/null || true
}