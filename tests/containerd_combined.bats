#!/usr/bin/env bats

# Combined netem + iptables scenarios via containerd runtime.
# These tests verify that multiple chaos actions can be applied to the same container.

load test_helper

NETTOOLS_IMAGE="ghcr.io/alexei-led/pumba-alpine-nettools:latest"

setup() {
    sudo ctr -n moby t kill -s SIGKILL test-combined-ctr >/dev/null 2>&1 || true
    sudo ctr -n moby c rm test-combined-ctr >/dev/null 2>&1 || true
    sudo ctr -n moby i pull docker.io/nicolaka/netshoot:latest >/dev/null 2>&1
    sudo ctr -n moby i pull "$NETTOOLS_IMAGE" >/dev/null 2>&1
    sudo ctr -n moby run -d --privileged docker.io/nicolaka/netshoot:latest test-combined-ctr \
        sh -c "ip link add dummy0 type dummy && ip link set dummy0 up && sleep infinity" >/dev/null 2>&1
    sleep 1
}

teardown() {
    sudo pkill -f "pumba.*test-combined-ctr" 2>/dev/null || true
    kill %1 2>/dev/null || true
    sudo ctr -n moby t kill -s SIGKILL test-combined-ctr >/dev/null 2>&1 || true
    sudo ctr -n moby c rm test-combined-ctr >/dev/null 2>&1 || true
    for sc in $(sudo ctr -n moby c ls -q 2>/dev/null | grep pumba-sidecar); do
        sudo ctr -n moby t kill -s SIGKILL $sc >/dev/null 2>&1 || true
        sudo ctr -n moby c rm $sc >/dev/null 2>&1 || true
    done
}

@test "Should apply netem then iptables on same container via containerd runtime" {
    # Apply netem delay
    run sudo pumba --runtime containerd --containerd-namespace moby --log-level debug netem --interface dummy0 --duration 2s delay --time 100 test-combined-ctr
    echo "Netem status: $status, output: $output"
    [ $status -eq 0 ]

    # Apply iptables loss on the same container
    sudo pumba --runtime containerd --containerd-namespace moby --log-level debug iptables --interface lo --duration 30s loss --probability 1.0 test-combined-ctr &
    PUMBA_PID=$!
    sleep 2

    run sudo ctr -n moby t exec --exec-id check-ipt-combined test-combined-ctr iptables -L INPUT -n -v
    echo "iptables output: $output"

    sudo kill $PUMBA_PID 2>/dev/null || kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    [[ "$output" =~ "DROP" ]] || [[ "$output" =~ "statistic" ]]
}

@test "Should apply rate limit with iptables protocol filter via containerd runtime" {
    # Apply rate limit
    run sudo pumba --runtime containerd --containerd-namespace moby --log-level debug netem --interface dummy0 --duration 2s rate --rate 1mbit test-combined-ctr
    echo "Rate limit status: $status"
    [ $status -eq 0 ]

    # Apply iptables with protocol filter
    sudo pumba --runtime containerd --containerd-namespace moby --log-level debug iptables --interface lo --protocol icmp --duration 30s loss --probability 0.5 test-combined-ctr &
    PUMBA_PID=$!
    sleep 2

    run sudo ctr -n moby t exec --exec-id check-ipt-proto test-combined-ctr iptables -L INPUT -n -v
    echo "iptables protocol output: $output"

    sudo kill $PUMBA_PID 2>/dev/null || kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    [[ "$output" =~ "DROP" ]] || [[ "$output" =~ "icmp" ]]
}

@test "Should apply netem with target IP filter via containerd runtime" {
    sudo pumba --runtime containerd --containerd-namespace moby --log-level debug netem --interface dummy0 --target 8.8.8.8 --duration 30s delay --time 100 test-combined-ctr &
    PUMBA_PID=$!
    sleep 2

    # Check tc qdisc — should have prio qdisc with netem on band 3
    run sudo ctr -n moby t exec --exec-id check-tc-ipf test-combined-ctr tc qdisc show dev dummy0
    echo "TC output: $output"

    sudo kill $PUMBA_PID 2>/dev/null || kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    [[ "$output" =~ "prio" ]] || [[ "$output" =~ "netem" ]]
}

@test "Should apply iptables nth mode in combined scenario via containerd runtime" {
    # Apply netem delay first
    run sudo pumba --runtime containerd --containerd-namespace moby --log-level debug netem --interface dummy0 --duration 2s delay --time 50 test-combined-ctr
    [ $status -eq 0 ]

    # Then iptables nth mode
    sudo pumba --runtime containerd --containerd-namespace moby --log-level debug iptables --interface lo --duration 30s loss --mode nth --every 5 --packet 0 test-combined-ctr &
    PUMBA_PID=$!
    sleep 2

    run sudo ctr -n moby t exec --exec-id check-ipt-nth2 test-combined-ctr iptables -L INPUT -n -v
    echo "iptables nth combined output: $output"

    sudo kill $PUMBA_PID 2>/dev/null || kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    [[ "$output" =~ "DROP" ]] || [[ "$output" =~ "statistic" ]]
}
