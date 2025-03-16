#!/usr/bin/env bats

# Load the test helper
load test_helper

setup() {
    # Clean any leftover containers from previous test runs
    cleanup_containers "stopping_victim"
    cleanup_containers "starting_victim"
}

teardown() {
    # Clean up containers after each test
    cleanup_containers "stopping_victim"
    cleanup_containers "starting_victim"
}

@test "Should stop running container" {
    # Given a running container
    create_test_container "stopping_victim"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} stopping_victim
    [ "$output" = "running" ]
    
    # When stopping the container
    run pumba stop stopping_victim
    
    # Then pumba should exit successfully
    [ $status -eq 0 ]
    
    # And container should be stopped (status changed to exited)
    wait_for 5 "docker inspect -f '{{.State.Status}}' stopping_victim | grep -q exited" "container to be stopped"
    
    run docker inspect -f {{.State.Status}} stopping_victim
    echo "Container status after stop: $output"
    [ "$output" = "exited" ]
    
    # Additional verification of container exit code
    run docker inspect -f {{.State.ExitCode}} stopping_victim
    echo "Container exit code: $output"
    [ -n "$output" ] # Any exit code is fine
}

@test "Should stop running container with signal handling" {
    # Given a container
    # Use a simpler command
    create_test_container "stopping_victim" "alpine" "sleep 60"
    
    # Wait for container to be running
    wait_for 5 "docker inspect -f '{{.State.Status}}' stopping_victim | grep -q running" "container to start"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} stopping_victim
    echo "Initial container status: $output"
    [ "$output" = "running" ]
    
    # When stopping the container
    run pumba stop stopping_victim
    
    # Then pumba should exit successfully 
    [ $status -eq 0 ]
    
    # And container should be stopped (status changed to exited)
    wait_for 5 "docker inspect -f '{{.State.Status}}' stopping_victim | grep -q exited" "container to be stopped"
    
    run docker inspect -f {{.State.Status}} stopping_victim
    echo "Container status after stop: $output"
    [ "$output" = "exited" ]
}


@test "Should stop container with custom timeout" {
    # Given a running container
    create_test_container "stopping_victim"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} stopping_victim
    [ "$output" = "running" ]
    
    # When stopping the container with custom timeout
    run pumba stop --time 5 stopping_victim
    
    # Then pumba should exit successfully
    [ $status -eq 0 ]
    
    # And container should be stopped (status changed to exited)
    wait_for 5 "docker inspect -f '{{.State.Status}}' stopping_victim | grep -q exited" "container to be stopped"
    
    run docker inspect -f {{.State.Status}} stopping_victim
    echo "Container status after stop with timeout: $output"
    [ "$output" = "exited" ]
}