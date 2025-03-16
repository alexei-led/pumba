#!/usr/bin/env bats

# Load the test helper
load test_helper

setup() {
    # Clean any leftover containers from previous test runs
    cleanup_containers "iptables_target"
    cleanup_containers "iptables_loss_target"
    
    # Also cleanup any nettools containers that might be left running
    docker ps -q --filter "ancestor=ghcr.io/alexei-led/pumba/pumba-alpine-nettools" | xargs -r docker rm -f
}

teardown() {
    # Clean up containers after each test
    cleanup_containers "iptables_target"
    cleanup_containers "iptables_loss_target"
    
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

@test "Should display iptables help" {
    run pumba iptables --help
    [ $status -eq 0 ]
    # Verify help contains expected commands
    [[ $output =~ "loss" ]]
    [[ $output =~ "duration" ]]
    [[ $output =~ "interface" ]]
}

@test "Should display iptables loss help" {
    run pumba iptables loss --help
    [ $status -eq 0 ]
    # Verify help contains loss options
    [[ $output =~ "probability" ]]
    [[ $output =~ "mode" ]]
}

@test "Should fail when Duration is unset for iptables loss" {
    run pumba iptables loss --probability 0.1
    # Should fail with exit code 1
    [ $status -eq 1 ]
    # Verify error message about duration
    [[ ${lines[0]} =~ "unset or invalid duration value" ]]
}

@test "Should handle gracefully when targeting non-existent container" {
    # When targeting a non-existent container
    run pumba iptables --duration 200ms loss --probability 0.1 nonexistent_container
    
    # Then command should succeed (exit code 0)
    [ $status -eq 0 ]
    
    # And output should indicate no containers were found
    echo "Command output: $output"
    [[ $output =~ "no containers found" ]]
}

@test "Should apply packet loss with iptables using external image" {
    # Given a running container
    create_test_container "iptables_loss_target" "alpine" "ping 8.8.8.8"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} iptables_loss_target
    [ "$output" = "running" ]
    
    # Ensure nettools image is available
    ensure_nettools_image
    
    # When applying packet loss with pumba
    echo "Applying packet loss..."
    run pumba -l debug --json iptables --duration 5s --iptables-image ghcr.io/alexei-led/pumba/pumba-alpine-nettools:latest --pull-image=false loss --mode random --probability 1.0 iptables_loss_target
    echo "Full output:"
    echo "$output"
    
    # Then pumba should execute successfully
    echo "Pumba execution status: $status"
    [ $status -eq 0 ]
}

@test "Should apply packet loss with source IP filter" {
    # Given a running container
    create_test_container "iptables_target" "alpine" "ping 8.8.8.8"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} iptables_target
    [ "$output" = "running" ]
    
    # Ensure nettools image is available
    ensure_nettools_image
    
    # When applying packet loss with a source IP filter
    echo "Applying packet loss with source IP filter..."
    run pumba iptables --duration 5s --iptables-image ghcr.io/alexei-led/pumba/pumba-alpine-nettools:latest --pull-image=false --source 192.168.0.1/24 loss --mode random --probability 1.0 iptables_target
    
    # Then pumba should execute successfully
    echo "Pumba execution status: $status"
    [ $status -eq 0 ]
}

@test "Should apply packet loss with destination IP filter" {
    # Given a running container
    create_test_container "iptables_target" "alpine" "ping 8.8.8.8"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} iptables_target
    [ "$output" = "running" ]
    
    # Ensure nettools image is available
    ensure_nettools_image
    
    # When applying packet loss with a destination IP filter
    echo "Applying packet loss with destination IP filter..."
    run pumba iptables --duration 5s --iptables-image ghcr.io/alexei-led/pumba/pumba-alpine-nettools:latest --pull-image=false --destination 8.8.8.8/32 loss --mode random --probability 1.0 iptables_target
    
    # Then pumba should execute successfully
    echo "Pumba execution status: $status"
    [ $status -eq 0 ]
}

@test "Should apply packet loss with port filters" {
    # Given a running container
    create_test_container "iptables_target" "alpine" "ping 8.8.8.8"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} iptables_target
    [ "$output" = "running" ]
    
    # Ensure nettools image is available
    ensure_nettools_image
    
    # When applying packet loss with port filters
    echo "Applying packet loss with port filters..."
    run pumba iptables --duration 5s --iptables-image ghcr.io/alexei-led/pumba/pumba-alpine-nettools:latest --pull-image=false --protocol tcp --dst-port 80,443 loss --mode random --probability 0.5 iptables_target
    
    # Then pumba should execute successfully
    echo "Pumba execution status: $status"
    [ $status -eq 0 ]
}

@test "Should apply packet loss using nth mode" {
    # Given a running container
    create_test_container "iptables_target" "alpine" "ping 8.8.8.8"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} iptables_target
    [ "$output" = "running" ]
    
    # Ensure nettools image is available
    ensure_nettools_image
    
    # When applying packet loss with nth mode
    echo "Applying packet loss with nth mode..."
    run pumba iptables --duration 5s --iptables-image ghcr.io/alexei-led/pumba/pumba-alpine-nettools:latest --pull-image=false loss --mode nth --every 3 --packet 0 iptables_target
    
    # Then pumba should execute successfully
    echo "Pumba execution status: $status"
    [ $status -eq 0 ]
}