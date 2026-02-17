#!/usr/bin/env bats

# Integration tests for stress --inject-cgroup mode
# Requires a stress image with both /cg-inject and /stress-ng binaries
# Skip all tests if the image is not available

STRESS_IMAGE="${PUMBA_STRESS_IMAGE:-ghcr.io/alexei-led/pumba-stress:latest}"

setup() {
    # Check if stress image with cg-inject is available
    if ! docker image inspect "${STRESS_IMAGE}" &>/dev/null; then
        skip "Stress image with cg-inject not available: ${STRESS_IMAGE}"
    fi
    # Check if cg-inject binary exists in the image (run with no args, expect error containing "cg-inject")
    if ! docker run --rm "${STRESS_IMAGE}" /cg-inject 2>&1 | grep -q "cg-inject"; then
        skip "Stress image does not contain /cg-inject binary: ${STRESS_IMAGE}"
    fi
    docker rm -f inject_cgroup_victim 2>/dev/null || true
}

teardown() {
    docker rm -f inject_cgroup_victim 2>/dev/null || true
    # Clean up any leftover stress sidecar containers
    docker ps -aq --filter "name=pumba_stress" | xargs -r docker rm -f 2>/dev/null || true
}

@test "Should stress container with inject-cgroup mode" {
    # Given: a running target container
    docker run -d --name inject_cgroup_victim alpine tail -f /dev/null
    run docker inspect -f '{{.State.Status}}' inject_cgroup_victim
    [ "$output" = "running" ]

    # When: stress with inject-cgroup
    run pumba --log-level=debug stress \
        --inject-cgroup \
        --stress-image "${STRESS_IMAGE}" \
        --duration=15s \
        --stressors="--cpu 1 --timeout 10s" \
        inject_cgroup_victim
    [ $status -eq 0 ]
    [[ $output =~ "stress testing container" ]]

    # Then: target container should still be running
    run docker inspect -f '{{.State.Status}}' inject_cgroup_victim
    [ "$output" = "running" ]
}

@test "Should complete inject-cgroup stress without leftover processes" {
    # Given: a running target container
    docker run -d --name inject_cgroup_victim alpine tail -f /dev/null

    # When: stress with inject-cgroup completes
    run pumba --log-level=debug stress \
        --inject-cgroup \
        --stress-image "${STRESS_IMAGE}" \
        --duration=15s \
        --stressors="--cpu 1 --timeout 10s" \
        inject_cgroup_victim
    [ $status -eq 0 ]

    # Then: no stress-ng processes should remain in the target
    stress_count=$(docker top inject_cgroup_victim -o pid,command 2>/dev/null | grep -c stress-ng || echo "0")
    [ "$stress_count" -eq 0 ]
}
