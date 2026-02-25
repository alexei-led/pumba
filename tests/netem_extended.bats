#!/usr/bin/env bats

# Load the test helper
load test_helper

setup() {
    # Clean any leftover containers from previous test runs
    cleanup_containers "netem_ext_target"
    
    # Also cleanup any nettools containers that might be left running
    docker ps -q --filter "ancestor=ghcr.io/alexei-led/pumba-alpine-nettools" | xargs -r docker rm -f
}

teardown() {
    # Kill any background pumba processes targeting our test containers
    sudo pkill -f "pumba.*netem_ext_target" 2>/dev/null || true

    # Clean up containers after each test
    cleanup_containers "netem_ext_target"
    
    # Also cleanup any nettools containers that might be left running
    docker ps -q --filter "ancestor=ghcr.io/alexei-led/pumba-alpine-nettools" | xargs -r docker rm -f
}

@test "Should verify netem duplicate help" {
    run pumba netem duplicate --help
    assert_success
    # Verify help contains expected options
    assert_output --partial "duplicate"
    assert_output --partial "percent"
    assert_output --partial "correlation"
}

@test "Should verify netem corrupt help" {
    run pumba netem corrupt --help
    assert_success
    # Verify help contains expected options 
    assert_output --partial "corrupt"
    assert_output --partial "percent"
    assert_output --partial "correlation"
}

@test "Should verify netem loss-state help" {
    run pumba netem loss-state --help
    assert_success
    # Verify help contains expected options
    assert_output --partial "loss-state"
    assert_output --partial "p13"
    assert_output --partial "p31"
}

@test "Should verify netem loss-gemodel help" {
    run pumba netem loss-gemodel --help
    assert_success
    # Verify help contains expected options
    assert_output --partial "loss-gemodel"
    assert_output --partial "pg"
    assert_output --partial "pb"
}

@test "Should apply packet duplicate with netem" {
    # Given a running container
    create_test_container "netem_ext_target" "alpine" "sleep infinity"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} netem_ext_target
    assert_output "running"
    
    # Ensure nettools image is available
    ensure_nettools_image

    # Run pumba in background with longer duration so we can inspect while active
    pumba netem --duration 10s --tc-image ${NETTOOLS_IMAGE} --pull-image=false duplicate --percent 10 netem_ext_target &
    PUMBA_PID=$!

    # Wait for netem to be applied
    wait_for 5 "nsenter -t \$(docker inspect -f '{{.State.Pid}}' netem_ext_target) -n tc qdisc show dev eth0 2>/dev/null | grep -qi netem" "netem to be applied"

    # Verify kernel-level tc rules
    assert_netem_applied "netem_ext_target" "duplicate"

    # Wait for pumba to finish and clean up
    wait $PUMBA_PID
    local pumba_exit=$?
    [ $pumba_exit -eq 0 ]

    # Verify cleanup
    assert_netem_cleaned "netem_ext_target"
    assert_sidecar_cleaned
}

@test "Should apply packet corruption with netem" {
    # Given a running container
    create_test_container "netem_ext_target" "alpine" "sleep infinity"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} netem_ext_target
    assert_output "running"
    
    # Ensure nettools image is available
    ensure_nettools_image

    # Run pumba in background with longer duration so we can inspect while active
    pumba netem --duration 10s --tc-image ${NETTOOLS_IMAGE} --pull-image=false corrupt --percent 5 netem_ext_target &
    PUMBA_PID=$!

    # Wait for netem to be applied
    wait_for 5 "nsenter -t \$(docker inspect -f '{{.State.Pid}}' netem_ext_target) -n tc qdisc show dev eth0 2>/dev/null | grep -qi netem" "netem to be applied"

    # Verify kernel-level tc rules
    assert_netem_applied "netem_ext_target" "corrupt"

    # Wait for pumba to finish and clean up
    wait $PUMBA_PID
    local pumba_exit=$?
    [ $pumba_exit -eq 0 ]

    # Verify cleanup
    assert_netem_cleaned "netem_ext_target"
    assert_sidecar_cleaned
}

@test "Should apply packet loss with correlation" {
    # Given a running container
    create_test_container "netem_ext_target" "alpine" "sleep infinity"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} netem_ext_target
    assert_output "running"
    
    # Ensure nettools image is available
    ensure_nettools_image

    # Run pumba in background with longer duration so we can inspect while active
    pumba netem --duration 10s --tc-image ${NETTOOLS_IMAGE} --pull-image=false loss --percent 20 --correlation 75 netem_ext_target &
    PUMBA_PID=$!

    # Wait for netem to be applied
    wait_for 5 "nsenter -t \$(docker inspect -f '{{.State.Pid}}' netem_ext_target) -n tc qdisc show dev eth0 2>/dev/null | grep -qi netem" "netem to be applied"

    # Verify kernel-level tc rules
    assert_netem_applied "netem_ext_target" "loss"

    # Wait for pumba to finish and clean up
    wait $PUMBA_PID
    local pumba_exit=$?
    [ $pumba_exit -eq 0 ]

    # Verify cleanup
    assert_netem_cleaned "netem_ext_target"
    assert_sidecar_cleaned
}

