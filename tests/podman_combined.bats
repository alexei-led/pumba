#!/usr/bin/env bats

# Combined netem + iptables scenarios via podman runtime, mirroring
# tests/containerd_combined.bats. Verifies that multiple chaos actions can
# be applied to the same container in sequence or overlapping windows.
#
# Prerequisites:
#   * rootful podman — netem + iptables both need CAP_NET_ADMIN on the
#     target's netns.
#   * nicolaka/netshoot image (brings tc/iproute2/iptables into the target).
#
# Run locally:
#   * macOS: podman machine ssh sudo bats tests/podman_combined.bats
#   * Linux: sudo bats tests/podman_combined.bats

load test_helper

NETTOOLS_IMAGE="ghcr.io/alexei-led/pumba-alpine-nettools:latest"

setup() {
    require_podman
    if podman info --format '{{.Host.Security.Rootless}}' 2>/dev/null | grep -qi '^true$'; then
        skip "podman is rootless — netem/iptables require rootful mode"
    fi
    podman_rm_force pdm-combined-ctr
    podman pull docker.io/nicolaka/netshoot:latest >/dev/null 2>&1 || true
    podman pull "$NETTOOLS_IMAGE" >/dev/null 2>&1 || true
    podman run -d --privileged --name pdm-combined-ctr docker.io/nicolaka/netshoot:latest \
        sh -c "ip link add dummy0 type dummy && ip link set dummy0 up && sleep infinity" >/dev/null 2>&1
    wait_for_running podman pdm-combined-ctr
}

teardown() {
    sudo pkill -f "pumba.*pdm-combined-ctr" 2>/dev/null || true
    kill %1 2>/dev/null || true
    podman_rm_force pdm-combined-ctr
    # Reap any lingering sidecar containers (identified by pumba skip label)
    for sc in $(podman ps -aq --filter "label=com.gaiaadm.pumba.skip=true" 2>/dev/null); do
        podman rm -f "$sc" >/dev/null 2>&1 || true
    done
}

@test "Should apply netem then iptables on same container via podman runtime" {
    # Apply netem delay (short duration, returns immediately after)
    run pumba --runtime podman --log-level debug netem --interface dummy0 --pull-image=false --duration 2s delay --time 100 pdm-combined-ctr
    echo "Netem status: $status, output: $output"
    assert_success

    # Apply iptables loss on the same container
    pumba --runtime podman --log-level debug iptables --interface lo --pull-image=false --duration 30s loss --probability 1.0 pdm-combined-ctr &
    PUMBA_PID=$!
    sleep 2

    run podman exec pdm-combined-ctr iptables -L INPUT -n -v
    echo "iptables output: $output"

    kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    assert_output --partial "DROP"
    assert_output --partial "statistic"
}

@test "Should apply rate limit with iptables protocol filter via podman runtime" {
    # Apply rate limit (netem rate)
    run pumba --runtime podman --log-level debug netem --interface dummy0 --pull-image=false --duration 2s rate --rate 1mbit pdm-combined-ctr
    echo "Rate limit status: $status"
    assert_success

    # Apply iptables with protocol filter
    pumba --runtime podman --log-level debug iptables --interface lo --pull-image=false --protocol icmp --duration 30s loss --probability 0.5 pdm-combined-ctr &
    PUMBA_PID=$!
    sleep 2

    run podman exec pdm-combined-ctr iptables -L INPUT -n -v
    echo "iptables protocol output: $output"

    kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    assert_output --partial "DROP"
    assert_output --partial "icmp"
}

@test "Should apply netem with target IP filter via podman runtime" {
    pumba --runtime podman --log-level debug netem --interface dummy0 --pull-image=false --target 8.8.8.8 --duration 30s delay --time 100 pdm-combined-ctr &
    PUMBA_PID=$!
    sleep 2

    # With a target filter pumba builds a prio qdisc with netem on the matching band
    run podman exec pdm-combined-ctr tc qdisc show dev dummy0
    echo "TC output: $output"

    kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    assert_output --partial "netem"
}

@test "Should apply iptables nth mode in combined scenario via podman runtime" {
    run pumba --runtime podman --log-level debug netem --interface dummy0 --pull-image=false --duration 2s delay --time 50 pdm-combined-ctr
    assert_success

    pumba --runtime podman --log-level debug iptables --interface lo --pull-image=false --duration 30s loss --mode nth --every 5 --packet 0 pdm-combined-ctr &
    PUMBA_PID=$!
    sleep 2

    run podman exec pdm-combined-ctr iptables -L INPUT -n -v
    echo "iptables nth combined output: $output"

    kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    assert_output --partial "DROP"
    assert_output --partial "statistic"
}
