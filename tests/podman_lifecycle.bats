#!/usr/bin/env bats

# Lifecycle tests for the podman runtime (kill, stop, start, restart, pause,
# unpause, remove, exec) mirroring tests/containerd_lifecycle.bats.
#
# Prerequisites:
#   * rootful podman (netem/iptables/stress also need root, but lifecycle works
#     rootless too — we keep these tests runtime-agnostic)
#   * `bats`, `podman`, `pumba` on PATH
#
# Run locally:
#   * macOS: podman machine ssh sudo bats tests/podman_lifecycle.bats
#   * Linux: sudo bats tests/podman_lifecycle.bats

load test_helper

setup() {
    require_podman
    podman_rm_force podman_victim podman_victim_2
}

teardown() {
    podman_rm_force podman_victim podman_victim_2
}

@test "Should kill container via podman runtime using ID" {
    cid=$(podman run -d --rm --name podman_victim alpine top)
    full_id=$(podman inspect --format="{{.Id}}" $cid)

    [ "$(podman inspect -f '{{.State.Status}}' $full_id)" = "running" ]

    run pumba --runtime podman --log-level debug kill $full_id
    assert_success

    wait_for 5 "! podman inspect $full_id >/dev/null 2>&1" "container to be removed"
}

@test "Should restart running container via podman runtime using name" {
    podman run -d --name podman_victim alpine top
    full_id=$(podman inspect --format="{{.Id}}" podman_victim)

    [ "$(podman inspect -f '{{.State.Status}}' $full_id)" = "running" ]

    run pumba --runtime podman --log-level debug restart podman_victim

    if [ $status -ne 0 ]; then
        echo "Pumba restart output: $output"
    fi
    assert_success

    sleep 2
    status_out=$(podman inspect -f '{{.State.Status}}' podman_victim)
    echo "Status after restart: $status_out"
    [ "$status_out" = "running" ]
}

@test "Should exec command in container via podman runtime" {
    podman run -d --name podman_victim alpine top
    full_id=$(podman inspect --format="{{.Id}}" podman_victim)

    [ "$(podman inspect -f '{{.State.Status}}' $full_id)" = "running" ]

    run pumba --runtime podman --log-level debug exec --command "touch" --args "/tmp/pumba_exec" $full_id

    if [ $status -ne 0 ]; then
        echo "Pumba exec output: $output"
    fi
    assert_success

    run podman exec $full_id ls /tmp/pumba_exec
    assert_success
}

@test "Should kill container with SIGTERM via podman runtime" {
    cid=$(podman run -d --name podman_victim alpine top)
    full_id=$(podman inspect --format="{{.Id}}" $cid)

    [ "$(podman inspect -f '{{.State.Status}}' $full_id)" = "running" ]

    run pumba --runtime podman --log-level debug kill --signal SIGTERM $full_id
    assert_success

    wait_for 5 "podman inspect -f '{{.State.Status}}' $full_id | grep -q exited" "container to exit"
    [ "$(podman inspect -f '{{.State.Status}}' $full_id)" = "exited" ]
}

@test "Should respect --limit when killing containers via podman runtime" {
    podman run -d --name podman_victim alpine top
    podman run -d --name podman_victim_2 alpine top
    sleep 1

    [ "$(podman inspect -f '{{.State.Status}}' podman_victim)" = "running" ]
    [ "$(podman inspect -f '{{.State.Status}}' podman_victim_2)" = "running" ]

    run pumba --runtime podman --log-level debug kill --limit 1 "re2:podman_victim"
    assert_success

    sleep 2
    local running=0
    podman inspect -f '{{.State.Status}}' podman_victim 2>/dev/null | grep -q running && running=$((running+1))
    podman inspect -f '{{.State.Status}}' podman_victim_2 2>/dev/null | grep -q running && running=$((running+1))
    echo "Running containers after limit=1 kill: $running"
    [ "$running" -eq 1 ]
}

@test "Should execute command with multiple arguments via podman runtime" {
    cid=$(podman run -d --name podman_victim alpine top)
    full_id=$(podman inspect --format="{{.Id}}" $cid)

    [ "$(podman inspect -f '{{.State.Status}}' $full_id)" = "running" ]

    run pumba --runtime podman --log-level debug exec --command "sh" --args "-c" --args "echo hello > /tmp/multi_args" $full_id

    if [ $status -ne 0 ]; then
        echo "Pumba exec output: $output"
    fi
    assert_success

    run podman exec $full_id cat /tmp/multi_args
    assert_success
    assert_output --partial "hello"
}

@test "Should respect exec --limit parameter via podman runtime" {
    podman run -d --name podman_victim alpine top
    podman run -d --name podman_victim_2 alpine top
    sleep 1

    run pumba --runtime podman --log-level debug exec --limit 1 --command "touch" --args "/tmp/exec_limit_test" "re2:podman_victim"
    assert_success

    local found=0
    podman exec podman_victim ls /tmp/exec_limit_test 2>/dev/null && found=$((found+1))
    podman exec podman_victim_2 ls /tmp/exec_limit_test 2>/dev/null && found=$((found+1))
    echo "Containers with exec file: $found"
    [ "$found" -eq 1 ]
}

@test "Should handle exec on non-existent container via podman runtime" {
    run pumba --runtime podman --log-level debug exec --command "echo" --args "test" nonexistent_container_12345
    assert_success
}

@test "Should restart container with timeout via podman runtime" {
    podman run -d --name podman_victim alpine top

    [ "$(podman inspect -f '{{.State.Status}}' podman_victim)" = "running" ]

    run pumba --runtime podman --log-level debug restart --timeout 3s podman_victim

    if [ $status -ne 0 ]; then
        echo "Pumba restart output: $output"
    fi
    assert_success

    sleep 2
    status_out=$(podman inspect -f '{{.State.Status}}' podman_victim)
    echo "Status after restart with timeout: $status_out"
    [ "$status_out" = "running" ]
}
