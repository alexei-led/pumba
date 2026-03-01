#!/usr/bin/env bats

load test_helper

setup() {
    require_containerd
    # Clean up any leftovers
    sudo ctr -n moby t kill -s SIGKILL test-netem-ctr >/dev/null 2>&1 || true
    sudo ctr -n moby c rm test-netem-ctr >/dev/null 2>&1 || true
    # Pull images: netshoot for the target container, nettools for pumba's sidecar
    ctr_pull_image moby docker.io/nicolaka/netshoot:latest
    ctr_pull_image moby ghcr.io/alexei-led/pumba-alpine-nettools:latest
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
    assert_success

    # Run pumba in BACKGROUND with long duration so we can inspect tc rules while active
    sudo pumba --runtime containerd --containerd-namespace moby --log-level debug netem --interface dummy0 --pull-image=false --duration 30s delay --time 100 test-netem-ctr &
    PUMBA_PID=$!
    # Give pumba time to apply tc rules
    sleep 2

    # Check tc qdisc - should show netem delay
    run sudo ctr -n moby t exec --exec-id check-tc test-netem-ctr tc qdisc show dev dummy0
    echo "TC output: $output"

    # Assert netem was applied with exact values
    assert_output --partial "netem"
    assert_output --partial "delay 100"

    # Kill pumba (it would run for 30s otherwise)
    sudo kill $PUMBA_PID 2>/dev/null || kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    # Verify cleanup
    sleep 1
    run sudo ctr -n moby t exec --exec-id check-tc-clean-delay test-netem-ctr tc qdisc show dev dummy0
    echo "TC after cleanup: $output"
    refute_output --partial "netem"
}

@test "Should apply packet loss via containerd runtime" {
    # Run pumba in BACKGROUND with long duration
    sudo pumba --runtime containerd --containerd-namespace moby --log-level debug netem --interface dummy0 --pull-image=false --duration 30s loss --percent 50 test-netem-ctr &
    PUMBA_PID=$!
    sleep 2

    # Check tc qdisc - should show netem loss
    run sudo ctr -n moby t exec --exec-id check-tc-loss test-netem-ctr tc qdisc show dev dummy0
    echo "TC output: $output"

    # Assert netem was applied with exact values
    assert_output --partial "netem"
    assert_output --partial "loss 50%"

    # Kill pumba
    sudo kill $PUMBA_PID 2>/dev/null || kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    # Verify cleanup
    sleep 1
    run sudo ctr -n moby t exec --exec-id check-tc-clean-loss test-netem-ctr tc qdisc show dev dummy0
    echo "TC after cleanup: $output"
    refute_output --partial "netem"
}

@test "Should apply packet duplicate via containerd runtime" {
    sudo pumba --runtime containerd --containerd-namespace moby --log-level debug netem --interface dummy0 --pull-image=false --duration 30s duplicate --percent 50 test-netem-ctr &
    PUMBA_PID=$!
    sleep 2

    run sudo ctr -n moby t exec --exec-id check-tc-dup test-netem-ctr tc qdisc show dev dummy0
    echo "TC output: $output"

    # Assert netem was applied with exact values
    assert_output --partial "netem"
    assert_output --partial "duplicate 50%"

    sudo kill $PUMBA_PID 2>/dev/null || kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    # Verify cleanup
    sleep 1
    run sudo ctr -n moby t exec --exec-id check-tc-clean-dup test-netem-ctr tc qdisc show dev dummy0
    echo "TC after cleanup: $output"
    refute_output --partial "netem"
}

@test "Should apply packet corruption via containerd runtime" {
    sudo pumba --runtime containerd --containerd-namespace moby --log-level debug netem --interface dummy0 --pull-image=false --duration 30s corrupt --percent 50 test-netem-ctr &
    PUMBA_PID=$!
    sleep 2

    run sudo ctr -n moby t exec --exec-id check-tc-cor test-netem-ctr tc qdisc show dev dummy0
    echo "TC output: $output"

    # Assert netem was applied with exact values
    assert_output --partial "netem"
    assert_output --partial "corrupt 50%"

    sudo kill $PUMBA_PID 2>/dev/null || kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    # Verify cleanup
    sleep 1
    run sudo ctr -n moby t exec --exec-id check-tc-clean-cor test-netem-ctr tc qdisc show dev dummy0
    echo "TC after cleanup: $output"
    refute_output --partial "netem"
}

@test "Should handle netem on non-existent container via containerd runtime" {
    # Pumba should handle gracefully — exit 0 (no matching containers found)
    run sudo pumba --runtime containerd --containerd-namespace moby --log-level debug netem --interface dummy0 --pull-image=false --duration 2s delay --time 100 nonexistent_container_12345
    assert_success
}

@test "Should apply loss-state model via containerd runtime" {
    sudo pumba --runtime containerd --containerd-namespace moby --log-level debug netem --interface dummy0 --pull-image=false --duration 30s loss-state --p13 5 --p31 15 --p32 10 --p23 20 --p14 5 test-netem-ctr &
    PUMBA_PID=$!
    sleep 2

    run sudo ctr -n moby t exec --exec-id check-tc-ls test-netem-ctr tc qdisc show dev dummy0
    echo "TC output: $output"

    # Assert netem was applied
    assert_output --partial "netem"

    sudo kill $PUMBA_PID 2>/dev/null || kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    # Verify cleanup
    sleep 1
    run sudo ctr -n moby t exec --exec-id check-tc-clean-ls test-netem-ctr tc qdisc show dev dummy0
    echo "TC after cleanup: $output"
    refute_output --partial "netem"
}

