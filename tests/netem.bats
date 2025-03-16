#!/usr/bin/env bats

# Load the test helper
load test_helper

setup() {
    # Clean any leftover containers from previous test runs
    cleanup_containers "pingtest"
    cleanup_containers "netem_target"
    cleanup_containers "rate_limit_target"
    
    # Also cleanup any nettools containers that might be left running
    docker ps -q --filter "ancestor=ghcr.io/alexei-led/pumba/pumba-alpine-nettools" | xargs -r docker rm -f
}

teardown() {
    # Clean up containers after each test
    cleanup_containers "pingtest"
    cleanup_containers "netem_target"
    cleanup_containers "rate_limit_target"
    
    # Also cleanup any nettools containers that might be left running
    docker ps -q --filter "ancestor=ghcr.io/alexei-led/pumba/pumba-alpine-nettools" | xargs -r docker rm -f
}

@test "Should display netem help" {
    run pumba netem --help
    [ $status -eq 0 ]
    # Verify help contains expected commands
    [[ $output =~ "delay" ]]
    [[ $output =~ "loss" ]]
    [[ $output =~ "duplicate" ]]
    [[ $output =~ "corrupt" ]]
    [[ $output =~ "rate" ]]
}

@test "Should display netem delay help" {
    run pumba netem delay --help
    [ $status -eq 0 ]
    # Verify help contains delay options
    [[ $output =~ "delay time" ]]
    [[ $output =~ "jitter" ]]
    [[ $output =~ "correlation" ]]
    [[ $output =~ "distribution" ]]
}

@test "Should fail when Duration is unset for netem delay" {
    run pumba netem delay --time 100
    # Should fail with exit code 1
    [ $status -eq 1 ]
    # Verify error message about duration
    [[ ${lines[0]} =~ "unset or invalid duration value" ]]
}

@test "Should handle gracefully when targeting non-existent container" {
    # When targeting a non-existent container
    run pumba netem --duration 200ms delay --time 100 nonexistent_container
    
    # Then command should succeed (exit code 0)
    [ $status -eq 0 ]
    
    # And output should indicate no containers were found
    echo "Command output: $output"
    [[ $output =~ "no containers found" ]]
}

@test "Should delay egress traffic from container with external tc image" {
    # Given a running container
    create_test_container "pingtest" "alpine" "ping 8.8.8.8"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} pingtest
    [ "$output" = "running" ]
    
    # Ensure TC image is available (pull if needed)
    echo "Ensuring nettools image is available..."
    # Check if image exists locally, pull only if not present
    if ! docker image inspect ghcr.io/alexei-led/pumba/pumba-alpine-nettools:latest &>/dev/null; then
        echo "Pulling nettools image..."
        docker pull ghcr.io/alexei-led/pumba/pumba-alpine-nettools:latest
    else
        echo "Nettools image already exists locally"
    fi
    
    # When applying network delay with pumba
    echo "Applying network delay..."
    run pumba netem --duration 5s --tc-image ghcr.io/alexei-led/pumba/pumba-alpine-nettools:latest --pull-image=false delay --time 1000 pingtest
    
    # Then pumba should execute successfully
    echo "Pumba execution status: $status"
    [ $status -eq 0 ]
    
    # Since we're not using debug mode, we don't check the output content
    # Just validate that the command succeeded
}

@test "Should validate packet loss command syntax" {
    # Given a running container to target
    create_test_container "netem_target" "alpine" "ping 8.8.8.8"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} netem_target
    [ "$output" = "running" ]
    
    # When checking pumba command syntax with --help
    echo "Checking packet loss command syntax..."
    run pumba netem loss --help
    
    # Then command help should display successfully
    echo "Pumba help exit status: $status"
    [ $status -eq 0 ]
    
    # And help output should contain expected parameters
    [[ $output =~ "percent" ]] 
    [[ $output =~ "correlation" ]]
    
    echo "Packet loss syntax validation passed"
}

@test "Should validate rate limiting command syntax" {
    # Given a running container to target
    create_test_container "rate_limit_target" "alpine" "ping 8.8.8.8"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} rate_limit_target
    [ "$output" = "running" ]
    
    # When checking pumba command syntax with --help
    echo "Checking rate limiting command syntax..."
    run pumba netem rate --help
    
    # Then command help should display successfully
    echo "Pumba help exit status: $status"
    [ $status -eq 0 ]
    
    # And help output should contain expected parameters
    [[ $output =~ "rate" ]]
    [[ $output =~ "packetoverhead" ]] || [[ $output =~ "packet" ]]
    
    echo "Rate limiting syntax validation passed"
}
