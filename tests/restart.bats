#!/usr/bin/env bats

# Load the test helper
load test_helper

setup() {
    # Clean any leftover containers from previous test runs
    cleanup_containers "restart_test"
}

teardown() {
    # Clean up containers after each test
    cleanup_containers "restart_test"
}

@test "Should start a stopped container from docker directly" {
    # Given a container
    docker run -d --name restart_test alpine tail -f /dev/null
    
    # Stop the container
    docker stop restart_test
    
    # Verify container is stopped
    run docker inspect -f {{.State.Status}} restart_test
    [ "$output" = "exited" ]
    
    # Use docker to start it (pumba restart doesn't work with stopped containers)
    docker start restart_test
    
    # Wait for the container to start
    wait_for 5 "docker inspect -f '{{.State.Status}}' restart_test | grep -q running" "container to start"
    
    # Verify container was started
    run docker inspect -f {{.State.Status}} restart_test
    echo "Container status: $output"
    [ "$output" = "running" ]
}

@test "Pumba restart should skip stopped containers" {
    # Given a container
    docker run -d --name restart_test alpine tail -f /dev/null
    
    # Stop the container
    docker stop restart_test
    
    # Verify container is stopped
    run docker inspect -f {{.State.Status}} restart_test
    [ "$output" = "exited" ]
    
    # Use pumba to attempt to restart it
    run pumba restart restart_test
    
    # Pumba should not find any containers to restart and exit without error
    [ $status -eq 0 ]
    
    # Container should still be stopped
    run docker inspect -f {{.State.Status}} restart_test
    echo "Container status: $output"
    [ "$output" = "exited" ]
}

@test "Should restart a running container" {
    # Given a running container
    docker run -d --name restart_test alpine tail -f /dev/null
    
    # Get initial start time
    local start_time
    start_time=$(docker inspect -f '{{.State.StartedAt}}' restart_test)
    
    # Use pumba to restart it
    run pumba restart restart_test
    [ $status -eq 0 ]
    
    # Wait for the container to restart
    wait_for 5 "docker inspect -f '{{.State.StartedAt}}' restart_test | grep -v '$start_time'" "container to have new start time"
    
    # Verify container was restarted
    run docker inspect -f {{.State.Status}} restart_test
    echo "Container status: $output"
    [ "$output" = "running" ]
    
    # Get new start time
    local new_start_time
    new_start_time=$(docker inspect -f '{{.State.StartedAt}}' restart_test)
    
    # Compare start times (they should be different)
    echo "Original start time: $start_time"
    echo "New start time: $new_start_time"
    [ "$start_time" != "$new_start_time" ]
}

@test "Should restart container with timeout" {
    # Given a running container
    docker run -d --name restart_test alpine tail -f /dev/null
    
    # Get initial start time
    local start_time
    start_time=$(docker inspect -f '{{.State.StartedAt}}' restart_test)
    
    # Use pumba to restart with timeout
    run pumba restart --timeout 3s restart_test
    [ $status -eq 0 ]
    
    # Wait for restart to complete
    sleep 5
    
    # Verify container was restarted
    run docker inspect -f {{.State.Status}} restart_test
    echo "Container status: $output"
    [ "$output" = "running" ]
    
    # Get new start time
    local new_start_time
    new_start_time=$(docker inspect -f '{{.State.StartedAt}}' restart_test)
    
    # Compare start times (they should be different)
    echo "Original start time: $start_time"
    echo "New start time: $new_start_time"
    [ "$start_time" != "$new_start_time" ]
}