#!/usr/bin/env bats

# Stress-ng tests for the podman runtime. Covers both placement modes:
#
#   * default (child-cgroup) mode — pumba resolves the target's cgroup from the
#     host view of /proc/<pid>/cgroup (Podman's modern default is cgroupns=private
#     which hides ancestry from in-container reads), then creates a sidecar
#     under HostConfig.Resources.CgroupParent.
#   * inject-cgroup (--inject-cgroup) mode — pumba starts the sidecar with
#     cgroupns=host + bind-mounted /sys/fs/cgroup and runs /cg-inject to move
#     the stress-ng PID into the target's exact cgroup.
#
# Verification is indirect: the "resolved podman target cgroup" log confirms
# the host-side resolver ran, pumba exits zero (which requires the sidecar to
# create/start — systemd rejects an invalid scope parent), AutoRemove reaps
# the sidecar, and the target stays running. A bad parent or leaf name would
# fail sidecar create and surface here as a non-zero pumba exit.
#
# Prerequisites:
#   * rootful podman — cgroup writes and CAP_SYS_ADMIN for cg-inject require
#     root. Rootless failure is covered by tests/podman_error_handling.bats.
#   * `bats`, `podman`, `pumba` on PATH
#   * ghcr.io/alexei-led/stress-ng:latest image (provides both /stress-ng and
#     /cg-inject binaries)
#
# Run locally:
#   * macOS: podman machine ssh sudo bats tests/podman_stress.bats
#   * Linux: sudo bats tests/podman_stress.bats

load test_helper

STRESS_IMAGE="ghcr.io/alexei-led/stress-ng:latest"

setup() {
    require_podman
    if podman info --format '{{.Host.Security.Rootless}}' 2>/dev/null | grep -qi '^true$'; then
        skip "podman is rootless — stress sidecar requires rootful mode"
    fi
    podman_rm_force pdm_stress_victim
    podman run -d --name pdm_stress_victim alpine sh -c "apk add --no-cache stress-ng >/dev/null 2>&1 && sleep infinity" >/dev/null 2>&1
    wait_for 30 "podman exec pdm_stress_victim which stress-ng >/dev/null 2>&1" "stress-ng to be installed"
    podman pull "${STRESS_IMAGE}" >/dev/null 2>&1 || true
}

teardown() {
    sudo pkill -f "pumba.*stress.*pdm_stress_victim" 2>/dev/null || true
    kill %1 2>/dev/null || true
    podman_rm_force pdm_stress_victim
    # AutoRemove is set on stress sidecars, but reap any stragglers left by
    # aborted test runs.
    for sc in $(podman ps -aq --filter "label=com.gaiaadm.pumba.skip=true" 2>/dev/null); do
        podman rm -f "$sc" >/dev/null 2>&1 || true
    done
}

@test "Should handle stress on non-existent container via podman runtime" {
    run pumba --runtime podman --log-level debug stress --duration 5s --stressors="--cpu 1 --timeout 2s" nonexistent_container_12345
    # Pumba should handle gracefully — exit 0 (no matching containers found)
    assert_success
}

@test "Should run stress in dry-run mode via podman runtime" {
    full_id=$(podman inspect --format="{{.Id}}" pdm_stress_victim)

    run pumba --runtime podman --dry-run --log-level debug stress --duration 5s --stress-image "${STRESS_IMAGE}" --stressors="--cpu 1 --timeout 2s" "$full_id"
    assert_success

    [ "$(podman inspect -f '{{.State.Status}}' pdm_stress_victim)" = "running" ]

    # No sidecar should have been created in dry-run mode
    run podman ps -aq --filter "label=com.gaiaadm.pumba.skip=true"
    [ -z "$output" ]
}

@test "Should run stress via sidecar (child-cgroup mode) on podman" {
    full_id=$(podman inspect --format="{{.Id}}" pdm_stress_victim)

    run pumba --runtime podman --log-level debug \
        stress --duration 10s --stress-image "${STRESS_IMAGE}" --stressors="--cpu 1 --cpu-method loop --timeout 3s" "$full_id"

    echo "Pumba output: $output"
    assert_success
    assert_output --partial "resolved podman target cgroup"

    # Verify sidecar was cleaned up (AutoRemove)
    run podman ps -aq --filter "label=com.gaiaadm.pumba.skip=true"
    [ -z "$output" ]

    [ "$(podman inspect -f '{{.State.Status}}' pdm_stress_victim)" = "running" ]
}

# NOTE: inject-cgroup mode is tested in tests/skip_ci/podman_stress_inject_cgroup.bats
# (excluded from CI). Stock Podman 4.9.x on Ubuntu 24.04 creates the libpod init
# sub-cgroup `<scope>/container` transiently and rmdir's it mid-flight, racing
# cg-inject's write. Podman 5.x (`podman machine`, Fedora CoreOS) keeps it stable.
# See pkg/runtime/podman/stress.go::resolveCgroup for the resolution logic.