@test "Should apply loss-gemodel model via containerd runtime" {
    sudo pumba --runtime containerd --containerd-namespace moby --log-level debug netem --interface dummy0 --pull-image=false --duration 30s loss-gemodel --pg 5 --pb 20 --one-h 80 --one-k 10 test-netem-ctr &
    PUMBA_PID=$!
    sleep 2

    run sudo ctr -n moby t exec --exec-id check-tc-ge test-netem-ctr tc qdisc show dev dummy0
    echo "TC output: $output"

    # Assert netem was applied
    assert_output --partial "netem"

    sudo kill $PUMBA_PID 2>/dev/null || kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    # Verify cleanup
    sleep 1
    run sudo ctr -n moby t exec --exec-id check-tc-clean-ge test-netem-ctr tc qdisc show dev dummy0
    echo "TC after cleanup: $output"
    refute_output --partial "netem"
}

@test "Should apply delay with normal distribution via containerd runtime" {
    sudo pumba --runtime containerd --containerd-namespace moby --log-level debug netem --interface dummy0 --pull-image=false --duration 30s delay --time 100 --jitter 30 --distribution normal test-netem-ctr &
    PUMBA_PID=$!
    sleep 2

    run sudo ctr -n moby t exec --exec-id check-tc-dist test-netem-ctr tc qdisc show dev dummy0
    echo "TC output: $output"

    # Assert netem was applied with exact values
    assert_output --partial "netem"
    assert_output --partial "delay 100"

    sudo kill $PUMBA_PID 2>/dev/null || kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    # Verify cleanup
    sleep 1
    run sudo ctr -n moby t exec --exec-id check-tc-clean-dist test-netem-ctr tc qdisc show dev dummy0
    echo "TC after cleanup: $output"
    refute_output --partial "netem"
}

@test "Should apply rate limit with cell options via containerd runtime" {
    sudo pumba --runtime containerd --containerd-namespace moby --log-level debug netem --interface dummy0 --pull-image=false --duration 30s rate --rate 1mbit --packetoverhead 10 --cellsize 1500 --celloverhead 20 test-netem-ctr &
    PUMBA_PID=$!
    sleep 2

    run sudo ctr -n moby t exec --exec-id check-tc-cell test-netem-ctr tc qdisc show dev dummy0
    echo "TC output: $output"

    # Assert netem was applied with exact values
    assert_output --partial "netem"
    assert_output --partial "rate 1Mbit"

    sudo kill $PUMBA_PID 2>/dev/null || kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    # Verify cleanup
    sleep 1
    run sudo ctr -n moby t exec --exec-id check-tc-clean-cell test-netem-ctr tc qdisc show dev dummy0
    echo "TC after cleanup: $output"
    refute_output --partial "netem"
}

@test "Should apply rate limiting via containerd runtime" {
    sudo pumba --runtime containerd --containerd-namespace moby --log-level debug netem --interface dummy0 --pull-image=false --duration 30s rate --rate 100kbit test-netem-ctr &
    PUMBA_PID=$!
    sleep 2

    run sudo ctr -n moby t exec --exec-id check-tc-rate test-netem-ctr tc qdisc show dev dummy0
    echo "TC output: $output"

    # Assert netem was applied with exact values
    assert_output --partial "netem"
    assert_output --partial "rate 100Kbit"

    sudo kill $PUMBA_PID 2>/dev/null || kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    # Verify cleanup
    sleep 1
    run sudo ctr -n moby t exec --exec-id check-tc-clean-rate test-netem-ctr tc qdisc show dev dummy0
    echo "TC after cleanup: $output"
    refute_output --partial "netem"
}

@test "Should apply netem delay with egress port filter via containerd runtime" {
    sudo pumba --runtime containerd --containerd-namespace moby --log-level debug netem --interface dummy0 --pull-image=false --egress-port 80 --duration 30s delay --time 100 test-netem-ctr &
    PUMBA_PID=$!
    sleep 2

    run sudo ctr -n moby t exec --exec-id check-tc-egress test-netem-ctr tc qdisc show dev dummy0
    echo "TC output: $output"

    assert_output --partial "netem"

    sudo kill $PUMBA_PID 2>/dev/null || kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    sleep 1
    run sudo ctr -n moby t exec --exec-id check-tc-clean-egress test-netem-ctr tc qdisc show dev dummy0
    refute_output --partial "netem"
}

@test "Should apply netem delay with ingress port filter via containerd runtime" {
    sudo pumba --runtime containerd --containerd-namespace moby --log-level debug netem --interface dummy0 --pull-image=false --ingress-port 443 --duration 30s delay --time 100 test-netem-ctr &
    PUMBA_PID=$!
    sleep 2

    run sudo ctr -n moby t exec --exec-id check-tc-ingress test-netem-ctr tc qdisc show dev dummy0
    echo "TC output: $output"

    assert_output --partial "netem"

    sudo kill $PUMBA_PID 2>/dev/null || kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    sleep 1
    run sudo ctr -n moby t exec --exec-id check-tc-clean-ingress test-netem-ctr tc qdisc show dev dummy0
    refute_output --partial "netem"
}
