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
    # Kill any backgrounded pumba
    kill %1 2>/dev/null || true
    sudo ctr -n moby t kill -s SIGKILL test-netem-ctr >/dev/null 2>&1 || true
    sudo ctr -n moby c rm test-netem-ctr >/dev/null 2>&1 || true
}

@test "Should apply netem delay via containerd runtime" {
    # Verify dummy0 exists and has no netem rules
    run sudo ctr -n moby t exec --exec-id check-iface test-netem-ctr ip link show dummy0
    [ $status -eq 0 ]

    # Run pumba in BACKGROUND with long duration so we can inspect tc rules while active
    pumba --runtime containerd --containerd-namespace moby --log-level debug netem --interface dummy0 --duration 30s delay --time 100 test-netem-ctr &
    PUMBA_PID=$!
    # Give pumba time to apply tc rules
    sleep 2

    # Check tc qdisc - should show netem delay
    run sudo ctr -n moby t exec --exec-id check-tc test-netem-ctr tc qdisc show dev dummy0
    echo "TC output: $output"

    # Kill pumba (it would run for 30s otherwise)
    kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    # Assert netem was applied
    [[ "$output" =~ "netem" ]]
    [[ "$output" =~ "delay" ]]
}

@test "Should apply packet loss via containerd runtime" {
    # Run pumba in BACKGROUND with long duration
    pumba --runtime containerd --containerd-namespace moby --log-level debug netem --interface dummy0 --duration 30s loss --percent 50 test-netem-ctr &
    PUMBA_PID=$!
    sleep 2

    # Check tc qdisc - should show netem loss
    run sudo ctr -n moby t exec --exec-id check-tc-loss test-netem-ctr tc qdisc show dev dummy0
    echo "TC output: $output"

    # Kill pumba
    kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    # Assert netem was applied
    [[ "$output" =~ "netem" ]]
    [[ "$output" =~ "loss" ]]
}
