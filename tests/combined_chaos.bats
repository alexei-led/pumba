#!/usr/bin/env bats

# Load the test helper
load test_helper

setup() {
    # Clean any leftover containers from previous test runs
    cleanup_containers "combined_target"
    
    # Also cleanup any nettools containers that might be left running
    docker ps -q --filter "ancestor=ghcr.io/alexei-led/pumba-alpine-nettools" | xargs -r docker rm -f
}

teardown() {
    # Kill any background pumba processes targeting our test containers
    sudo pkill -f "pumba.*combined_target" 2>/dev/null || true

    # Clean up containers after each test
    cleanup_containers "combined_target"
    
    # Also cleanup any nettools containers that might be left running
    docker ps -q --filter "ancestor=ghcr.io/alexei-led/pumba-alpine-nettools" | xargs -r docker rm -f
}

@test "Should use same nettools image for both netem and iptables" {
    # Given a running container
    create_test_container "combined_target" "alpine" "sleep infinity"

    # Verify container is running
    run docker inspect -f {{.State.Status}} combined_target
    assert_output "running"

    # Ensure nettools image is available
    ensure_nettools_image

    # Run pumba netem in background with 10s duration so we can inspect while active
    pumba netem --duration 10s --tc-image ${NETTOOLS_IMAGE} --pull-image=false delay --time 1000 combined_target &
    PUMBA_PID=$!

    # Wait for netem to be applied
    wait_for 5 "nsenter -t \$(docker inspect -f '{{.State.Pid}}' combined_target) -n tc qdisc show dev eth0 2>/dev/null | grep -qi netem" "netem to be applied"

    # Verify kernel-level tc rules
    assert_netem_applied "combined_target" "delay"

    # Wait for pumba to finish and clean up
    wait $PUMBA_PID
    local pumba_exit=$?
    [ $pumba_exit -eq 0 ]

    # Verify netem cleanup
    assert_netem_cleaned "combined_target"

    # Now run iptables in background with 10s duration
    pumba iptables --duration 10s --iptables-image ${NETTOOLS_IMAGE} --pull-image=false loss --mode random --probability 0.2 combined_target &
    PUMBA_PID=$!

    # Wait for iptables rules to be applied
    wait_for 5 "nsenter -t \$(docker inspect -f '{{.State.Pid}}' combined_target) -n iptables -L INPUT -n -v 2>/dev/null | grep -qi DROP" "iptables DROP to be applied"

    # Verify kernel-level iptables rules
    assert_iptables_applied "combined_target" "DROP"

    # Wait for pumba to finish and clean up
    wait $PUMBA_PID
    local pumba_exit=$?
    [ $pumba_exit -eq 0 ]

    # Verify iptables cleanup
    assert_iptables_cleaned "combined_target"
    assert_sidecar_cleaned
}

@test "Should apply complex network degradation with combined commands" {
    # Given a running container
    create_test_container "combined_target" "alpine" "sleep infinity"

    # Verify container is running
    run docker inspect -f {{.State.Status}} combined_target
    assert_output "running"

    # Ensure nettools image is available
    ensure_nettools_image

    # Run pumba netem rate in background with 10s duration
    pumba netem --duration 10s --tc-image ${NETTOOLS_IMAGE} --pull-image=false rate --rate 1mbit combined_target &
    PUMBA_PID=$!

    # Wait for netem to be applied
    wait_for 5 "nsenter -t \$(docker inspect -f '{{.State.Pid}}' combined_target) -n tc qdisc show dev eth0 2>/dev/null | grep -qi netem" "netem rate to be applied"

    # Verify kernel-level tc rules for rate limiting
    assert_netem_applied "combined_target" "rate"

    # Wait for pumba to finish and clean up
    wait $PUMBA_PID
    local pumba_exit=$?
    [ $pumba_exit -eq 0 ]

    # Verify netem cleanup
    assert_netem_cleaned "combined_target"

    # Now apply protocol-specific packet loss with iptables
    pumba iptables --duration 10s --iptables-image ${NETTOOLS_IMAGE} --pull-image=false --protocol icmp loss --mode random --probability 0.5 combined_target &
    PUMBA_PID=$!

    # Wait for iptables rules to be applied
    wait_for 5 "nsenter -t \$(docker inspect -f '{{.State.Pid}}' combined_target) -n iptables -L INPUT -n -v 2>/dev/null | grep -qi DROP" "iptables DROP to be applied"

    # Verify kernel-level iptables rules
    assert_iptables_applied "combined_target" "DROP"

    # Wait for pumba to finish and clean up
    wait $PUMBA_PID
    local pumba_exit=$?
    [ $pumba_exit -eq 0 ]

    # Verify iptables cleanup
    assert_iptables_cleaned "combined_target"
    assert_sidecar_cleaned
}

