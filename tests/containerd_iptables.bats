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
    # Kill backgrounded pumba
    kill %1 2>/dev/null || true
    sudo ctr -n moby t kill -s SIGKILL test-ipt-ctr >/dev/null 2>&1 || true
    sudo ctr -n moby c rm test-ipt-ctr >/dev/null 2>&1 || true
}

@test "Should apply iptables packet loss via containerd runtime" {
    # Run pumba in background with long duration
    pumba --runtime containerd --containerd-namespace moby --log-level debug iptables --interface lo --duration 30s loss --probability 1.0 test-ipt-ctr &
    PUMBA_PID=$!
    sleep 2

    # Check iptables rules inside the container
    run sudo ctr -n moby t exec --exec-id check-ipt test-ipt-ctr iptables -L INPUT -n -v
    echo "iptables output: $output"

    # Kill pumba
    kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    # There should be some DROP rule
    [[ "$output" =~ "DROP" ]] || [[ "$output" =~ "drop" ]] || [[ "$output" =~ "statistic" ]]
}
