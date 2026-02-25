#\!/usr/bin/env bats

# Load the test helper
load test_helper

setup() {
    skip_if_dind
    # Clean any leftover containers from previous test runs
    cleanup_containers "iptables_target"
    cleanup_containers "iptables_loss_target"

    # Also cleanup any nettools containers that might be left running
    docker ps -q --filter "ancestor=ghcr.io/alexei-led/pumba-alpine-nettools" | xargs -r docker rm -f
}

teardown() {
    # Stop any pumba processes targeting our test containers
    sudo pkill -f "pumba.*iptables_target" 2>/dev/null || true
    sudo pkill -f "pumba.*iptables_loss_target" 2>/dev/null || true

    # Clean up containers after each test
    cleanup_containers "iptables_target"
    cleanup_containers "iptables_loss_target"
    
    # Also cleanup any nettools containers that might be left running
    docker ps -q --filter "ancestor=ghcr.io/alexei-led/pumba-alpine-nettools" | xargs -r docker rm -f
}

@test "Should display iptables help" {
    run pumba iptables --help
    assert_success
    # Verify help contains expected commands
    assert_output --partial "loss"
    assert_output --partial "duration"
    assert_output --partial "interface"
}

@test "Should display iptables loss help" {
    run pumba iptables loss --help
    assert_success
    # Verify help contains loss options
    assert_output --partial "probability"
    assert_output --partial "mode"
}

@test "Should fail when Duration is unset for iptables loss" {
    run pumba iptables loss --probability 0.1
    # Should fail with exit code 1
    assert_failure
    # Verify error message about duration
    assert_line --index 0 --partial "unset or invalid duration value"
}

@test "Should handle gracefully when targeting non-existent container" {
    # When targeting a non-existent container
    run pumba iptables --duration 200ms loss --probability 0.1 nonexistent_container
    
    # Then command should succeed (exit code 0)
    assert_success
    
    # And output should indicate no containers were found
    echo "Command output: $output"
    assert_output --partial "no containers found"
}

@test "Should apply packet loss with iptables using external image" {
    # Given a running container
    create_test_container "iptables_loss_target" "alpine" "sleep infinity"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} iptables_loss_target
    assert_output "running"
    
    # Ensure nettools image is available
    ensure_nettools_image
    
    # When applying packet loss with pumba in background
    pumba -l debug --json iptables --duration 10s --iptables-image ${NETTOOLS_IMAGE} --pull-image=false loss --mode random --probability 1.0 iptables_loss_target &
    PUMBA_PID=$!

    # Then iptables DROP rules should be applied
    wait_for 5 "nsenter -t \$(docker inspect -f '{{.State.Pid}}' iptables_loss_target) -n iptables -L INPUT -n -v 2>/dev/null | grep -qi DROP" "iptables rules to be applied"

    assert_iptables_applied "iptables_loss_target" "DROP"

    # Wait for pumba to finish and verify clean exit
    wait $PUMBA_PID
    local pumba_exit=$?
    [ $pumba_exit -eq 0 ]

    # Verify cleanup
    assert_iptables_cleaned "iptables_loss_target"
    assert_sidecar_cleaned
}

@test "Should apply packet loss with source IP filter" {
    # Given a running container
    create_test_container "iptables_target" "alpine" "sleep infinity"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} iptables_target
    assert_output "running"
    
    # Ensure nettools image is available
    ensure_nettools_image
    
    # When applying packet loss with a source IP filter in background
    pumba iptables --duration 10s --iptables-image ${NETTOOLS_IMAGE} --pull-image=false --source 127.0.0.1/32 loss --mode random --probability 1.0 iptables_target &
    PUMBA_PID=$!

    # Then iptables DROP rules with source IP should be applied
    wait_for 5 "nsenter -t \$(docker inspect -f '{{.State.Pid}}' iptables_target) -n iptables -L INPUT -n -v 2>/dev/null | grep -qi DROP" "iptables rules to be applied"

    assert_iptables_applied "iptables_target" "DROP"
    assert_iptables_applied "iptables_target" "127.0.0.1"

    # Wait for pumba to finish and verify clean exit
    wait $PUMBA_PID
    local pumba_exit=$?
    [ $pumba_exit -eq 0 ]

    # Verify cleanup
    assert_iptables_cleaned "iptables_target"
    assert_sidecar_cleaned
}

