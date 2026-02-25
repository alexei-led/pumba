#!/usr/bin/env bats

load test_helper

setup() {
    # Clean up any leftovers
    sudo ctr -n moby t kill -s SIGKILL test-netem-ctr >/dev/null 2>&1 || true
    sudo ctr -n moby c rm test-netem-ctr >/dev/null 2>&1 || true
    # Pull images: netshoot for the target container, nettools for pumba's sidecar
    sudo ctr -n moby i pull docker.io/nicolaka/netshoot:latest >/dev/null 2>&1
    sudo ctr -n moby i pull ghcr.io/alexei-led/pumba-alpine-nettools:latest >/dev/null 2>&1
    # Create container with a dummy interface for testing
    sudo ctr -n moby run -d --privileged docker.io/nicolaka/netshoot:latest test-netem-ctr \
        sh -c "ip link add dummy0 type dummy && ip link set dummy0 up && sleep infinity" >/dev/null 2>&1
    # Give container a moment to start and create the interface
    sleep 1
}

teardown() {
    # Kill any backgrounded pumba (may be running as sudo)
    sudo pkill -f "pumba.*netem.*test-netem-ctr" 2>/dev/null || true
    kill %1 2>/dev/null || true
    sudo ctr -n moby t kill -s SIGKILL test-netem-ctr >/dev/null 2>&1 || true
    sudo ctr -n moby c rm test-netem-ctr >/dev/null 2>&1 || true
    # Clean up any leftover sidecar containers
    for sc in $(sudo ctr -n moby c ls -q 2>/dev/null | grep pumba-sidecar); do
        sudo ctr -n moby t kill -s SIGKILL $sc >/dev/null 2>&1 || true
        sudo ctr -n moby c rm $sc >/dev/null 2>&1 || true
    done
}

@test "Should apply netem delay via containerd runtime" {
    # Verify dummy0 exists and has no netem rules
    run sudo ctr -n moby t exec --exec-id check-iface test-netem-ctr ip link show dummy0
    [ $status -eq 0 ]

    # Run pumba in BACKGROUND with long duration so we can inspect tc rules while active
    sudo pumba --runtime containerd --containerd-namespace moby --log-level debug netem --interface dummy0 --duration 30s delay --time 100 test-netem-ctr &
    PUMBA_PID=$!
    # Give pumba time to apply tc rules
    sleep 2

    # Check tc qdisc - should show netem delay
    run sudo ctr -n moby t exec --exec-id check-tc test-netem-ctr tc qdisc show dev dummy0
    echo "TC output: $output"

    # Kill pumba (it would run for 30s otherwise)
    sudo kill $PUMBA_PID 2>/dev/null || kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    # Assert netem was applied
    [[ "$output" =~ "netem" ]]
    [[ "$output" =~ "delay" ]]
}

@test "Should apply packet loss via containerd runtime" {
    # Run pumba in BACKGROUND with long duration
    sudo pumba --runtime containerd --containerd-namespace moby --log-level debug netem --interface dummy0 --duration 30s loss --percent 50 test-netem-ctr &
    PUMBA_PID=$!
    sleep 2

    # Check tc qdisc - should show netem loss
    run sudo ctr -n moby t exec --exec-id check-tc-loss test-netem-ctr tc qdisc show dev dummy0
    echo "TC output: $output"

    # Kill pumba
    sudo kill $PUMBA_PID 2>/dev/null || kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    # Assert netem was applied
    [[ "$output" =~ "netem" ]]
    [[ "$output" =~ "loss" ]]
}

@test "Should apply packet duplicate via containerd runtime" {
    sudo pumba --runtime containerd --containerd-namespace moby --log-level debug netem --interface dummy0 --duration 30s duplicate --percent 50 test-netem-ctr &
    PUMBA_PID=$!
    sleep 2

    run sudo ctr -n moby t exec --exec-id check-tc-dup test-netem-ctr tc qdisc show dev dummy0
    echo "TC output: $output"

    sudo kill $PUMBA_PID 2>/dev/null || kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    [[ "$output" =~ "netem" ]]
    [[ "$output" =~ "duplicate" ]]
}

