#!/usr/bin/env bats

load test_helper

setup() {
    docker rm -f ctr_victim >/dev/null 2>&1 || true
    sudo ctr -n moby t kill -s SIGKILL ctr-pure-victim >/dev/null 2>&1 || true
    sudo ctr -n moby c rm ctr-pure-victim >/dev/null 2>&1 || true
}

teardown() {
    docker rm -f ctr_victim >/dev/null 2>&1 || true
    sudo ctr -n moby t kill -s SIGKILL ctr-pure-victim >/dev/null 2>&1 || true
    sudo ctr -n moby c rm ctr-pure-victim >/dev/null 2>&1 || true
}

# ── STOP ────────────────────────────────────────────────────────────────────

@test "Should stop running container via containerd runtime" {
    docker run -d --name ctr_victim alpine top
    full_id=$(docker inspect --format="{{.Id}}" ctr_victim)

    [ "$(docker inspect -f '{{.State.Status}}' ctr_victim)" = "running" ]

    run pumba --log-level debug stop $full_id
    [ $status -eq 0 ]

    wait_for 5 "docker inspect -f '{{.State.Status}}' ctr_victim | grep -q exited" "container to be stopped"
    [ "$(docker inspect -f '{{.State.Status}}' ctr_victim)" = "exited" ]
}

@test "Should stop container with custom timeout via containerd runtime" {
    docker run -d --name ctr_victim alpine top
    full_id=$(docker inspect --format="{{.Id}}" ctr_victim)

    run pumba --log-level debug stop --time 2 $full_id
    [ $status -eq 0 ]

    wait_for 5 "docker inspect -f '{{.State.Status}}' ctr_victim | grep -q exited" "container to be stopped"
    [ "$(docker inspect -f '{{.State.Status}}' ctr_victim)" = "exited" ]
}

# ── PAUSE / UNPAUSE ─────────────────────────────────────────────────────────

@test "Should pause and unpause container via containerd runtime" {
    docker run -d --name ctr_victim alpine top
    full_id=$(docker inspect --format="{{.Id}}" ctr_victim)

    [ "$(docker inspect -f '{{.State.Status}}' ctr_victim)" = "running" ]

    # Pause for 3 seconds (pumba applies pause then unpauses after duration)
    run pumba --log-level debug pause --duration 3s $full_id
    [ $status -eq 0 ]

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
    [ $status -eq 0 ]

    # Container process should be killed (exited)
    wait_for 5 "docker inspect -f '{{.State.Status}}' ctr_victim 2>&1 | grep -qE 'exited|No such'" "container to be killed"
}

@test "Should fully remove pure containerd container" {
    # Create container directly in containerd (not Docker-managed)
    sudo ctr -n moby i pull docker.io/library/alpine:latest >/dev/null 2>&1
    sudo ctr -n moby run -d docker.io/library/alpine:latest ctr-pure-victim top

    # Verify it exists
    run sudo ctr -n moby c info ctr-pure-victim
    [ $status -eq 0 ]

    # Remove via pumba
    run pumba --log-level debug rm ctr-pure-victim
    [ $status -eq 0 ]

    # Should be fully gone from containerd
    run sudo ctr -n moby c info ctr-pure-victim
    [ $status -ne 0 ]
}
