#!/usr/bin/env bats

# Load the test helper
load test_helper

setup() {
    # Clean any leftover containers from previous test runs
    cleanup_containers "netem_ext_target"
    
    # Also cleanup any nettools containers that might be left running
    docker ps -q --filter "ancestor=ghcr.io/alexei-led/pumba/pumba-alpine-nettools" | xargs -r docker rm -f
}

teardown() {
    # Clean up containers after each test
    cleanup_containers "netem_ext_target"
    
    # Also cleanup any nettools containers that might be left running
    docker ps -q --filter "ancestor=ghcr.io/alexei-led/pumba/pumba-alpine-nettools" | xargs -r docker rm -f
}

# Helper function to ensure nettools image is available
ensure_nettools_image() {
    echo "Ensuring nettools image is available..."
    # Check if image exists locally, pull only if not present
    if ! docker image inspect ghcr.io/alexei-led/pumba/pumba-alpine-nettools:latest &>/dev/null; then
        echo "Pulling nettools image..."
        docker pull ghcr.io/alexei-led/pumba/pumba-alpine-nettools:latest
    else
        echo "Nettools image already exists locally"
    fi
}

@test "Should verify netem duplicate help" {
    run pumba netem duplicate --help
    [ $status -eq 0 ]
    # Verify help contains expected options
    [[ $output =~ "duplicate" ]]
    [[ $output =~ "percent" ]]
    [[ $output =~ "correlation" ]]
}

@test "Should verify netem corrupt help" {
    run pumba netem corrupt --help
    [ $status -eq 0 ]
    # Verify help contains expected options 
    [[ $output =~ "corrupt" ]]
    [[ $output =~ "percent" ]]
    [[ $output =~ "correlation" ]]
}

@test "Should verify netem loss-state help" {
    run pumba netem loss-state --help
    [ $status -eq 0 ]
    # Verify help contains expected options
    [[ $output =~ "loss-state" ]]
    [[ $output =~ "p13" ]]
    [[ $output =~ "p31" ]]
}

@test "Should verify netem loss-gemodel help" {
    run pumba netem loss-gemodel --help
    [ $status -eq 0 ]
    # Verify help contains expected options
    [[ $output =~ "loss-gemodel" ]]
    [[ $output =~ "pg" ]]
    [[ $output =~ "pb" ]]
}

@test "Should apply packet duplicate with netem" {
    # Given a running container
    create_test_container "netem_ext_target" "alpine" "ping 8.8.8.8"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} netem_ext_target
    [ "$output" = "running" ]
    
    # Ensure nettools image is available
    ensure_nettools_image
    
    # When applying packet duplication with pumba
    echo "Applying packet duplication..."
    run pumba netem --duration 2s --tc-image ghcr.io/alexei-led/pumba/pumba-alpine-nettools:latest --pull-image=false duplicate --percent 10 netem_ext_target
    
    # Then command should succeed
    echo "Packet duplication status: $status"
    [ $status -eq 0 ]
}

@test "Should apply packet corruption with netem" {
    # Given a running container
    create_test_container "netem_ext_target" "alpine" "ping 8.8.8.8"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} netem_ext_target
    [ "$output" = "running" ]
    
    # Ensure nettools image is available
    ensure_nettools_image
    
    # When applying packet corruption with pumba
    echo "Applying packet corruption..."
    run pumba netem --duration 2s --tc-image ghcr.io/alexei-led/pumba/pumba-alpine-nettools:latest --pull-image=false corrupt --percent 5 netem_ext_target
    
    # Then command should succeed
    echo "Packet corruption status: $status"
    [ $status -eq 0 ]
}

@test "Should apply packet loss with correlation" {
    # Given a running container
    create_test_container "netem_ext_target" "alpine" "ping 8.8.8.8"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} netem_ext_target
    [ "$output" = "running" ]
    
    # Ensure nettools image is available
    ensure_nettools_image
    
    # When applying packet loss with correlation
    echo "Applying packet loss with correlation..."
    run pumba netem --duration 2s --tc-image ghcr.io/alexei-led/pumba/pumba-alpine-nettools:latest --pull-image=false loss --percent 20 --correlation 75 netem_ext_target
    
    # Then command should succeed
    echo "Packet loss with correlation status: $status"
    [ $status -eq 0 ]
}

