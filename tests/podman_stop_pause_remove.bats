#!/usr/bin/env bats

# Stop / pause / remove lifecycle tests for the podman runtime, mirroring
# tests/containerd_stop_pause_remove.bats. Covers graceful stop, pause+unpause,
# rm on running/stopped containers, restart-after-stop, --limit variants for
# stop/pause/rm, and regex-based rm.
#
# Prerequisites:
#   * rootful podman (not strictly required by these operations, but the whole
#     podman suite assumes rootful — see setup()).
#   * `bats`, `podman`, `pumba` on PATH.
#
# Run locally:
#   * macOS: podman machine ssh sudo bats tests/podman_stop_pause_remove.bats
#   * Linux: sudo bats tests/podman_stop_pause_remove.bats

load test_helper

setup() {
    require_podman
    podman_rm_force pdm_victim pdm_victim_1 pdm_victim_2
}

teardown() {
    sudo pkill -f "pumba.*pdm_victim" 2>/dev/null || true
    kill %1 2>/dev/null || true
    podman_rm_force pdm_victim pdm_victim_1 pdm_victim_2
}

# ── STOP ────────────────────────────────────────────────────────────────────

@test "Should stop running container via podman runtime" {
    podman run -d --name pdm_victim alpine top
    full_id=$(podman inspect --format="{{.Id}}" pdm_victim)

    [ "$(podman inspect -f '{{.State.Status}}' pdm_victim)" = "running" ]

    run pumba --runtime podman --log-level debug stop $full_id
    assert_success

    wait_for 5 "podman inspect -f '{{.State.Status}}' pdm_victim | grep -q exited" "container to be stopped"
    [ "$(podman inspect -f '{{.State.Status}}' pdm_victim)" = "exited" ]
}

@test "Should stop container with custom timeout via podman runtime" {
    podman run -d --name pdm_victim alpine top

    run pumba --runtime podman --log-level debug stop --time 2 pdm_victim
    assert_success

    wait_for 5 "podman inspect -f '{{.State.Status}}' pdm_victim | grep -q exited" "container to be stopped"
    [ "$(podman inspect -f '{{.State.Status}}' pdm_victim)" = "exited" ]
}

@test "Should stop and restart container via podman runtime" {
    podman run -d --name pdm_victim alpine top

    local start_time
    start_time=$(podman inspect -f '{{.State.StartedAt}}' pdm_victim)

    pumba --runtime podman --log-level debug stop --restart --duration 5s pdm_victim &
    PUMBA_PID=$!

    wait_for 10 "podman inspect -f '{{.State.Status}}' pdm_victim | grep -q exited" "container to be stopped"

    wait_for 15 "podman inspect -f '{{.State.Status}}' pdm_victim | grep -q running" "container to be restarted"

    local new_start_time
    new_start_time=$(podman inspect -f '{{.State.StartedAt}}' pdm_victim)
    [ "$start_time" != "$new_start_time" ]

    wait $PUMBA_PID 2>/dev/null || true
}

@test "Should stop and restart with custom grace period via podman runtime" {
    podman run -d --name pdm_victim alpine top

    local start_time
    start_time=$(podman inspect -f '{{.State.StartedAt}}' pdm_victim)

    pumba --runtime podman --log-level debug stop --restart --duration 3s --time 2 pdm_victim &
    PUMBA_PID=$!

    wait_for 10 "podman inspect -f '{{.State.Status}}' pdm_victim | grep -q exited" "container to be stopped"

    wait_for 15 "podman inspect -f '{{.State.Status}}' pdm_victim | grep -q running" "container to be restarted"

    local new_start_time
    new_start_time=$(podman inspect -f '{{.State.StartedAt}}' pdm_victim)
    [ "$start_time" != "$new_start_time" ]

    wait $PUMBA_PID 2>/dev/null || true
}

# ── PAUSE / UNPAUSE ─────────────────────────────────────────────────────────

@test "Should pause and unpause container via podman runtime" {
    podman run -d --name pdm_victim alpine top

    [ "$(podman inspect -f '{{.State.Status}}' pdm_victim)" = "running" ]

    run pumba --runtime podman --log-level debug pause --duration 3s pdm_victim
    assert_success

    # After pumba exits the container should be running again (unpaused)
    wait_for 5 "podman inspect -f '{{.State.Status}}' pdm_victim | grep -q running" "container to be running after unpause"
    [ "$(podman inspect -f '{{.State.Status}}' pdm_victim)" = "running" ]
}

# ── REMOVE (running) ────────────────────────────────────────────────────────

@test "Should remove running container via podman runtime" {
    podman run -d --name pdm_victim alpine top
    full_id=$(podman inspect --format="{{.Id}}" pdm_victim)

    [ "$(podman inspect -f '{{.State.Status}}' pdm_victim)" = "running" ]

    run pumba --runtime podman --log-level debug rm $full_id
    assert_success

    wait_for 5 "! podman inspect pdm_victim >/dev/null 2>&1" "container to be removed"
}

# ── RESTART (skip stopped) ─────────────────────────────────────────────────

@test "Should skip stopped containers during restart via podman runtime" {
    podman run -d --name pdm_victim alpine top
    full_id=$(podman inspect --format="{{.Id}}" pdm_victim)
    podman stop pdm_victim >/dev/null
    wait_for 5 "podman inspect -f '{{.State.Status}}' pdm_victim | grep -q exited" "container to stop"

    # Pumba restart on a stopped container should succeed (skip it)
    run pumba --runtime podman --log-level debug restart $full_id
    assert_success

    # Container should still be exited (pumba skipped it)
    [ "$(podman inspect -f '{{.State.Status}}' pdm_victim)" = "exited" ]
}

