#!/usr/bin/env bats

# Tests for the sidecar container approach in containerd runtime.
# The sidecar creates a helper container that shares the target's network namespace,
# allowing netem/iptables even when the target container lacks tc/iptables tools.

load test_helper

setup() {
    sudo ctr -n moby t kill -s SIGKILL test-sidecar-target >/dev/null 2>&1 || true
    sudo ctr -n moby c rm test-sidecar-target >/dev/null 2>&1 || true
    # Use plain alpine — no tc tools installed
    sudo ctr -n moby i pull docker.io/library/alpine:latest >/dev/null 2>&1
    # Create with a dummy network interface for testing
    sudo ctr -n moby run -d --privileged docker.io/library/alpine:latest test-sidecar-target \
        sh -c "ip link add dummy0 type dummy && ip link set dummy0 up && sleep infinity" >/dev/null 2>&1
    sleep 1
}

teardown() {
    sudo pkill -f "pumba.*netem.*test-sidecar-target" 2>/dev/null || true
    kill %1 2>/dev/null || true
    sudo ctr -n moby t kill -s SIGKILL test-sidecar-target >/dev/null 2>&1 || true
    sudo ctr -n moby c rm test-sidecar-target >/dev/null 2>&1 || true
    # Clean up any leftover sidecar containers
    for sc in $(sudo ctr -n moby c ls -q 2>/dev/null | grep pumba-sidecar); do
        sudo ctr -n moby t kill -s SIGKILL $sc >/dev/null 2>&1 || true
        sudo ctr -n moby c rm $sc >/dev/null 2>&1 || true
    done
}

@test "Should apply netem delay via sidecar container (tc-image)" {
    # Run pumba with --tc-image to use sidecar approach
    # The target container (alpine) does NOT have tc installed
    sudo pumba --runtime containerd --containerd-namespace moby --log-level debug netem --interface dummy0 --tc-image docker.io/nicolaka/netshoot:latest --duration 30s delay --time 100 test-sidecar-target &
    PUMBA_PID=$!
    sleep 3

    # Check if sidecar was created
    run sudo ctr -n moby c ls -q
    echo "Containers: $output"

    # Verify tc rules were applied to the target's dummy0 interface
    # We need to check via the sidecar (target has no tc), but the sidecar is short-lived per exec.
    # Instead, use nsenter from the host to check the target's network namespace
    target_pid=$(sudo ctr -n moby t ls | grep test-sidecar-target | awk '{print $2}')
    run sudo nsenter -t $target_pid -n tc qdisc show dev dummy0
    echo "TC output: $output"

    sudo kill $PUMBA_PID 2>/dev/null || kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    [[ "$output" =~ "netem" ]]
    [[ "$output" =~ "delay" ]]
}
