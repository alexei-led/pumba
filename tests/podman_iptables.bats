#!/usr/bin/env bats

# iptables-based chaos tests for the podman runtime mirroring
# tests/containerd_iptables.bats: loss rules with nth mode, source/destination
# IP filters, and port filters applied via pumba, verified inside the target
# via `podman exec target iptables -L`.
#
# Prerequisites:
#   * rootful podman — iptables chaos inside the target netns needs CAP_NET_ADMIN.
#     Rootless failure mode is covered by tests/podman_error_handling.bats.
#   * `bats`, `podman`, `pumba` on PATH
#   * nicolaka/netshoot image (provides iptables inside the target)
#
# Run locally:
#   * macOS: podman machine ssh sudo bats tests/podman_iptables.bats
#   * Linux: sudo bats tests/podman_iptables.bats

load test_helper

setup() {
    require_podman
    if podman info --format '{{.Host.Security.Rootless}}' 2>/dev/null | grep -qi '^true$'; then
        skip "podman is rootless — iptables requires rootful mode"
    fi
    podman_rm_force pdm-ipt-ctr
    podman pull docker.io/nicolaka/netshoot:latest >/dev/null 2>&1 || true
    podman pull ghcr.io/alexei-led/pumba-alpine-nettools:latest >/dev/null 2>&1 || true
    podman run -d --privileged --name pdm-ipt-ctr docker.io/nicolaka/netshoot:latest sleep infinity >/dev/null 2>&1
    sleep 1
}

teardown() {
    sudo pkill -f "pumba.*iptables.*pdm-ipt-ctr" 2>/dev/null || true
    kill %1 2>/dev/null || true
    podman_rm_force pdm-ipt-ctr
    for sc in $(podman ps -q --filter "label=com.gaiaadm.pumba.skip=true" 2>/dev/null); do
        podman rm -f "$sc" >/dev/null 2>&1 || true
    done
}

@test "Should apply iptables packet loss via podman runtime" {
    pumba --runtime podman --log-level debug iptables --interface lo --pull-image=false --duration 30s loss --probability 1.0 pdm-ipt-ctr &
    PUMBA_PID=$!
    sleep 2

    run podman exec pdm-ipt-ctr iptables -L INPUT -n -v
    echo "iptables output: $output"
    assert_output --partial "DROP"
    assert_output --partial "statistic"

    kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    sleep 1
    run podman exec pdm-ipt-ctr iptables -L INPUT -n -v
    echo "iptables after cleanup: $output"
    refute_output --partial "DROP"
}

@test "Should apply iptables loss with nth mode via podman runtime" {
    pumba --runtime podman --log-level debug iptables --interface lo --pull-image=false --duration 30s loss --mode nth --every 3 pdm-ipt-ctr &
    PUMBA_PID=$!
    sleep 2

    run podman exec pdm-ipt-ctr iptables -L INPUT -n -v
    echo "iptables nth output: $output"
    assert_output --partial "DROP"
    assert_output --partial "statistic"

    kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    sleep 1
    run podman exec pdm-ipt-ctr iptables -L INPUT -n -v
    refute_output --partial "DROP"
}

@test "Should handle iptables on non-existent container via podman runtime" {
    run pumba --runtime podman --log-level debug iptables --interface lo --pull-image=false --duration 2s loss --probability 1.0 nonexistent_container_12345
    assert_success
}

@test "Should apply iptables loss with destination IP filter via podman runtime" {
    pumba --runtime podman --log-level debug iptables --interface lo --destination 10.0.0.0/8 --pull-image=false --duration 30s loss --probability 1.0 pdm-ipt-ctr &
    PUMBA_PID=$!
    sleep 2

    run podman exec pdm-ipt-ctr iptables -L INPUT -n -v
    echo "iptables destination filter output: $output"
    assert_output --partial "DROP"
    assert_output --partial "10.0.0.0"

    kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    sleep 1
    run podman exec pdm-ipt-ctr iptables -L INPUT -n -v
    refute_output --partial "DROP"
}

@test "Should apply iptables loss with port filters via podman runtime" {
    pumba --runtime podman --log-level debug iptables --interface lo --protocol tcp --dst-port 80 --pull-image=false --duration 30s loss --probability 1.0 pdm-ipt-ctr &
    PUMBA_PID=$!
    sleep 2

    run podman exec pdm-ipt-ctr iptables -L INPUT -n -v
    echo "iptables port filter output: $output"
    assert_output --partial "DROP"
    assert_output --partial "tcp"
    assert_output --partial "dpt:80"

    kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    sleep 1
    run podman exec pdm-ipt-ctr iptables -L INPUT -n -v
    refute_output --partial "DROP"
}

@test "Should apply iptables loss with source IP filter via podman runtime" {
    pumba --runtime podman --log-level debug iptables --interface lo --source 10.0.0.0/8 --pull-image=false --duration 30s loss --probability 1.0 pdm-ipt-ctr &
    PUMBA_PID=$!
    sleep 2

    run podman exec pdm-ipt-ctr iptables -L INPUT -n -v
    echo "iptables source filter output: $output"
    assert_output --partial "DROP"
    assert_output --partial "10.0.0.0"

    kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    sleep 1
    run podman exec pdm-ipt-ctr iptables -L INPUT -n -v
    refute_output --partial "DROP"
}
