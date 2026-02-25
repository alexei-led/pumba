#!/usr/bin/env bats

# Load the test helper
load test_helper

setup() {
    skip_if_dind
    # Clean any leftover containers from previous test runs
    cleanup_containers "pingtest"
    cleanup_containers "netem_target"
    cleanup_containers "rate_limit_target"

    # Also cleanup any nettools containers that might be left running
    docker ps -q --filter "ancestor=ghcr.io/alexei-led/pumba-alpine-nettools" | xargs -r docker rm -f
}

teardown() {
    # Kill any background pumba processes targeting our test containers
    sudo pkill -f "pumba.*pingtest" 2>/dev/null || true

    # Clean up containers after each test
    cleanup_containers "pingtest"
    cleanup_containers "netem_target"
    cleanup_containers "rate_limit_target"
    
    # Also cleanup any nettools containers that might be left running
    docker ps -q --filter "ancestor=ghcr.io/alexei-led/pumba-alpine-nettools" | xargs -r docker rm -f
}

@test "Should display netem help" {
    run pumba netem --help
    assert_success
    # Verify help contains expected commands
    assert_output --partial "delay"
    assert_output --partial "loss"
    assert_output --partial "duplicate"
    assert_output --partial "corrupt"
    assert_output --partial "rate"
}

@test "Should display netem delay help" {
    run pumba netem delay --help
    assert_success
    # Verify help contains delay options
    assert_output --partial "delay time"
    assert_output --partial "jitter"
    assert_output --partial "correlation"
    assert_output --partial "distribution"
}

@test "Should fail when Duration is unset for netem delay" {
    run pumba netem delay --time 100
    # Should fail with exit code 1
    assert_failure
    # Verify error message about duration
    assert_line --index 0 --partial "unset or invalid duration value"
}

@test "Should handle gracefully when targeting non-existent container" {
    # When targeting a non-existent container
    run pumba netem --duration 200ms delay --time 100 nonexistent_container
    
    # Then command should succeed (exit code 0)
    assert_success
    
    # And output should indicate no containers were found
    echo "Command output: $output"
    assert_output --partial "no containers found"
}

@test "Should delay egress traffic from container with external tc image" {
    # Given a running container
    create_test_container "pingtest" "alpine" "sleep infinity"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} pingtest
    assert_output "running"
    
    # Ensure TC image is available
    ensure_nettools_image

    # Run pumba in background with longer duration so we can inspect while active
    pumba netem --duration 10s --tc-image ${NETTOOLS_IMAGE} --pull-image=false delay --time 1000 pingtest &
    PUMBA_PID=$!

    # Wait for netem to be applied
    wait_for 5 "nsenter -t \$(docker inspect -f '{{.State.Pid}}' pingtest) -n tc qdisc show dev eth0 2>/dev/null | grep -qi netem" "netem to be applied"

    # Verify kernel-level tc rules
    assert_netem_applied "pingtest" "delay"

    # Wait for pumba to finish and clean up
    wait $PUMBA_PID
    local pumba_exit=$?
    [ $pumba_exit -eq 0 ]

    # Verify cleanup
    assert_netem_cleaned "pingtest"
    assert_sidecar_cleaned
}

@test "Should validate packet loss command syntax" {
    # Given a running container to target
    create_test_container "netem_target" "alpine" "sleep infinity"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} netem_target
    assert_output "running"
    
    # When checking pumba command syntax with --help
    echo "Checking packet loss command syntax..."
    run pumba netem loss --help
    
    # Then command help should display successfully
    echo "Pumba help exit status: $status"
    assert_success
    
    # And help output should contain expected parameters
    assert_output --partial "percent"
    assert_output --partial "correlation"
    
    echo "Packet loss syntax validation passed"
}

@test "Should validate rate limiting command syntax" {
    # Given a running container to target
    create_test_container "rate_limit_target" "alpine" "sleep infinity"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} rate_limit_target
    assert_output "running"
    
    # When checking pumba command syntax with --help
    echo "Checking rate limiting command syntax..."
    run pumba netem rate --help
    
    # Then command help should display successfully
    echo "Pumba help exit status: $status"
    assert_success
    
    # And help output should contain expected parameters
    assert_output --partial "rate"
    [[ $output =~ "packetoverhead" ]] || [[ $output =~ "packet" ]]
    
    echo "Rate limiting syntax validation passed"
}

@test "Should apply netem delay with egress port filter" {
    create_test_container "pingtest" "alpine" "sleep infinity"
    assert_container_running "pingtest"
    ensure_nettools_image

    pumba netem --duration 10s --tc-image ${NETTOOLS_IMAGE} --pull-image=false --egress-port 80 delay --time 100 pingtest &
    PUMBA_PID=$!

    wait_for 5 "nsenter -t \$(docker inspect -f '{{.State.Pid}}' pingtest) -n tc qdisc show dev eth0 2>/dev/null | grep -qi netem" "netem to be applied"

    # Port filters use tc filter rules with prio qdisc
    local pid
    pid=$(docker inspect -f '{{.State.Pid}}' pingtest)
    run nsenter -t "$pid" -n tc qdisc show dev eth0
    assert_output --partial "netem"

    wait $PUMBA_PID
    local pumba_exit=$?
    [ $pumba_exit -eq 0 ]

    assert_netem_cleaned "pingtest"
    assert_sidecar_cleaned
}

@test "Should apply netem delay with ingress port filter" {
    create_test_container "pingtest" "alpine" "sleep infinity"
    assert_container_running "pingtest"
    ensure_nettools_image

    pumba netem --duration 10s --tc-image ${NETTOOLS_IMAGE} --pull-image=false --ingress-port 443 delay --time 100 pingtest &
    PUMBA_PID=$!

    wait_for 5 "nsenter -t \$(docker inspect -f '{{.State.Pid}}' pingtest) -n tc qdisc show dev eth0 2>/dev/null | grep -qi netem" "netem to be applied"

    local pid
    pid=$(docker inspect -f '{{.State.Pid}}' pingtest)
    run nsenter -t "$pid" -n tc qdisc show dev eth0
    assert_output --partial "netem"

    wait $PUMBA_PID
    local pumba_exit=$?
    [ $pumba_exit -eq 0 ]

    assert_netem_cleaned "pingtest"
    assert_sidecar_cleaned
}

@test "Should apply netem delay with combined port filters" {
    create_test_container "pingtest" "alpine" "sleep infinity"
    assert_container_running "pingtest"
    ensure_nettools_image

    pumba netem --duration 10s --tc-image ${NETTOOLS_IMAGE} --pull-image=false --egress-port 80 --ingress-port 443 delay --time 100 pingtest &
    PUMBA_PID=$!

    wait_for 5 "nsenter -t \$(docker inspect -f '{{.State.Pid}}' pingtest) -n tc qdisc show dev eth0 2>/dev/null | grep -qi netem" "netem to be applied"

    local pid
    pid=$(docker inspect -f '{{.State.Pid}}' pingtest)
    run nsenter -t "$pid" -n tc qdisc show dev eth0
    assert_output --partial "netem"

    wait $PUMBA_PID
    local pumba_exit=$?
    [ $pumba_exit -eq 0 ]

    assert_netem_cleaned "pingtest"
    assert_sidecar_cleaned
}

@test "Should clean up sidecar containers after netem completes" {
    create_test_container "pingtest" "alpine" "sleep infinity"
    assert_container_running "pingtest"
    ensure_nettools_image

    run pumba netem --duration 3s --tc-image ${NETTOOLS_IMAGE} --pull-image=false delay --time 100 pingtest
    assert_success

    assert_sidecar_cleaned
}
