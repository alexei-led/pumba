#!/usr/bin/env bats

# `pumba status [--check]` via the podman runtime, mirroring a minimal
# tests/status.bats.
#
# Prerequisites:
#   * rootful podman — status inspects the target netns; rootless is refused
#     like netem/iptables (covered by tests/podman_error_handling.bats).
#   * nicolaka/netshoot (target with tc and iptables) and pumba-alpine-nettools
#     (inspect sidecar) images
#
# Run locally:
#   * macOS: podman machine ssh sudo bats tests/podman_status.bats
#   * Linux: sudo bats tests/podman_status.bats

load test_helper

bats_require_minimum_version 1.5.0

NETTOOLS="ghcr.io/alexei-led/pumba-alpine-nettools:latest"

setup() {
    require_podman
    if podman info --format '{{.Host.Security.Rootless}}' 2>/dev/null | grep -qi '^true$'; then
        skip "podman is rootless — status requires rootful mode"
    fi
    podman_rm_force pdm-status-ctr
    podman pull docker.io/nicolaka/netshoot:latest >/dev/null 2>&1 || true
    podman pull "${NETTOOLS}" >/dev/null 2>&1 || true
    podman run -d --privileged --name pdm-status-ctr docker.io/nicolaka/netshoot:latest sleep infinity >/dev/null 2>&1
    wait_for_running podman pdm-status-ctr
}

teardown() {
    pkill -f "pumba.*pdm-status-ctr" 2>/dev/null || true
    kill %1 2>/dev/null || true
    podman_rm_force pdm-status-ctr
    for sc in $(podman ps -aq --filter "label=com.gaiaadm.pumba.skip=true" 2>/dev/null); do
        podman rm -f "$sc" >/dev/null 2>&1 || true
    done
}

# Run jq filter $1 on the state of the only target.result in $output.
assert_state() {
    assert_events "[.[] | select(.type == \"target.result\")] | length == 1 and (.[0].state | $1)"
}

status_ndjson() {
    run --separate-stderr pumba --runtime podman -o ndjson status --tc-image "${NETTOOLS}" --pull-image=false "$@" pdm-status-ctr
}

@test "Should find a tagged iptables rule, dirty after kill -9 and clean after manual cleanup via podman" {
    pumba --runtime podman --run-id pdm-st-ipt iptables --interface lo --pull-image=false --duration 60s \
        loss --probability 1.0 pdm-status-ctr >/dev/null 2>&1 3>&- &
    PUMBA_PID=$!
    wait_for 10 "podman exec pdm-status-ctr iptables -S INPUT | grep -q 'pumba:pdm-st-ipt'" "tagged iptables rule"

    status_ndjson
    assert_success
    assert_state '(.rules | length) == 1 and (.rules[0] | .run_id == "pdm-st-ipt" and (.spec | contains("--comment \"pumba:pdm-st-ipt\"")))'

    kill -9 "$PUMBA_PID"
    wait "$PUMBA_PID" 2>/dev/null || true

    status_ndjson --check --run-id pdm-st-ipt
    assert_failure 1
    assert_run_end '.status == "dirty"'

    podman exec pdm-status-ctr iptables -F INPUT

    status_ndjson --check --run-id pdm-st-ipt
    assert_success
    assert_run_end '.status == "clean"'
}

@test "Should report a netem qdisc as pumba-owned via podman" {
    pumba --runtime podman --run-id pdm-st-netem netem --interface lo --pull-image=false --duration 60s \
        delay --time 100 pdm-status-ctr >/dev/null 2>&1 3>&- &
    wait_for 10 "podman exec pdm-status-ctr tc qdisc show dev lo | grep -q netem" "netem qdisc"

    status_ndjson --check
    assert_failure 1
    assert_state '.qdiscs == [{"device":"lo","kind":"netem","ownership":"pumba-owned","run_id":null}]'
    assert_run_end '.status == "dirty"'
}

@test "Should report unknown when status --check selects nothing via podman" {
    run --separate-stderr pumba --runtime podman -o ndjson status --check pdm-missing-12345
    assert_failure 1
    assert_run_end '.status == "unknown" and .coverage.missing_targets == ["pdm-missing-12345"]'
}
