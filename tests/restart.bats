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
    assert_output "exited"
    
    # Use docker to start it (pumba restart doesn't work with stopped containers)
    docker start restart_test
    
    # Wait for the container to start
    wait_for 5 "docker inspect -f '{{.State.Status}}' restart_test | grep -q running" "container to start"
    
    # Verify container was started
    run docker inspect -f {{.State.Status}} restart_test
    echo "Container status: $output"
    assert_output "running"
}

@test "Pumba restart should skip stopped containers" {
    # Given a container
    docker run -d --name restart_test alpine tail -f /dev/null
    
    # Stop the container
    docker stop restart_test
    
    # Verify container is stopped
    run docker inspect -f {{.State.Status}} restart_test
    assert_output "exited"
    
    # Use pumba to attempt to restart it
    run pumba restart restart_test
    
    # Pumba should not find any containers to restart and exit without error
    assert_success
    
    # Container should still be stopped
    run docker inspect -f {{.State.Status}} restart_test
    echo "Container status: $output"
    assert_output "exited"
}

@test "Should restart a running container" {
    # Given a running container
    docker run -d --name restart_test alpine tail -f /dev/null
    
    # Get initial start time
    local start_time
    start_time=$(docker inspect -f '{{.State.StartedAt}}' restart_test)
    
    # Use pumba to restart it
    run pumba restart restart_test
    assert_success
    
    # Wait for the container to restart
    wait_for 5 "docker inspect -f '{{.State.StartedAt}}' restart_test | grep -v '$start_time'" "container to have new start time"
    
    # Verify container was restarted
    run docker inspect -f {{.State.Status}} restart_test
    echo "Container status: $output"
    assert_output "running"
    
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
    assert_success
    
    # Wait for restart to complete
    sleep 5
    
    # Verify container was restarted
    run docker inspect -f {{.State.Status}} restart_test
    echo "Container status: $output"
    assert_output "running"
    
    # Get new start time
    local new_start_time
    new_start_time=$(docker inspect -f '{{.State.StartedAt}}' restart_test)
    
    # Compare start times (they should be different)
    echo "Original start time: $start_time"
    echo "New start time: $new_start_time"
    [ "$start_time" != "$new_start_time" ]
}
@test "Should respect --limit when restarting containers" {
    docker run -d --name restart_test_1 alpine tail -f /dev/null
    docker run -d --name restart_test_2 alpine tail -f /dev/null
    sleep 1

    local start_time_1 start_time_2
    start_time_1=$(docker inspect -f '{{.State.StartedAt}}' restart_test_1)
    start_time_2=$(docker inspect -f '{{.State.StartedAt}}' restart_test_2)

    run pumba restart --limit 1 "re2:restart_test_.*"
    assert_success

    sleep 3
    local restarted=0
    local new_time_1 new_time_2
    new_time_1=$(docker inspect -f '{{.State.StartedAt}}' restart_test_1)
    new_time_2=$(docker inspect -f '{{.State.StartedAt}}' restart_test_2)
    [ "$start_time_1" \!= "$new_time_1" ] && restarted=$((restarted+1))
    [ "$start_time_2" \!= "$new_time_2" ] && restarted=$((restarted+1))
    echo "Restarted containers after limit=1: $restarted"
    [ "$restarted" -eq 1 ]

    docker rm -f restart_test_1 restart_test_2 2>/dev/null || true
}