@test "Should apply advanced loss models (loss-state)" {
    # Given a running container
    create_test_container "netem_ext_target" "alpine" "sleep infinity"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} netem_ext_target
    assert_output "running"
    
    # Ensure nettools image is available
    ensure_nettools_image

    # Run pumba in background with longer duration so we can inspect while active
    pumba netem --duration 10s --tc-image ${NETTOOLS_IMAGE} --pull-image=false loss-state --p13 5 --p31 15 --p32 10 --p23 20 --p14 5 netem_ext_target &
    PUMBA_PID=$!

    # Wait for netem to be applied
    wait_for 5 "nsenter -t \$(docker inspect -f '{{.State.Pid}}' netem_ext_target) -n tc qdisc show dev eth0 2>/dev/null | grep -qi netem" "netem to be applied"

    # Verify kernel-level tc rules
    assert_netem_applied "netem_ext_target" "netem"

    # Wait for pumba to finish and clean up
    wait $PUMBA_PID
    local pumba_exit=$?
    [ $pumba_exit -eq 0 ]

    # Verify cleanup
    assert_netem_cleaned "netem_ext_target"
    assert_sidecar_cleaned
}

@test "Should apply loss-gemodel model" {
    # Given a running container
    create_test_container "netem_ext_target" "alpine" "sleep infinity"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} netem_ext_target
    assert_output "running"
    
    # Ensure nettools image is available
    ensure_nettools_image

    # Run pumba in background with longer duration so we can inspect while active
    pumba netem --duration 10s --tc-image ${NETTOOLS_IMAGE} --pull-image=false loss-gemodel --pg 5 --pb 20 --one-h 80 --one-k 10 netem_ext_target &
    PUMBA_PID=$!

    # Wait for netem to be applied
    wait_for 5 "nsenter -t \$(docker inspect -f '{{.State.Pid}}' netem_ext_target) -n tc qdisc show dev eth0 2>/dev/null | grep -qi netem" "netem to be applied"

    # Verify kernel-level tc rules
    assert_netem_applied "netem_ext_target" "netem"

    # Wait for pumba to finish and clean up
    wait $PUMBA_PID
    local pumba_exit=$?
    [ $pumba_exit -eq 0 ]

    # Verify cleanup
    assert_netem_cleaned "netem_ext_target"
    assert_sidecar_cleaned
}

@test "Should apply delay with distribution options" {
    # Given a running container
    create_test_container "netem_ext_target" "alpine" "sleep infinity"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} netem_ext_target
    assert_output "running"
    
    # Ensure nettools image is available
    ensure_nettools_image

    # Test each distribution: normal, pareto, paretonormal
    for dist in normal pareto paretonormal; do
        echo "Applying delay with $dist distribution..."

        pumba netem --duration 10s --tc-image ${NETTOOLS_IMAGE} --pull-image=false delay --time 100 --jitter 30 --distribution "$dist" netem_ext_target &
        PUMBA_PID=$!

        # Wait for netem to be applied
        wait_for 5 "nsenter -t \$(docker inspect -f '{{.State.Pid}}' netem_ext_target) -n tc qdisc show dev eth0 2>/dev/null | grep -qi netem" "netem to be applied ($dist)"

        # Verify kernel-level tc rules
        assert_netem_applied "netem_ext_target" "delay"

        # Wait for pumba to finish and clean up
        wait $PUMBA_PID
        local pumba_exit=$?
        [ $pumba_exit -eq 0 ]

        # Verify cleanup
        assert_netem_cleaned "netem_ext_target"
        assert_sidecar_cleaned

        echo "Delay with $dist distribution verified successfully"
    done
}

@test "Should apply rate limit with cell options" {
    # Given a running container
    create_test_container "netem_ext_target" "alpine" "sleep infinity"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} netem_ext_target
    assert_output "running"
    
    # Ensure nettools image is available
    ensure_nettools_image

    # Run pumba in background with longer duration so we can inspect while active
    pumba netem --duration 10s --tc-image ${NETTOOLS_IMAGE} --pull-image=false rate --rate 1mbit --packetoverhead 10 --cellsize 1500 --celloverhead 20 netem_ext_target &
    PUMBA_PID=$!

    # Wait for netem to be applied
    wait_for 5 "nsenter -t \$(docker inspect -f '{{.State.Pid}}' netem_ext_target) -n tc qdisc show dev eth0 2>/dev/null | grep -qi netem" "netem to be applied"

    # Verify kernel-level tc rules
    assert_netem_applied "netem_ext_target" "rate"

    # Wait for pumba to finish and clean up
    wait $PUMBA_PID
    local pumba_exit=$?
    [ $pumba_exit -eq 0 ]

    # Verify cleanup
    assert_netem_cleaned "netem_ext_target"
    assert_sidecar_cleaned
}

@test "Should support custom interface parameter" {
    # Given a running container
    create_test_container "netem_ext_target" "alpine" "sleep infinity"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} netem_ext_target
    assert_output "running"
    
    # Ensure nettools image is available
    ensure_nettools_image

    # Run pumba in background with longer duration so we can inspect while active
    pumba netem --duration 10s --tc-image ${NETTOOLS_IMAGE} --pull-image=false --interface eth0 delay --time 100 netem_ext_target &
    PUMBA_PID=$!

    # Wait for netem to be applied
    wait_for 5 "nsenter -t \$(docker inspect -f '{{.State.Pid}}' netem_ext_target) -n tc qdisc show dev eth0 2>/dev/null | grep -qi netem" "netem to be applied"

    # Verify kernel-level tc rules with explicit interface
    assert_netem_applied "netem_ext_target" "delay" "eth0"

    # Wait for pumba to finish and clean up
    wait $PUMBA_PID
    local pumba_exit=$?
    [ $pumba_exit -eq 0 ]

    # Verify cleanup
    assert_netem_cleaned "netem_ext_target"
    assert_sidecar_cleaned
}