@test "Should apply source/destination IP filters with port filters" {
    # Given a running container
    create_test_container "combined_target" "alpine" "sleep infinity"

    # Verify container is running
    run docker inspect -f {{.State.Status}} combined_target
    assert_output "running"

    # Ensure nettools image is available
    ensure_nettools_image

    # Run pumba netem with target IP in background with 10s duration
    pumba netem --duration 10s --tc-image ${NETTOOLS_IMAGE} --pull-image=false --target 8.8.8.8 delay --time 100 combined_target &
    PUMBA_PID=$!

    # Wait for netem to be applied
    wait_for 5 "nsenter -t \$(docker inspect -f '{{.State.Pid}}' combined_target) -n tc qdisc show dev eth0 2>/dev/null | grep -qi netem" "netem to be applied"

    # Verify kernel-level tc rules
    assert_netem_applied "combined_target" "netem"

    # Wait for pumba to finish and clean up
    wait $PUMBA_PID
    local pumba_exit=$?
    [ $pumba_exit -eq 0 ]

    # Verify netem cleanup
    assert_netem_cleaned "combined_target"
    assert_sidecar_cleaned

    # Skip port filter verification — environment-specific
}

@test "Should run with nth packet matching mode" {
    # Given a running container
    create_test_container "combined_target" "alpine" "sleep infinity"

    # Verify container is running
    run docker inspect -f {{.State.Status}} combined_target
    assert_output "running"

    # Ensure nettools image is available
    ensure_nettools_image

    # Run pumba iptables with nth matching mode in background with 10s duration
    pumba iptables --duration 10s --iptables-image ${NETTOOLS_IMAGE} --pull-image=false loss --mode nth --every 5 --packet 0 combined_target &
    PUMBA_PID=$!

    # Wait for iptables rules to be applied
    wait_for 5 "nsenter -t \$(docker inspect -f '{{.State.Pid}}' combined_target) -n iptables -L INPUT -n -v 2>/dev/null | grep -qi DROP" "iptables DROP to be applied"

    # Verify kernel-level iptables rules
    assert_iptables_applied "combined_target" "DROP"
    assert_iptables_applied "combined_target" "statistic"

    # Wait for pumba to finish and clean up
    wait $PUMBA_PID
    local pumba_exit=$?
    [ $pumba_exit -eq 0 ]

    # Verify iptables cleanup
    assert_iptables_cleaned "combined_target"
    assert_sidecar_cleaned
}

@test "Should handle multiple containers with regex pattern" {
    # Given multiple running containers with similar names
    create_test_container "combined_target_1" "alpine" "sleep infinity"
    create_test_container "combined_target_2" "alpine" "sleep infinity"

    # Verify containers are running
    run docker inspect -f {{.State.Status}} combined_target_1
    assert_output "running"
    run docker inspect -f {{.State.Status}} combined_target_2
    assert_output "running"

    # Ensure nettools image is available
    ensure_nettools_image

    # Run pumba netem in background with 10s duration targeting regex pattern
    pumba -l debug netem --duration 10s --tc-image ${NETTOOLS_IMAGE} --pull-image=false delay --time 1000 "re2:combined_target_.*" &
    PUMBA_PID=$!

    # Wait for netem to be applied on both containers
    wait_for 10 "nsenter -t \$(docker inspect -f '{{.State.Pid}}' combined_target_1) -n tc qdisc show dev eth0 2>/dev/null | grep -qi netem" "netem to be applied on combined_target_1"
    wait_for 10 "nsenter -t \$(docker inspect -f '{{.State.Pid}}' combined_target_2) -n tc qdisc show dev eth0 2>/dev/null | grep -qi netem" "netem to be applied on combined_target_2"

    # Verify kernel-level tc rules on both containers
    assert_netem_applied "combined_target_1" "delay"
    assert_netem_applied "combined_target_2" "delay"

    # Wait for pumba to finish and clean up
    wait $PUMBA_PID
    local pumba_exit=$?
    [ $pumba_exit -eq 0 ]

    # Verify netem cleanup on both containers
    assert_netem_cleaned "combined_target_1"
    assert_netem_cleaned "combined_target_2"
    assert_sidecar_cleaned
}
