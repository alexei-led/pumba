#!/usr/bin/env bats

load test_helper

setup() {
    docker rm -f ctr_victim ctr_victim_1 ctr_victim_2 >/dev/null 2>&1 || true
    sudo ctr -n moby t kill -s SIGKILL ctr-pure-victim >/dev/null 2>&1 || true
    sudo ctr -n moby c rm ctr-pure-victim >/dev/null 2>&1 || true
    sudo ctr t kill -s SIGKILL ctr-stopped-victim >/dev/null 2>&1 || true
    sudo ctr c rm ctr-stopped-victim >/dev/null 2>&1 || true
}

teardown() {
    sudo pkill -f "pumba.*ctr_victim" 2>/dev/null || true
    docker rm -f ctr_victim ctr_victim_1 ctr_victim_2 >/dev/null 2>&1 || true
    sudo ctr -n moby t kill -s SIGKILL ctr-pure-victim >/dev/null 2>&1 || true
    sudo ctr -n moby c rm ctr-pure-victim >/dev/null 2>&1 || true
    sudo ctr t kill -s SIGKILL ctr-stopped-victim >/dev/null 2>&1 || true
    sudo ctr c rm ctr-stopped-victim >/dev/null 2>&1 || true
}

# ── STOP ────────────────────────────────────────────────────────────────────

@test "Should stop running container via containerd runtime" {
    docker run -d --name ctr_victim alpine top
    full_id=$(docker inspect --format="{{.Id}}" ctr_victim)

    [ "$(docker inspect -f '{{.State.Status}}' ctr_victim)" = "running" ]

    run pumba --log-level debug stop $full_id
    assert_success

    wait_for 5 "docker inspect -f '{{.State.Status}}' ctr_victim | grep -q exited" "container to be stopped"
    [ "$(docker inspect -f '{{.State.Status}}' ctr_victim)" = "exited" ]
}

@test "Should stop container with custom timeout via containerd runtime" {
    docker run -d --name ctr_victim alpine top
    full_id=$(docker inspect --format="{{.Id}}" ctr_victim)

    run pumba --log-level debug stop --time 2 $full_id
    assert_success

    wait_for 5 "docker inspect -f '{{.State.Status}}' ctr_victim | grep -q exited" "container to be stopped"
    [ "$(docker inspect -f '{{.State.Status}}' ctr_victim)" = "exited" ]
}

@test "Should stop and restart container via containerd runtime" {
    docker run -d --name ctr_victim alpine top
    full_id=$(docker inspect --format="{{.Id}}" ctr_victim)

    local start_time
    start_time=$(docker inspect -f '{{.State.StartedAt}}' ctr_victim)

    pumba --log-level debug stop --restart --duration 5s $full_id &
    PUMBA_PID=$!

    wait_for 10 "docker inspect -f '{{.State.Status}}' ctr_victim | grep -q exited" "container to be stopped"

    wait_for 15 "docker inspect -f '{{.State.Status}}' ctr_victim | grep -q running" "container to be restarted"

    local new_start_time
    new_start_time=$(docker inspect -f '{{.State.StartedAt}}' ctr_victim)
    [ "$start_time" != "$new_start_time" ]

    wait $PUMBA_PID 2>/dev/null || true
}

@test "Should stop and restart with custom grace period via containerd runtime" {
    docker run -d --name ctr_victim alpine top
    full_id=$(docker inspect --format="{{.Id}}" ctr_victim)

    local start_time
    start_time=$(docker inspect -f '{{.State.StartedAt}}' ctr_victim)

    pumba --log-level debug stop --restart --duration 3s --time 2 $full_id &
    PUMBA_PID=$!

    wait_for 10 "docker inspect -f '{{.State.Status}}' ctr_victim | grep -q exited" "container to be stopped"

    wait_for 15 "docker inspect -f '{{.State.Status}}' ctr_victim | grep -q running" "container to be restarted"

    local new_start_time
    new_start_time=$(docker inspect -f '{{.State.StartedAt}}' ctr_victim)
    [ "$start_time" != "$new_start_time" ]

    wait $PUMBA_PID 2>/dev/null || true
}

# ── PAUSE / UNPAUSE ─────────────────────────────────────────────────────────

@test "Should pause and unpause container via containerd runtime" {
    docker run -d --name ctr_victim alpine top
    full_id=$(docker inspect --format="{{.Id}}" ctr_victim)

    [ "$(docker inspect -f '{{.State.Status}}' ctr_victim)" = "running" ]

    # Pause for 3 seconds (pumba applies pause then unpauses after duration)
    run pumba --log-level debug pause --duration 3s $full_id
    assert_success

    # After pumba exits the container should be running again (unpaused)
    wait_for 5 "docker inspect -f '{{.State.Status}}' ctr_victim | grep -q running" "container to be running after unpause"
    [ "$(docker inspect -f '{{.State.Status}}' ctr_victim)" = "running" ]
}

# ── REMOVE ──────────────────────────────────────────────────────────────────

@test "Should remove running container via containerd runtime (Docker-managed)" {
    docker run -d --name ctr_victim alpine top
    full_id=$(docker inspect --format="{{.Id}}" ctr_victim)

    [ "$(docker inspect -f '{{.State.Status}}' ctr_victim)" = "running" ]

    # For Docker-managed containers: pumba kills the task and removes containerd metadata.
    # Docker daemon still retains its own metadata (container shows as "exited").
    run pumba --log-level debug rm $full_id
    assert_success

    # Container process should be killed (exited)
    wait_for 5 "docker inspect -f '{{.State.Status}}' ctr_victim 2>&1 | grep -qE 'exited|No such'" "container to be killed"
}

