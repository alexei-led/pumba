#!/usr/bin/env bats

load test_helper

setup() {
    sudo ctr -n moby t kill -s SIGKILL test-ipt-ctr >/dev/null 2>&1 || true
    sudo ctr -n moby c rm test-ipt-ctr >/dev/null 2>&1 || true
    # Need iptables in the container image for direct exec mode
    sudo ctr -n moby i pull docker.io/nicolaka/netshoot:latest >/dev/null 2>&1
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

    # Kill pumba
    sudo kill $PUMBA_PID 2>/dev/null || kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    # There should be some DROP rule
    [[ "$output" =~ "DROP" ]] || [[ "$output" =~ "drop" ]] || [[ "$output" =~ "statistic" ]]
}

@test "Should apply iptables loss with nth mode via containerd runtime" {
    sudo pumba --runtime containerd --containerd-namespace moby --log-level debug iptables --interface lo --duration 30s loss --mode nth --every 3 test-ipt-ctr &
    PUMBA_PID=$!
    sleep 2

    run sudo ctr -n moby t exec --exec-id check-ipt-nth test-ipt-ctr iptables -L INPUT -n -v
    echo "iptables nth output: $output"

    sudo kill $PUMBA_PID 2>/dev/null || kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    [[ "$output" =~ "DROP" ]] || [[ "$output" =~ "statistic" ]]
}

@test "Should handle iptables on non-existent container via containerd runtime" {
    # Pumba should handle gracefully — exit 0 (no matching containers found)
    run sudo pumba --runtime containerd --containerd-namespace moby --log-level debug iptables --interface lo --duration 2s loss --probability 1.0 nonexistent_container_12345
    [ $status -eq 0 ]
}

@test "Should apply iptables loss with destination IP filter via containerd runtime" {
    sudo pumba --runtime containerd --containerd-namespace moby --log-level debug iptables --interface lo --destination 10.0.0.0/8 --duration 30s loss --probability 1.0 test-ipt-ctr &
    PUMBA_PID=$!
    sleep 2

    run sudo ctr -n moby t exec --exec-id check-ipt-dst test-ipt-ctr iptables -L INPUT -n -v
    echo "iptables destination filter output: $output"

    sudo kill $PUMBA_PID 2>/dev/null || kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    [[ "$output" =~ "DROP" ]] || [[ "$output" =~ "10.0.0.0" ]]
}

@test "Should apply iptables loss with port filters via containerd runtime" {
    sudo pumba --runtime containerd --containerd-namespace moby --log-level debug iptables --interface lo --protocol tcp --dst-port 80 --duration 30s loss --probability 1.0 test-ipt-ctr &
    PUMBA_PID=$!
    sleep 2

    run sudo ctr -n moby t exec --exec-id check-ipt-port test-ipt-ctr iptables -L INPUT -n -v
    echo "iptables port filter output: $output"

    sudo kill $PUMBA_PID 2>/dev/null || kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    [[ "$output" =~ "DROP" ]] || [[ "$output" =~ "tcp" ]] || [[ "$output" =~ "dpt:80" ]]
}

@test "Should apply iptables loss with source IP filter via containerd runtime" {
    sudo pumba --runtime containerd --containerd-namespace moby --log-level debug iptables --interface lo --source 10.0.0.0/8 --duration 30s loss --probability 1.0 test-ipt-ctr &
    PUMBA_PID=$!
    sleep 2

    run sudo ctr -n moby t exec --exec-id check-ipt-src test-ipt-ctr iptables -L INPUT -n -v
    echo "iptables source filter output: $output"

    sudo kill $PUMBA_PID 2>/dev/null || kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    [[ "$output" =~ "DROP" ]] || [[ "$output" =~ "10.0.0.0" ]]
}