# ── REMOVE (stopped) ───────────────────────────────────────────────────────

@test "Should remove stopped container via podman runtime" {
    podman run -d --name pdm_victim alpine top
    full_id=$(podman inspect --format="{{.Id}}" pdm_victim)
    podman stop pdm_victim >/dev/null
    wait_for 5 "podman inspect -f '{{.State.Status}}' pdm_victim | grep -q exited" "container to stop"

    run pumba --runtime podman --log-level debug rm $full_id
    assert_success

    wait_for 5 "! podman inspect pdm_victim >/dev/null 2>&1" "container to be removed"
}

# ── LIMIT ──────────────────────────────────────────────────────────────────

@test "Should respect --limit when stopping containers via podman runtime" {
    podman run -d --name pdm_victim_1 alpine top
    podman run -d --name pdm_victim_2 alpine top
    sleep 1

    [ "$(podman inspect -f '{{.State.Status}}' pdm_victim_1)" = "running" ]
    [ "$(podman inspect -f '{{.State.Status}}' pdm_victim_2)" = "running" ]

    run pumba --runtime podman --log-level debug stop --limit 1 "re2:pdm_victim_.*"
    assert_success

    sleep 2
    local running=0
    podman inspect -f '{{.State.Status}}' pdm_victim_1 2>/dev/null | grep -q running && running=$((running+1))
    podman inspect -f '{{.State.Status}}' pdm_victim_2 2>/dev/null | grep -q running && running=$((running+1))
    echo "Running containers after limit=1 stop: $running"
    [ "$running" -eq 1 ]
}

@test "Should respect --limit when pausing containers via podman runtime" {
    podman run -d --name pdm_victim_1 alpine top
    podman run -d --name pdm_victim_2 alpine top
    sleep 1

    pumba --runtime podman --log-level debug pause --limit 1 --duration 5s "re2:pdm_victim_.*" &
    PUMBA_PID=$!

    sleep 2
    local paused=0
    podman inspect -f '{{.State.Status}}' pdm_victim_1 2>/dev/null | grep -q paused && paused=$((paused+1))
    podman inspect -f '{{.State.Status}}' pdm_victim_2 2>/dev/null | grep -q paused && paused=$((paused+1))
    echo "Paused containers after limit=1: $paused"
    [ "$paused" -eq 1 ]

    kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true
}

@test "Should respect --limit when removing containers via podman runtime" {
    podman run -d --name pdm_victim_1 alpine top
    podman run -d --name pdm_victim_2 alpine top
    sleep 1

    run pumba --runtime podman --log-level debug rm --limit 1 "re2:pdm_victim_.*"
    assert_success

    sleep 2
    local remaining=0
    podman inspect pdm_victim_1 >/dev/null 2>&1 && remaining=$((remaining+1))
    podman inspect pdm_victim_2 >/dev/null 2>&1 && remaining=$((remaining+1))
    echo "Remaining containers after limit=1 rm: $remaining"
    [ "$remaining" -eq 1 ]
}

# ── REMOVE (flags: --force, --volumes) ─────────────────────────────────────

@test "Should force-remove running container with --force via podman runtime" {
    podman run -d --name pdm_victim alpine top
    [ "$(podman inspect -f '{{.State.Status}}' pdm_victim)" = "running" ]

    run pumba --runtime podman --log-level debug rm --force pdm_victim
    assert_success

    wait_for 5 "! podman inspect pdm_victim >/dev/null 2>&1" "container to be removed"
}

@test "Should remove container with --volumes flag via podman runtime" {
    podman volume create pdm_vol >/dev/null
    podman run -d --name pdm_victim -v pdm_vol:/data alpine top

    run pumba --runtime podman --log-level debug rm --volumes pdm_victim
    assert_success

    wait_for 5 "! podman inspect pdm_victim >/dev/null 2>&1" "container to be removed"
    podman volume rm -f pdm_vol >/dev/null 2>&1 || true
}

# ── RESTART (--limit) ──────────────────────────────────────────────────────

@test "Should respect --limit when restarting containers via podman runtime" {
    podman run -d --name pdm_victim_1 alpine top
    podman run -d --name pdm_victim_2 alpine top
    sleep 1

    start_1=$(podman inspect -f '{{.State.StartedAt}}' pdm_victim_1)
    start_2=$(podman inspect -f '{{.State.StartedAt}}' pdm_victim_2)

    run pumba --runtime podman --log-level debug restart --limit 1 "re2:pdm_victim_.*"
    assert_success

    sleep 2
    new_1=$(podman inspect -f '{{.State.StartedAt}}' pdm_victim_1)
    new_2=$(podman inspect -f '{{.State.StartedAt}}' pdm_victim_2)

    local restarted=0
    [ "$start_1" != "$new_1" ] && restarted=$((restarted+1))
    [ "$start_2" != "$new_2" ] && restarted=$((restarted+1))
    echo "Restarted containers after limit=1: $restarted"
    [ "$restarted" -eq 1 ]
}

# ── REMOVE (regex, multi-container) ────────────────────────────────────────

@test "Should remove containers matched by regex via podman runtime" {
    podman run -d --name pdm_victim_1 alpine top
    podman run -d --name pdm_victim_2 alpine top
    sleep 1

    run pumba --runtime podman --log-level debug rm "re2:pdm_victim_.*"
    assert_success

    sleep 2
    for name in pdm_victim_1 pdm_victim_2; do
        if podman inspect "$name" >/dev/null 2>&1; then
            state=$(podman inspect -f '{{.State.Status}}' "$name")
            echo "Container $name: $state"
            [[ "$state" =~ exited ]]
        else
            echo "Container $name: removed"
        fi
    done
}
