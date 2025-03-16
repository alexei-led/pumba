#!/usr/bin/env bats

# Load the test helper
load test_helper

setup() {
    # Clean any leftover containers from previous test runs
    cleanup_containers "multi_test"
    cleanup_containers "multi_test_other"
    cleanup_containers "multi_test_random"
    cleanup_containers "multi_test_interval"
    cleanup_containers "multi_test_labeled"
}

teardown() {
    # Clean up containers after each test
    cleanup_containers "multi_test"
    cleanup_containers "multi_test_other"
    cleanup_containers "multi_test_random"
    cleanup_containers "multi_test_interval"
    cleanup_containers "multi_test_labeled"
}

@test "Should stop multiple containers using regex pattern" {
    # Given multiple running containers with similar names
    for i in {1..3}; do
        create_test_container "multi_test_$i"
    done
    
    # Verify all containers are running
    for i in {1..3}; do
        run docker inspect -f {{.State.Status}} multi_test_$i
        [ "$output" = "running" ]
    done
    
    # When stopping containers using regex pattern
    run pumba stop "re2:^multi_test_"
    
    # Then pumba should exit successfully
    [ $status -eq 0 ]
    
    # And all containers should be stopped
    for i in {1..3}; do
        run docker inspect -f {{.State.Status}} multi_test_$i
        [ "$output" = "exited" ]
    done
}

@test "Should only affect containers with matching labels" {
    # Given multiple containers with different labels
    # Create containers with label test=true
    create_labeled_containers "multi_test" "test" "true" 2
    
    # Create containers with label test=false 
    create_labeled_containers "multi_test_other" "test" "false" 2
    
    # Verify all containers are running
    for i in {1..2}; do
        run docker inspect -f {{.State.Status}} multi_test_$i
        [ "$output" = "running" ]
        
        run docker inspect -f {{.State.Status}} multi_test_other_$i
        [ "$output" = "running" ]
    done
    
    # When targeting containers with specific label
    run pumba --label test=true kill "re2:multi_test"
    
    # Then pumba should exit successfully
    [ $status -eq 0 ]
    
    # And only containers with matching label should be affected
    for i in {1..2}; do
        # Use wait_for to make sure the state change has time to take effect
        wait_for 5 "docker inspect -f '{{.State.Status}}' multi_test_$i | grep -q exited" "container multi_test_$i to be killed"
        
        run docker inspect -f {{.State.Status}} multi_test_$i
        echo "Container multi_test_$i status: $output"
        [ "$output" = "exited" ]
    done
    
    # And containers with non-matching label should not be affected
    for i in {1..2}; do
        run docker inspect -f {{.State.Status}} multi_test_other_$i
        echo "Container multi_test_other_$i status: $output"
        [ "$output" = "running" ]
    done
    
    # Cleanup is handled by teardown
}

@test "Should randomly select a container when using --random flag" {
    # Given multiple running containers with similar names
    for i in {1..5}; do
        create_test_container "multi_test_random_$i"
    done
    
    # Verify all containers are running
    for i in {1..5}; do
        run docker inspect -f {{.State.Status}} multi_test_random_$i
        [ "$output" = "running" ]
    done
    
    # When killing a random container
    run pumba --random kill "re2:multi_test_random"
    
    # Then pumba should exit successfully
    [ $status -eq 0 ]
    
    echo "Checking which containers were affected..."
    
    # Count how many containers were killed (should be exactly 1)
    local killed_count=0
    for i in {1..5}; do
        # Give containers time to change state
        sleep 1
        
        # Check container state
        container_state=$(docker inspect -f {{.State.Status}} multi_test_random_$i 2>/dev/null || echo "removed")
        echo "Container multi_test_random_$i state: $container_state"
        
        if [[ "$container_state" == "exited" ]]; then
            killed_count=$((killed_count+1))
        fi
    done
    
    echo "Total killed containers: $killed_count"
    
    # Verify exactly one container was killed
    [ $killed_count -eq 1 ]
}

@test "Should run chaos on interval with --interval flag" {
    # Given a running container
    create_test_container "multi_test_interval"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} multi_test_interval
    [ "$output" = "running" ]
    
    # When running pumba with interval (run pumba in background)
    # The interval flag is a global option to run commands repeatedly
    echo "Starting interval-based pumba command..."
    pumba --interval=2s kill multi_test_interval &
    PUMBA_PID=$!
    
    # Then after first interval, container should be killed
    echo "Waiting for first interval to execute..."
    wait_for 5 "docker inspect -f '{{.State.Status}}' multi_test_interval | grep -q exited" "container to be killed in first interval"
    
    # Verify container is exited
    run docker inspect -f {{.State.Status}} multi_test_interval
    echo "Container status after first interval: $output"
    [ "$output" = "exited" ]
    
    # Start the container again to test next interval
    docker start multi_test_interval
    echo "Container restarted manually"
    
    # Wait for container to be running
    wait_for 5 "docker inspect -f '{{.State.Status}}' multi_test_interval | grep -q running" "container to be running again"
    
    # After second interval, container should be killed again
    echo "Waiting for second interval to execute..."
    wait_for 5 "docker inspect -f '{{.State.Status}}' multi_test_interval | grep -q exited" "container to be killed in second interval"
    
    # Verify container is exited again
    run docker inspect -f {{.State.Status}} multi_test_interval
    echo "Container status after second interval: $output"
    [ "$output" = "exited" ]
    
    # Clean up the background pumba process
    echo "Stopping interval-based pumba process..."
    kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true
}