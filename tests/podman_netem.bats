#!/usr/bin/env bats

# Netem tests for the podman runtime (delay, loss, corrupt, duplicate, rate,
# loss-state, loss-gemodel, port filters) mirroring tests/containerd_netem.bats.
#
# Prerequisites:
#   * rootful podman — netem needs NET_ADMIN on the target's netns, which
#     rootless podman (slirp4netns/pasta) does not grant. Pumba fails fast
#     with a diagnostic error in rootless mode; those tests live in
#     tests/podman_error_handling.bats.
#   * `bats`, `podman`, `pumba` on PATH
#   * nicolaka/netshoot image (brings tc/iproute2 into the target)
#
# Run locally:
#   * macOS: podman machine ssh sudo bats tests/podman_netem.bats
#   * Linux: sudo bats tests/podman_netem.bats

load test_helper

setup() {
    require_podman
    # Skip when rootless — netem cannot succeed without CAP_NET_ADMIN on the
    # target's netns; the error-handling suite covers the failure path.
    if podman info --format '{{.Host.Security.Rootless}}' 2>/dev/null | grep -qi '^true$'; then
        skip "podman is rootless — netem requires rootful mode"
    fi
    podman_rm_force pdm-netem-ctr
    podman pull docker.io/nicolaka/netshoot:latest >/dev/null 2>&1 || true
    podman pull ghcr.io/alexei-led/pumba-alpine-nettools:latest >/dev/null 2>&1 || true
    podman run -d --privileged --name pdm-netem-ctr docker.io/nicolaka/netshoot:latest \
        sh -c "ip link add dummy0 type dummy && ip link set dummy0 up && sleep infinity" >/dev/null 2>&1
    sleep 1
}

teardown() {
    sudo pkill -f "pumba.*netem.*pdm-netem-ctr" 2>/dev/null || true
    kill %1 2>/dev/null || true
    podman_rm_force pdm-netem-ctr
    # Reap any lingering sidecar containers (identified by pumba skip label)
    for sc in $(podman ps -q --filter "label=com.gaiaadm.pumba.skip=true" 2>/dev/null); do
        podman rm -f "$sc" >/dev/null 2>&1 || true
    done
}

@test "Should apply netem delay via podman runtime" {
    run podman exec pdm-netem-ctr ip link show dummy0
    assert_success

    pumba --runtime podman --log-level debug netem --interface dummy0 --pull-image=false --duration 30s delay --time 100 pdm-netem-ctr &
    PUMBA_PID=$!
    sleep 2

    run podman exec pdm-netem-ctr tc qdisc show dev dummy0
    echo "TC output: $output"
    assert_output --partial "netem"
    assert_output --partial "delay 100"

    kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    sleep 1
    run podman exec pdm-netem-ctr tc qdisc show dev dummy0
    echo "TC after cleanup: $output"
    refute_output --partial "netem"
}

@test "Should apply packet loss via podman runtime" {
    pumba --runtime podman --log-level debug netem --interface dummy0 --pull-image=false --duration 30s loss --percent 50 pdm-netem-ctr &
    PUMBA_PID=$!
    sleep 2

    run podman exec pdm-netem-ctr tc qdisc show dev dummy0
    echo "TC output: $output"
    assert_output --partial "netem"
    assert_output --partial "loss 50%"

    kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    sleep 1
    run podman exec pdm-netem-ctr tc qdisc show dev dummy0
    refute_output --partial "netem"
}

@test "Should apply packet duplicate via podman runtime" {
    pumba --runtime podman --log-level debug netem --interface dummy0 --pull-image=false --duration 30s duplicate --percent 50 pdm-netem-ctr &
    PUMBA_PID=$!
    sleep 2

    run podman exec pdm-netem-ctr tc qdisc show dev dummy0
    echo "TC output: $output"
    assert_output --partial "netem"
    assert_output --partial "duplicate 50%"

    kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    sleep 1
    run podman exec pdm-netem-ctr tc qdisc show dev dummy0
    refute_output --partial "netem"
}

@test "Should apply packet corruption via podman runtime" {
    pumba --runtime podman --log-level debug netem --interface dummy0 --pull-image=false --duration 30s corrupt --percent 50 pdm-netem-ctr &
    PUMBA_PID=$!
    sleep 2

    run podman exec pdm-netem-ctr tc qdisc show dev dummy0
    echo "TC output: $output"
    assert_output --partial "netem"
    assert_output --partial "corrupt 50%"

    kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    sleep 1
    run podman exec pdm-netem-ctr tc qdisc show dev dummy0
    refute_output --partial "netem"
}

