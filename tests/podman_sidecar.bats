#!/usr/bin/env bats

# Sidecar lifecycle tests for the podman runtime. Covers the sidecar-image
# mode of the netem chaos command: when the target container lacks tc/iproute2,
# pumba launches a sidecar sharing the target's network namespace. Verifies
# (a) the sidecar is created with the pumba skip label so recursive listing
# ignores it, (b) netem rules are applied to the target's netns, and (c) the
# sidecar is torn down on success / on pumba termination.
#
# Prerequisites:
#   * rootful podman (netem inside the target netns needs CAP_NET_ADMIN)
#   * `bats`, `podman`, `pumba` on PATH
#   * alpine:latest (target — NO tc installed)
#   * nicolaka/netshoot:latest (sidecar — brings tc)
#
# Run locally:
#   * macOS: podman machine ssh sudo bats tests/podman_sidecar.bats
#   * Linux: sudo bats tests/podman_sidecar.bats

load test_helper

setup() {
    require_podman
    if podman info --format '{{.Host.Security.Rootless}}' 2>/dev/null | grep -qi '^true$'; then
        skip "podman is rootless — sidecar netem requires rootful mode"
    fi
    podman_rm_force pdm-sidecar-target
    podman pull docker.io/library/alpine:latest >/dev/null 2>&1 || true
    podman pull docker.io/nicolaka/netshoot:latest >/dev/null 2>&1 || true
    podman run -d --privileged --name pdm-sidecar-target docker.io/library/alpine:latest \
        sh -c "ip link add dummy0 type dummy && ip link set dummy0 up && sleep infinity" >/dev/null 2>&1
    wait_for_running podman pdm-sidecar-target
}

teardown() {
    sudo pkill -f "pumba.*netem.*pdm-sidecar-target" 2>/dev/null || true
    kill %1 2>/dev/null || true
    podman_rm_force pdm-sidecar-target
    for sc in $(podman ps -aq --filter "label=com.gaiaadm.pumba.skip=true" 2>/dev/null); do
        podman rm -f "$sc" >/dev/null 2>&1 || true
    done
}

@test "Should apply netem delay via sidecar container (tc-image) on podman" {
    # The target (plain alpine) has no tc installed; pumba must launch a
    # nettools sidecar sharing the target's netns to apply tc rules.
    pumba --runtime podman --log-level debug netem --interface dummy0 --tc-image docker.io/nicolaka/netshoot:latest --pull-image=false --duration 30s delay --time 100 pdm-sidecar-target &
    PUMBA_PID=$!
    sleep 3

    # Inspect the target's netns from the host via nsenter — the target has
    # no tc binary of its own so `podman exec` won't work to check.
    target_pid=$(podman inspect -f '{{.State.Pid}}' pdm-sidecar-target)
    run sudo nsenter -t "$target_pid" -n tc qdisc show dev dummy0
    echo "TC output: $output"

    assert_output --partial "netem"
    assert_output --partial "delay"

    kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true

    # After pumba exits, sidecar should be removed (autoremove or cleanup)
    sleep 2
    run podman ps -aq --filter "label=com.gaiaadm.pumba.skip=true"
    [ -z "$output" ] || echo "leftover sidecars: $output"
    [ -z "$output" ]
}

@test "Should skip containers tagged with pumba skip label (recursive-chaos guard)" {
    # pumba's own sidecars carry com.gaiaadm.pumba.skip=true so that a
    # broad re2: pattern cannot accidentally recurse onto them. Verify
    # that invariant by planting a fake labeled container and confirming
    # pumba's regex-based `kill` ignores it.
    podman_rm_force pdm-skip-labeled
    podman run -d --name pdm-skip-labeled --label com.gaiaadm.pumba.skip=true \
        docker.io/library/alpine:latest top >/dev/null

    # Targets BOTH pdm-sidecar-target and pdm-skip-labeled via regex.
    run pumba --runtime podman --log-level debug kill "re2:^/?pdm-"
    assert_success

    # Labeled container must still be running — pumba skipped it.
    status_labeled=$(podman inspect -f '{{.State.Status}}' pdm-skip-labeled 2>/dev/null || echo "missing")
    echo "skip-labeled container status: $status_labeled"
    [ "$status_labeled" = "running" ]

    podman_rm_force pdm-skip-labeled
}

@test "Should clean up netem rules on target on SIGTERM via podman runtime" {
    # After pumba is SIGTERM'd mid-run, the target's netns must be clean —
    # no netem qdisc left behind. The tc sidecar is ephemeral by design
    # (created, executes tc, removed) so we only validate the user-visible
    # invariant: target state is restored.
    pumba --runtime podman --log-level debug netem --interface dummy0 --tc-image docker.io/nicolaka/netshoot:latest --pull-image=false --duration 60s delay --time 50 pdm-sidecar-target &
    PUMBA_PID=$!
    sleep 3

    # Netem must be applied on the target by now.
    target_pid=$(podman inspect -f '{{.State.Pid}}' pdm-sidecar-target)
    run sudo nsenter -t "$target_pid" -n tc qdisc show dev dummy0
    echo "TC mid-run: $output"
    assert_output --partial "netem"

    # SIGTERM pumba; wait for it to exit fully.
    kill -TERM $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true
    sleep 2

    # Target's netem must be cleaned up.
    run sudo nsenter -t "$target_pid" -n tc qdisc show dev dummy0
    echo "TC post-SIGTERM: $output"
    refute_output --partial "netem"

    # No tc sidecar should linger.
    run podman ps -aq --filter "label=com.gaiaadm.pumba.skip=true"
    echo "remaining sidecars: $output"
    [ -z "$output" ]
}