@test "Should apply packet loss with destination IP filter" {
    # Given a running container
    create_test_container "iptables_target" "alpine" "sleep infinity"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} iptables_target
    assert_output "running"
    
    # Ensure nettools image is available
    ensure_nettools_image
    
    # When applying packet loss with a destination IP filter in background
    pumba iptables --duration 10s --iptables-image ${NETTOOLS_IMAGE} --pull-image=false --destination 127.0.0.1/32 loss --mode random --probability 1.0 iptables_target &
    PUMBA_PID=$!

    # Then iptables DROP rules with destination IP should be applied
    wait_for 5 "nsenter -t \$(docker inspect -f '{{.State.Pid}}' iptables_target) -n iptables -L INPUT -n -v 2>/dev/null | grep -qi DROP" "iptables rules to be applied"

    assert_iptables_applied "iptables_target" "DROP"
    assert_iptables_applied "iptables_target" "127.0.0.1"

    # Wait for pumba to finish and verify clean exit
    wait $PUMBA_PID
    local pumba_exit=$?
    [ $pumba_exit -eq 0 ]

    # Verify cleanup
    assert_iptables_cleaned "iptables_target"
    assert_sidecar_cleaned
}

@test "Should apply packet loss with port filters" {
    # Given a running container
    create_test_container "iptables_target" "alpine" "sleep infinity"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} iptables_target
    assert_output "running"
    
    # Ensure nettools image is available
    ensure_nettools_image
    
    # When applying packet loss with port filters in background
    pumba iptables --duration 10s --iptables-image ${NETTOOLS_IMAGE} --pull-image=false --protocol tcp --dst-port 80,443 loss --mode random --probability 0.5 iptables_target &
    PUMBA_PID=$!

    # Then iptables DROP rules with tcp protocol should be applied
    wait_for 5 "nsenter -t \$(docker inspect -f '{{.State.Pid}}' iptables_target) -n iptables -L INPUT -n -v 2>/dev/null | grep -qi DROP" "iptables rules to be applied"

    assert_iptables_applied "iptables_target" "DROP"
    assert_iptables_applied "iptables_target" "tcp"

    # Wait for pumba to finish and verify clean exit
    wait $PUMBA_PID
    local pumba_exit=$?
    [ $pumba_exit -eq 0 ]

    # Verify cleanup
    assert_iptables_cleaned "iptables_target"
    assert_sidecar_cleaned
}

@test "Should apply packet loss using nth mode" {
    # Given a running container
    create_test_container "iptables_target" "alpine" "sleep infinity"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} iptables_target
    assert_output "running"
    
    # Ensure nettools image is available
    ensure_nettools_image
    
    # When applying packet loss with nth mode in background
    pumba iptables --duration 10s --iptables-image ${NETTOOLS_IMAGE} --pull-image=false loss --mode nth --every 3 --packet 0 iptables_target &
    PUMBA_PID=$!

    # Then iptables DROP rules with statistic module should be applied
    wait_for 5 "nsenter -t \$(docker inspect -f '{{.State.Pid}}' iptables_target) -n iptables -L INPUT -n -v 2>/dev/null | grep -qi DROP" "iptables rules to be applied"

    assert_iptables_applied "iptables_target" "DROP"
    assert_iptables_applied "iptables_target" "statistic"

    # Wait for pumba to finish and verify clean exit
    wait $PUMBA_PID
    local pumba_exit=$?
    [ $pumba_exit -eq 0 ]

    # Verify cleanup
    assert_iptables_cleaned "iptables_target"
    assert_sidecar_cleaned
}

@test "Should clean up sidecar containers after iptables completes" {
    create_test_container "iptables_target" "alpine" "sleep infinity"
    assert_container_running "iptables_target"
    ensure_nettools_image

    run pumba iptables --duration 3s --iptables-image ${NETTOOLS_IMAGE} --pull-image=false loss --mode random --probability 0.5 iptables_target
    assert_success

    assert_sidecar_cleaned
}
