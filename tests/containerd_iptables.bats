#!/usr/bin/env bats

load test_helper

setup() {
    require_containerd
    sudo ctr -n moby t kill -s SIGKILL test-ipt-ctr >/dev/null 2>&1 || true
    sudo ctr -n moby c rm test-ipt-ctr >/dev/null 2>&1 || true
    # Need iptables in the container image for direct exec mode
    ctr_pull_image moby docker.io/nicolaka/netshoot:latest
    sudo ctr -n moby run -d --privileged docker.io/nicolaka/netshoot:latest test-ipt-ctr sleep infinity >/dev/null 2>&1
    sleep 1
}

teardown() {
    # Kill any backgrounded pumba (may be running as sudo)
    sudo pkill -f "pumba.*iptables.*test-ipt-ctr" 2>/dev/null || true
    kill %1 2>/dev/null || true
    sudo ctr -n moby t kill -s SIGKILL test-ipt-ctr >/dev/null 2>&1 || true
    sudo ctr -n moby c rm test-ipt-ctr >/dev/null 2>&1 || true
}

@test "Should apply iptables packet loss via containerd runtime" {
    # Run pumba in background with long duration
    sudo pumba --runtime containerd --containerd-namespace moby --log-level debug iptables --interface lo --duration 30s loss --probability 1.0 test-ipt-ctr &
    PUMBA_PID=$!
    sleep 2

    # Check iptables rules inside the container
    run sudo ctr -n moby t exec --exec-id check-ipt test-ipt-ctr iptables -L INPUT -n -v
    echo "iptables output: $output"

    # Verify both DROP rule and statistic module are present
    assert_output --partial "DROP"
    assert_output --partial "statistic"

    # Kill pumba
    sudo kill $PUMBA_PID 2>/dev/null || kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    # Verify cleanup removed iptables rules
    sleep 1
    run sudo ctr -n moby t exec --exec-id check-ipt-clean test-ipt-ctr iptables -L INPUT -n -v
    echo "iptables after cleanup: $output"
    refute_output --partial "DROP"
}

@test "Should apply iptables loss with nth mode via containerd runtime" {
    sudo pumba --runtime containerd --containerd-namespace moby --log-level debug iptables --interface lo --duration 30s loss --mode nth --every 3 test-ipt-ctr &
    PUMBA_PID=$!
    sleep 2

    run sudo ctr -n moby t exec --exec-id check-ipt-nth test-ipt-ctr iptables -L INPUT -n -v
    echo "iptables nth output: $output"

    # Verify both DROP rule and statistic module are present
    assert_output --partial "DROP"
    assert_output --partial "statistic"

    sudo kill $PUMBA_PID 2>/dev/null || kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    # Verify cleanup removed iptables rules
    sleep 1
    run sudo ctr -n moby t exec --exec-id check-ipt-clean-nth test-ipt-ctr iptables -L INPUT -n -v
    echo "iptables after cleanup: $output"
    refute_output --partial "DROP"
}

@test "Should handle iptables on non-existent container via containerd runtime" {
    # Pumba should handle gracefully — exit 0 (no matching containers found)
    run sudo pumba --runtime containerd --containerd-namespace moby --log-level debug iptables --interface lo --duration 2s loss --probability 1.0 nonexistent_container_12345
    assert_success
}

@test "Should apply iptables loss with destination IP filter via containerd runtime" {
    sudo pumba --runtime containerd --containerd-namespace moby --log-level debug iptables --interface lo --destination 10.0.0.0/8 --duration 30s loss --probability 1.0 test-ipt-ctr &
    PUMBA_PID=$!
    sleep 2

    run sudo ctr -n moby t exec --exec-id check-ipt-dst test-ipt-ctr iptables -L INPUT -n -v
    echo "iptables destination filter output: $output"

    # Verify both DROP rule and destination IP filter are present
    assert_output --partial "DROP"
    assert_output --partial "10.0.0.0"

    sudo kill $PUMBA_PID 2>/dev/null || kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    # Verify cleanup removed iptables rules
    sleep 1
    run sudo ctr -n moby t exec --exec-id check-ipt-clean-dst test-ipt-ctr iptables -L INPUT -n -v
    echo "iptables after cleanup: $output"
    refute_output --partial "DROP"
}

@test "Should apply iptables loss with port filters via containerd runtime" {
    sudo pumba --runtime containerd --containerd-namespace moby --log-level debug iptables --interface lo --protocol tcp --dst-port 80 --duration 30s loss --probability 1.0 test-ipt-ctr &
    PUMBA_PID=$!
    sleep 2

    run sudo ctr -n moby t exec --exec-id check-ipt-port test-ipt-ctr iptables -L INPUT -n -v
    echo "iptables port filter output: $output"

    # Verify DROP rule, protocol, and port filter are all present
    assert_output --partial "DROP"
    assert_output --partial "tcp"
    assert_output --partial "dpt:80"

    sudo kill $PUMBA_PID 2>/dev/null || kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    # Verify cleanup removed iptables rules
    sleep 1
    run sudo ctr -n moby t exec --exec-id check-ipt-clean-port test-ipt-ctr iptables -L INPUT -n -v
    echo "iptables after cleanup: $output"
    refute_output --partial "DROP"
}

@test "Should apply iptables loss with source IP filter via containerd runtime" {
    sudo pumba --runtime containerd --containerd-namespace moby --log-level debug iptables --interface lo --source 10.0.0.0/8 --duration 30s loss --probability 1.0 test-ipt-ctr &
    PUMBA_PID=$!
    sleep 2

    run sudo ctr -n moby t exec --exec-id check-ipt-src test-ipt-ctr iptables -L INPUT -n -v
    echo "iptables source filter output: $output"

    # Verify both DROP rule and source IP filter are present
    assert_output --partial "DROP"
    assert_output --partial "10.0.0.0"

    sudo kill $PUMBA_PID 2>/dev/null || kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    # Verify cleanup removed iptables rules
    sleep 1
    run sudo ctr -n moby t exec --exec-id check-ipt-clean-src test-ipt-ctr iptables -L INPUT -n -v
    echo "iptables after cleanup: $output"
    refute_output --partial "DROP"
}