@test "Should fully remove pure containerd container" {
    require_containerd
    # Create container directly in containerd (not Docker-managed)
    ctr_pull_image moby docker.io/library/alpine:latest
    sudo ctr -n moby run -d docker.io/library/alpine:latest ctr-pure-victim top

    # Verify it exists
    run sudo ctr -n moby c info ctr-pure-victim
    assert_success

    # Remove via pumba
    run pumba --runtime containerd --containerd-namespace moby --log-level debug rm ctr-pure-victim
    assert_success

    # Should be fully gone from containerd
    run sudo ctr -n moby c info ctr-pure-victim
    assert_failure
}

# ── RESTART (skip stopped) ─────────────────────────────────────────────────

@test "Should skip stopped containers during restart via containerd runtime" {
    # Create and immediately stop a container
    docker run -d --name ctr_victim alpine top
    full_id=$(docker inspect --format="{{.Id}}" ctr_victim)
    docker stop ctr_victim
    wait_for 5 "docker inspect -f '{{.State.Status}}' ctr_victim | grep -q exited" "container to stop"

    # Pumba restart on a stopped container should succeed (skip it)
    run pumba --log-level debug restart $full_id
    assert_success
}

# ── REMOVE (stopped, regex) ───────────────────────────────────────────────

@test "Should remove stopped container via containerd runtime" {
    docker run -d --name ctr_victim alpine top
    full_id=$(docker inspect --format="{{.Id}}" ctr_victim)
    docker stop ctr_victim
    wait_for 5 "docker inspect -f '{{.State.Status}}' ctr_victim | grep -q exited" "container to stop"

    run pumba --log-level debug rm $full_id
    assert_success

    # Container should be gone or at least task killed
    wait_for 5 "docker inspect ctr_victim 2>&1 | grep -qE 'No such|exited'" "container to be removed"
}

# ── LIMIT ──────────────────────────────────────────────────────────────────

@test "Should respect --limit when stopping containers via containerd runtime" {
    docker run -d --name ctr_victim_1 alpine top
    docker run -d --name ctr_victim_2 alpine top
    sleep 1

    [ "$(docker inspect -f '{{.State.Status}}' ctr_victim_1)" = "running" ]
    [ "$(docker inspect -f '{{.State.Status}}' ctr_victim_2)" = "running" ]

    run pumba --log-level debug stop --limit 1 "re2:ctr_victim_.*"
    assert_success

    sleep 2
    local running=0
    docker inspect -f '{{.State.Status}}' ctr_victim_1 2>/dev/null | grep -q running && running=$((running+1))
    docker inspect -f '{{.State.Status}}' ctr_victim_2 2>/dev/null | grep -q running && running=$((running+1))
    echo "Running containers after limit=1 stop: $running"
    [ "$running" -eq 1 ]
}

@test "Should respect --limit when pausing containers via containerd runtime" {
    docker run -d --name ctr_victim_1 alpine tail -f /dev/null
    docker run -d --name ctr_victim_2 alpine tail -f /dev/null
    sleep 1

    [ "$(docker inspect -f '{{.State.Status}}' ctr_victim_1)" = "running" ]
    [ "$(docker inspect -f '{{.State.Status}}' ctr_victim_2)" = "running" ]

    pumba --log-level debug pause --limit 1 --duration 5s "re2:ctr_victim_.*" &
    PUMBA_PID=$!

    sleep 2
    local paused=0
    docker inspect -f '{{.State.Status}}' ctr_victim_1 2>/dev/null | grep -q paused && paused=$((paused+1))
    docker inspect -f '{{.State.Status}}' ctr_victim_2 2>/dev/null | grep -q paused && paused=$((paused+1))
    echo "Paused containers after limit=1: $paused"
    [ "$paused" -eq 1 ]

    kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true
}

@test "Should respect --limit when removing containers via containerd runtime" {
    docker run -d --name ctr_victim_1 alpine top
    docker run -d --name ctr_victim_2 alpine top
    sleep 1

    [ "$(docker inspect -f '{{.State.Status}}' ctr_victim_1)" = "running" ]
    [ "$(docker inspect -f '{{.State.Status}}' ctr_victim_2)" = "running" ]

    run pumba --log-level debug rm --limit 1 "re2:ctr_victim_.*"
    assert_success

    sleep 2
    local remaining=0
    docker inspect ctr_victim_1 &>/dev/null && remaining=$((remaining+1))
    docker inspect ctr_victim_2 &>/dev/null && remaining=$((remaining+1))
    echo "Remaining containers after limit=1 rm: $remaining"
    [ "$remaining" -eq 1 ]
}

# ── REMOVE (stopped, regex) ───────────────────────────────────────────────

@test "Should remove containers matched by regex via containerd runtime" {
    docker run -d --name ctr_victim_1 alpine top
    docker run -d --name ctr_victim_2 alpine top
    sleep 1

    run pumba --log-level debug rm "re2:ctr_victim_.*"
    assert_success

    sleep 2
    # Both should be killed/removed
    for name in ctr_victim_1 ctr_victim_2; do
        state=$(docker inspect -f '{{.State.Status}}' $name 2>&1 || echo "removed")
        echo "Container $name: $state"
        [[ "$state" =~ "exited" ]] || [[ "$state" =~ "removed" ]] || [[ "$state" =~ "No such" ]]
    done
}