@test "Should handle netem on non-existent container via podman runtime" {
    run pumba --runtime podman --log-level debug netem --interface dummy0 --pull-image=false --duration 2s delay --time 100 nonexistent_container_12345
    assert_success
}

@test "Should apply loss-state model via podman runtime" {
    pumba --runtime podman --log-level debug netem --interface dummy0 --pull-image=false --duration 30s loss-state --p13 5 --p31 15 --p32 10 --p23 20 --p14 5 pdm-netem-ctr &
    PUMBA_PID=$!
    sleep 2

    run podman exec pdm-netem-ctr tc qdisc show dev dummy0
    echo "TC output: $output"
    assert_output --partial "netem"

    kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    sleep 1
    run podman exec pdm-netem-ctr tc qdisc show dev dummy0
    refute_output --partial "netem"
}

@test "Should apply loss-gemodel model via podman runtime" {
    pumba --runtime podman --log-level debug netem --interface dummy0 --pull-image=false --duration 30s loss-gemodel --pg 5 --pb 20 --one-h 80 --one-k 10 pdm-netem-ctr &
    PUMBA_PID=$!
    sleep 2

    run podman exec pdm-netem-ctr tc qdisc show dev dummy0
    echo "TC output: $output"
    assert_output --partial "netem"

    kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    sleep 1
    run podman exec pdm-netem-ctr tc qdisc show dev dummy0
    refute_output --partial "netem"
}

@test "Should apply delay with normal distribution via podman runtime" {
    pumba --runtime podman --log-level debug netem --interface dummy0 --pull-image=false --duration 30s delay --time 100 --jitter 30 --distribution normal pdm-netem-ctr &
    PUMBA_PID=$!
    sleep 2

    run podman exec pdm-netem-ctr tc qdisc show dev dummy0
    echo "TC output: $output"
    assert_output --partial "netem"
    assert_output --partial "delay 100"

    kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    sleep 1
    run podman exec pdm-netem-ctr tc qdisc show dev dummy0
    refute_output --partial "netem"
}

@test "Should apply rate limit with cell options via podman runtime" {
    pumba --runtime podman --log-level debug netem --interface dummy0 --pull-image=false --duration 30s rate --rate 1mbit --packetoverhead 10 --cellsize 1500 --celloverhead 20 pdm-netem-ctr &
    PUMBA_PID=$!
    sleep 2

    run podman exec pdm-netem-ctr tc qdisc show dev dummy0
    echo "TC output: $output"
    assert_output --partial "netem"
    assert_output --partial "rate 1Mbit"

    kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    sleep 1
    run podman exec pdm-netem-ctr tc qdisc show dev dummy0
    refute_output --partial "netem"
}

@test "Should apply rate limiting via podman runtime" {
    pumba --runtime podman --log-level debug netem --interface dummy0 --pull-image=false --duration 30s rate --rate 100kbit pdm-netem-ctr &
    PUMBA_PID=$!
    sleep 2

    run podman exec pdm-netem-ctr tc qdisc show dev dummy0
    echo "TC output: $output"
    assert_output --partial "netem"
    assert_output --partial "rate 100Kbit"

    kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    sleep 1
    run podman exec pdm-netem-ctr tc qdisc show dev dummy0
    refute_output --partial "netem"
}

@test "Should apply netem delay with egress port filter via podman runtime" {
    pumba --runtime podman --log-level debug netem --interface dummy0 --pull-image=false --egress-port 80 --duration 30s delay --time 100 pdm-netem-ctr &
    PUMBA_PID=$!
    sleep 2

    run podman exec pdm-netem-ctr tc qdisc show dev dummy0
    echo "TC output: $output"
    assert_output --partial "netem"

    kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    sleep 1
    run podman exec pdm-netem-ctr tc qdisc show dev dummy0
    refute_output --partial "netem"
}

@test "Should apply netem delay with ingress port filter via podman runtime" {
    pumba --runtime podman --log-level debug netem --interface dummy0 --pull-image=false --ingress-port 443 --duration 30s delay --time 100 pdm-netem-ctr &
    PUMBA_PID=$!
    sleep 2

    run podman exec pdm-netem-ctr tc qdisc show dev dummy0
    echo "TC output: $output"
    assert_output --partial "netem"

    kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    sleep 1
    run podman exec pdm-netem-ctr tc qdisc show dev dummy0
    refute_output --partial "netem"
}
