#!/usr/bin/env bats

load test_helper

bats_require_minimum_version 1.5.0

# Pinned to 0.20.01 — first release that ships /cg-inject alongside /stress-ng
# (see https://github.com/alexei-led/stress-ng/releases/tag/0.20.01).
# Floating :latest can leave stale 0.20.00 cached locally, which lacks cg-inject
# and silently skipped the inject-cgroup test. Bump deliberately when a newer
# tag is published and verified.
STRESS_IMAGE="ghcr.io/alexei-led/stress-ng:0.20.01"

setup() {
    STRESS_TARGET_ID=""
    require_containerd
    # Record only the fixture created here; do not remove a pre-existing name.
    STRESS_TARGET_ID=$(docker run -d --name stress_victim alpine sh -c "apk add --no-cache stress-ng >/dev/null 2>&1 && sleep infinity")
    wait_for 30 "docker exec $STRESS_TARGET_ID which stress-ng >/dev/null 2>&1" "stress-ng to be installed"
}

teardown() {
    [ -n "${STRESS_TARGET_ID:-}" ] || return 0
    sudo pkill -f "^pumba .*$STRESS_TARGET_ID" 2>/dev/null || true
    # Other runs' stress sidecars must survive this suite's teardown.
    for sc in $(sudo ctr -n moby c ls -q "labels.\"com.gaiaadm.pumba.target\"==$STRESS_TARGET_ID" 2>/dev/null); do
        sudo ctr -n moby t rm -f "$sc" >/dev/null 2>&1 || true
        sudo ctr -n moby c rm "$sc" >/dev/null 2>&1 || true
        sudo ctr -n moby snapshots rm "${sc}-snapshot" >/dev/null 2>&1 || true
    done
    docker rm -f "$STRESS_TARGET_ID" >/dev/null 2>&1 || true
}

assert_direct_stress_clean() {
    assert_container_running "$STRESS_TARGET_ID"
    run docker exec "$STRESS_TARGET_ID" ps -o comm
    assert_success
    refute_output --partial "stress-ng"
}

# ── Direct exec mode (existing behaviour) ────────────────────────────────

@test "Should handle stress on non-existent container via containerd runtime" {
    run sudo pumba --runtime containerd --containerd-namespace moby --log-level debug stress --stress-image "" --duration 5s --stressors="--cpu 1 --cpu-method loop --timeout 2s" nonexistent_container_12345
    # Pumba should handle gracefully — exit 0 (no matching containers found)
    assert_success
}

@test "Should run stress in dry-run mode via containerd runtime" {
    full_id=$(docker inspect --format="{{.Id}}" stress_victim)

    run sudo pumba --runtime containerd --containerd-namespace moby --dry-run --log-level debug stress --stress-image "" --duration 5s --stressors="--cpu 1 --cpu-method loop --timeout 2s" "$full_id"
    assert_success

    [ "$(docker inspect -f '{{.State.Status}}' stress_victim)" = "running" ]
}

@test "Should apply stress (CPU) via containerd runtime" {
    # Get full container ID (Docker-created containers live in moby namespace)
    full_id=$(docker inspect --format="{{.Id}}" stress_victim)

    run sudo pumba --runtime containerd --containerd-namespace moby --log-level debug stress --stress-image "" --duration 10s --pull-image=false --stressors="--cpu 1 --cpu-method loop --timeout 3s" "$full_id"

    echo "Pumba stress output: $output"

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
        stress --duration 5s --pull-image=false --stress-image ${STRESS_IMAGE} --stressors="--cpu 1 --cpu-method loop" $full_id
    assert_success

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
        stress --duration 10s --pull-image=false --stress-image ${STRESS_IMAGE} --stressors="--cpu 1 --cpu-method loop --timeout 3s" $full_id

    echo "Pumba output: $output"

    assert_success
    assert_output --partial "resolved target cgroup for stress sidecar"

    run sudo ctr -n moby c ls -q
    refute_output --partial "pumba-stress-"

    assert_container_running stress_victim
}