@test "Should apply advanced loss models (loss-state)" {
    # Given a running container
    create_test_container "netem_ext_target" "alpine" "ping 8.8.8.8"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} netem_ext_target
    [ "$output" = "running" ]
    
    # Ensure nettools image is available
    ensure_nettools_image
    
    # When applying advanced loss-state model
    echo "Applying loss-state model..."
    run pumba netem --duration 2s --tc-image ghcr.io/alexei-led/pumba/pumba-alpine-nettools:latest --pull-image=false loss-state --p13 5 --p31 15 --p32 10 --p23 20 --p14 5 netem_ext_target
    
    # Then command should succeed
    echo "Loss-state model status: $status"
    [ $status -eq 0 ]
}

@test "Should apply loss-gemodel model" {
    # Given a running container
    create_test_container "netem_ext_target" "alpine" "ping 8.8.8.8"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} netem_ext_target
    [ "$output" = "running" ]
    
    # Ensure nettools image is available
    ensure_nettools_image
    
    # When applying loss-gemodel model
    echo "Applying loss-gemodel..."
    run pumba netem --duration 2s --tc-image ghcr.io/alexei-led/pumba/pumba-alpine-nettools:latest --pull-image=false loss-gemodel --pg 5 --pb 20 --one-h 80 --one-k 10 netem_ext_target
    
    # Then command should succeed
    echo "Loss-gemodel status: $status"
    [ $status -eq 0 ]
}

@test "Should apply delay with distribution options" {
    # Given a running container
    create_test_container "netem_ext_target" "alpine" "ping 8.8.8.8"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} netem_ext_target
    [ "$output" = "running" ]
    
    # Ensure nettools image is available
    ensure_nettools_image
    
    # When applying delay with normal distribution
    echo "Applying delay with normal distribution..."
    run pumba netem --duration 2s --tc-image ghcr.io/alexei-led/pumba/pumba-alpine-nettools:latest --pull-image=false delay --time 100 --jitter 30 --distribution normal netem_ext_target
    
    # Then command should succeed
    echo "Delay with normal distribution status: $status"
    [ $status -eq 0 ]
    
    # When applying delay with pareto distribution
    echo "Applying delay with pareto distribution..."
    run pumba netem --duration 2s --tc-image ghcr.io/alexei-led/pumba/pumba-alpine-nettools:latest --pull-image=false delay --time 100 --jitter 30 --distribution pareto netem_ext_target
    
    # Then command should succeed
    echo "Delay with pareto distribution status: $status"
    [ $status -eq 0 ]
    
    # When applying delay with paretonormal distribution
    echo "Applying delay with paretonormal distribution..."
    run pumba netem --duration 2s --tc-image ghcr.io/alexei-led/pumba/pumba-alpine-nettools:latest --pull-image=false delay --time 100 --jitter 30 --distribution paretonormal netem_ext_target
    
    # Then command should succeed
    echo "Delay with paretonormal distribution status: $status"
    [ $status -eq 0 ]
}

@test "Should apply rate limit with cell options" {
    # Given a running container
    create_test_container "netem_ext_target" "alpine" "ping 8.8.8.8"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} netem_ext_target
    [ "$output" = "running" ]
    
    # Ensure nettools image is available
    ensure_nettools_image
    
    # When applying rate limit with cell size options
    echo "Applying rate limit with cell options..."
    run pumba netem --duration 2s --tc-image ghcr.io/alexei-led/pumba/pumba-alpine-nettools:latest --pull-image=false rate --rate 1mbit --packetoverhead 10 --cellsize 1500 --celloverhead 20 netem_ext_target
    
    # Then command should succeed
    echo "Rate limit with cell options status: $status"
    [ $status -eq 0 ]
}

@test "Should support custom interface parameter" {
    # Given a running container
    create_test_container "netem_ext_target" "alpine" "ping 8.8.8.8"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} netem_ext_target
    [ "$output" = "running" ]
    
    # Ensure nettools image is available
    ensure_nettools_image
    
    # When applying netem with custom interface parameter
    echo "Applying netem with custom interface..."
    run pumba netem --duration 2s --tc-image ghcr.io/alexei-led/pumba/pumba-alpine-nettools:latest --pull-image=false --interface eth0 delay --time 100 netem_ext_target
    
    # Then command should succeed
    echo "Custom interface parameter status: $status"
    [ $status -eq 0 ]
}