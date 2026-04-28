#!/usr/bin/env bats

# Inject-cgroup stress test for the podman runtime.
#
# Excluded from GitHub CI because stock Ubuntu 24.04 ships Podman 4.9.x, which
# creates the libpod init `<scope>/container` sub-cgroup transiently and then
# rmdir's it during container startup. This races cg-inject's write to
# `<scope>/container/cgroup.procs` (succeeds open, fails write with ENOENT).
# Podman 5.x (`podman machine`, Fedora CoreOS) keeps `/container` stable for
# the container's lifetime and this test passes.
#
# Prerequisites:
#   * Podman 5.x (rootful) — `podman machine` on macOS, Fedora CoreOS / RHEL 9+
#     rootful podman on Linux
#   * ghcr.io/alexei-led/stress-ng:0.20.01 image (first tag that ships
#     /cg-inject + /stress-ng)
#
# Run locally (macOS):
#   podman machine ssh sudo bats tests/skip_ci/podman_stress_inject_cgroup.bats

load ../test_helper

# Pinned to 0.20.01 — see tests/containerd_stress.bats for full rationale.
STRESS_IMAGE="ghcr.io/alexei-led/stress-ng:0.20.01"

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
    for sc in $(podman ps -aq --filter "label=com.gaiaadm.pumba.skip=true" 2>/dev/null); do
        podman rm -f "$sc" >/dev/null 2>&1 || true
    done
}

@test "Should run stress via inject-cgroup sidecar on podman" {
    full_id=$(podman inspect --format="{{.Id}}" pdm_stress_victim)

    run pumba --runtime podman --log-level debug \
        stress --duration 10s --inject-cgroup --stress-image "${STRESS_IMAGE}" --stressors="--cpu 1 --cpu-method loop --timeout 3s" "$full_id"

    echo "Pumba output: $output"
    assert_success
    assert_output --partial "resolved podman target cgroup"

    run podman ps -aq --filter "label=com.gaiaadm.pumba.skip=true"
    [ -z "$output" ]

    [ "$(podman inspect -f '{{.State.Status}}' pdm_stress_victim)" = "running" ]
}
