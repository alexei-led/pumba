#!/usr/bin/env bats

load test_helper

STRESS_IMAGE="ghcr.io/alexei-led/stress-ng:latest"

setup() {
    # Use Docker to create container (has networking for apk add)
    # Then target it via containerd using its full ID
    docker rm -f stress_victim >/dev/null 2>&1 || true
    docker run -d --name stress_victim alpine sh -c "apk add --no-cache stress-ng >/dev/null 2>&1 && sleep infinity"
    # Wait for stress-ng to be installed
    wait_for 30 "docker exec stress_victim which stress-ng >/dev/null 2>&1" "stress-ng to be installed"
}

teardown() {
    sudo pkill -f "pumba.*stress.*stress_victim" 2>/dev/null || true
    kill %1 2>/dev/null || true
    docker rm -f stress_victim >/dev/null 2>&1 || true
    # Clean up any leftover stress sidecar containers
    for sc in $(sudo ctr -n moby c ls -q 2>/dev/null | grep pumba-stress); do
        sudo ctr -n moby t kill -s SIGKILL "$sc" >/dev/null 2>&1 || true
        sudo ctr -n moby c rm "$sc" >/dev/null 2>&1 || true
        sudo ctr -n moby snapshots rm "${sc}-snapshot" >/dev/null 2>&1 || true
    done
}

# ── Direct exec mode (existing behaviour) ────────────────────────────────

@test "Should handle stress on non-existent container via containerd runtime" {
    run pumba --log-level debug stress --duration 5s --stressors="--cpu 1 --timeout 2s" nonexistent_container_12345
    # Pumba should handle gracefully — exit 0 (no matching containers found)
    assert_success
}

@test "Should run stress in dry-run mode via containerd runtime" {
    full_id=$(docker inspect --format="{{.Id}}" stress_victim)

    run pumba --dry-run --log-level debug stress --duration 5s --stressors="--cpu 1 --timeout 2s" $full_id
    assert_success

    # Container should still be running (dry-run)
    [ "$(docker inspect -f '{{.State.Status}}' stress_victim)" = "running" ]
}

@test "Should apply stress (CPU) via containerd runtime" {
    # Get full container ID (Docker-created containers live in moby namespace)
    full_id=$(docker inspect --format="{{.Id}}" stress_victim)

    # Run stress via pumba containerd runtime
    # The containerd StressContainer runs stress-ng directly inside the container via exec
    # Duration is the pumba-side timeout; stressors --timeout is stress-ng side
    run pumba --log-level debug stress --duration 10s --stressors="--cpu 1 --timeout 3s" $full_id

    echo "Pumba stress output: $output"

    # Pumba should exit successfully
    assert_success

    # In containerd mode, stress-ng stdout is not forwarded to pumba output
    # But debug log should confirm completion
    assert_output --partial "stress-ng completed"
}

# ── Default sidecar mode (--stress-image) ────────────────────────────────

@test "Should run stress sidecar in dry-run mode via containerd runtime" {
    require_containerd
    full_id=$(docker inspect --format="{{.Id}}" stress_victim)

    run sudo pumba --runtime containerd --containerd-namespace moby --dry-run --log-level debug \
        stress --duration 5s --stress-image ${STRESS_IMAGE} --stressors="--cpu 1" $full_id
    assert_success

    # No sidecar containers should be created in dry-run mode
    run sudo ctr -n moby c ls -q
    refute_output --partial "pumba-stress-"
}

@test "Should run stress via sidecar (child-cgroup mode) on containerd" {
    require_containerd
    full_id=$(docker inspect --format="{{.Id}}" stress_victim)

    # Pull image first (in moby namespace)
    ctr_pull_image moby ${STRESS_IMAGE}

    # Run stress sidecar — pumba creates a container with /stress-ng as entrypoint,
    # placed in the target's cgroup parent (child cgroup)
    run sudo pumba --runtime containerd --containerd-namespace moby --log-level debug \
        stress --duration 10s --stress-image ${STRESS_IMAGE} --stressors="--cpu 1 --timeout 3s" $full_id

    echo "Pumba output: $output"

    assert_success

    # Verify sidecar was cleaned up
    run sudo ctr -n moby c ls -q
    refute_output --partial "pumba-stress-"

    # Target should still be running
    assert_container_running stress_victim
}

# ── Inject-cgroup mode (--inject-cgroup --stress-image) ──────────────────

@test "Should run stress via inject-cgroup sidecar on containerd" {
    require_containerd
    full_id=$(docker inspect --format="{{.Id}}" stress_victim)

    ctr_pull_image moby ${STRESS_IMAGE}

    # Run inject-cgroup stress — pumba creates a sidecar with /cg-inject as entrypoint,
    # host cgroupns, /sys/fs/cgroup mount, and --cgroup-path pointing to the target
    run sudo pumba --runtime containerd --containerd-namespace moby --log-level debug \
        stress --duration 10s --inject-cgroup --stress-image ${STRESS_IMAGE} --stressors="--cpu 1 --timeout 3s" $full_id

    echo "Pumba output: $output"

    assert_success

    # Debug output should show the resolved cgroup path
    assert_output --partial "resolved target cgroup for stress sidecar"

    # Verify sidecar was cleaned up
    run sudo ctr -n moby c ls -q
    refute_output --partial "pumba-stress-"

    # Target should still be running
    assert_container_running stress_victim
}