# ── Inject-cgroup mode (--inject-cgroup --stress-image) ──────────────────

@test "Should run stress via inject-cgroup sidecar on containerd" {
    require_containerd
    full_id=$(docker inspect --format="{{.Id}}" stress_victim)

    ctr_pull_image moby ${STRESS_IMAGE}
    docker image inspect ${STRESS_IMAGE} >/dev/null 2>&1 || \
        fail "stress image ${STRESS_IMAGE} not available after pull"
    # ${STRESS_IMAGE} is pinned to a tag that ships /cg-inject (0.20.01+);
    # fail loudly if a future bump regresses on this contract.
    check_id=$(docker create --entrypoint /stress-ng ${STRESS_IMAGE} --help)
    docker export "$check_id" | tar -tf - | grep -qx "cg-inject" || {
        docker rm -f "$check_id" >/dev/null 2>&1 || true
        fail "stress image ${STRESS_IMAGE} missing /cg-inject — pin to a tag that ships it"
    }
    docker rm -f "$check_id" >/dev/null 2>&1 || true

    # Run inject-cgroup stress — pumba creates a sidecar with /cg-inject as entrypoint,
    # host cgroupns, /sys/fs/cgroup mount, and --cgroup-path pointing to the target
    run sudo pumba --runtime containerd --containerd-namespace moby --log-level debug \
        stress --duration 10s --inject-cgroup --pull-image=false --stress-image ${STRESS_IMAGE} --stressors="--cpu 1 --cpu-method loop --timeout 3s" $full_id

    echo "Pumba output: $output"

    assert_success
    assert_output --partial "resolved target cgroup for stress sidecar"

    run sudo ctr -n moby c ls -q
    refute_output --partial "pumba-stress-"

    assert_container_running stress_victim
}

@test "Should reap duration-limited direct stress without stopping its container" {
    run --separate-stderr sudo pumba --runtime containerd --containerd-namespace moby \
        --run-id ctr-stress-direct-duration -o ndjson stress --stress-image "" --duration 1s \
        --stressors="--cpu 1 --cpu-method loop --timeout 30s" "$STRESS_TARGET_ID"
    assert_success
    assert_events '[.[] | select(.type == "target.result")] | length == 1 and (.[0] | .status == "ok" and .cleanup.ok == true)'
    assert_run_end '.status == "ok"'
    assert_direct_stress_clean
}

@test "Should reap direct stress after SIGTERM without stopping its container" {
    local events="$BATS_TEST_TMPDIR/direct-stress.ndjson"
    local logs="$BATS_TEST_TMPDIR/direct-stress.stderr"
    # Bats owns these writable logs; sudo is only for runtime access.
    # shellcheck disable=SC2024
    sudo pumba --runtime containerd --containerd-namespace moby \
        --run-id ctr-stress-direct-signal -o ndjson stress --stress-image "" --duration 30s \
        --stressors="--cpu 1 --cpu-method loop --timeout 30s" "$STRESS_TARGET_ID" >"$events" 2>"$logs" 3>&- &
    local process=$!
    wait_for 10 "jq -es 'any(.[]; .type == \"target.applied\")' '$events' >/dev/null" "direct stress applied"
    wait_for 10 "docker exec $STRESS_TARGET_ID ps -o comm | grep -q stress-ng" "direct stress worker running"
    sudo pkill -TERM -f "^pumba .*--run-id ctr-stress-direct-signal.*$STRESS_TARGET_ID"
    local result=0
    wait "$process" || result=$?
    [ "$result" -eq 0 ]
    output=$(cat "$events")
    assert_events '[.[] | select(.type == "target.result")] | length == 1 and (.[0] | .status == "ok" and .cleanup.ok == true)'
    assert_run_end '.status == "aborted" and .error.code == "canceled"'
    assert_direct_stress_clean
}
