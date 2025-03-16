#!/usr/bin/env bats

# Load the test helper
load test_helper

setup() {
    # Clean any leftover containers from previous test runs
    cleanup_containers "rm_victim"
}

teardown() {
    # Clean up containers after each test
    cleanup_containers "rm_victim"
}

@test "Should remove running docker container" {
    # Given a running container
    create_test_container "rm_victim"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} rm_victim
    [ "$output" = "running" ]
    
    # When removing the container with pumba
    run pumba rm rm_victim
    
    # Then pumba should exit successfully
    [ $status -eq 0 ]
    
    # And container should be removed
    run docker ps -a --filter "name=rm_victim" --format "{{.Names}}"
    echo "Remaining containers: $output"
    [ -z "$output" ]
}