@test "Should apply packet corruption via containerd runtime" {
    sudo pumba --runtime containerd --containerd-namespace moby --log-level debug netem --interface dummy0 --duration 30s corrupt --percent 50 test-netem-ctr &
    PUMBA_PID=$!
    sleep 2

    run sudo ctr -n moby t exec --exec-id check-tc-cor test-netem-ctr tc qdisc show dev dummy0
    echo "TC output: $output"

    sudo kill $PUMBA_PID 2>/dev/null || kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    [[ "$output" =~ "netem" ]]
    [[ "$output" =~ "corrupt" ]]
}

@test "Should handle netem on non-existent container via containerd runtime" {
    # Pumba should handle gracefully — exit 0 (no matching containers found)
    run sudo pumba --runtime containerd --containerd-namespace moby --log-level debug netem --interface dummy0 --duration 2s delay --time 100 nonexistent_container_12345
    [ $status -eq 0 ]
}

@test "Should apply loss-state model via containerd runtime" {
    sudo pumba --runtime containerd --containerd-namespace moby --log-level debug netem --interface dummy0 --duration 30s loss-state --p13 5 --p31 15 --p32 10 --p23 20 --p14 5 test-netem-ctr &
    PUMBA_PID=$!
    sleep 2

    run sudo ctr -n moby t exec --exec-id check-tc-ls test-netem-ctr tc qdisc show dev dummy0
    echo "TC output: $output"

    sudo kill $PUMBA_PID 2>/dev/null || kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    [[ "$output" =~ "netem" ]]
}

@test "Should apply loss-gemodel model via containerd runtime" {
    sudo pumba --runtime containerd --containerd-namespace moby --log-level debug netem --interface dummy0 --duration 30s loss-gemodel --pg 5 --pb 20 --one-h 80 --one-k 10 test-netem-ctr &
    PUMBA_PID=$!
    sleep 2

    run sudo ctr -n moby t exec --exec-id check-tc-ge test-netem-ctr tc qdisc show dev dummy0
    echo "TC output: $output"

    sudo kill $PUMBA_PID 2>/dev/null || kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    [[ "$output" =~ "netem" ]]
}

@test "Should apply delay with normal distribution via containerd runtime" {
    sudo pumba --runtime containerd --containerd-namespace moby --log-level debug netem --interface dummy0 --duration 30s delay --time 100 --jitter 30 --distribution normal test-netem-ctr &
    PUMBA_PID=$!
    sleep 2

    run sudo ctr -n moby t exec --exec-id check-tc-dist test-netem-ctr tc qdisc show dev dummy0
    echo "TC output: $output"

    sudo kill $PUMBA_PID 2>/dev/null || kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    [[ "$output" =~ "netem" ]]
    [[ "$output" =~ "delay" ]]
}

@test "Should apply rate limit with cell options via containerd runtime" {
    sudo pumba --runtime containerd --containerd-namespace moby --log-level debug netem --interface dummy0 --duration 30s rate --rate 1mbit --packetoverhead 10 --cellsize 1500 --celloverhead 20 test-netem-ctr &
    PUMBA_PID=$!
    sleep 2

    run sudo ctr -n moby t exec --exec-id check-tc-cell test-netem-ctr tc qdisc show dev dummy0
    echo "TC output: $output"

    sudo kill $PUMBA_PID 2>/dev/null || kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    [[ "$output" =~ "netem" ]]
    [[ "$output" =~ "rate" ]]
}

@test "Should apply rate limiting via containerd runtime" {
    sudo pumba --runtime containerd --containerd-namespace moby --log-level debug netem --interface dummy0 --duration 30s rate --rate 100kbit test-netem-ctr &
    PUMBA_PID=$!
    sleep 2

    run sudo ctr -n moby t exec --exec-id check-tc-rate test-netem-ctr tc qdisc show dev dummy0
    echo "TC output: $output"

    sudo kill $PUMBA_PID 2>/dev/null || kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    [[ "$output" =~ "netem" ]]
    [[ "$output" =~ "rate" ]]
}
