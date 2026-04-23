#!/usr/bin/env bats

# Global-flag tests for the podman runtime (--dry-run, --random, --label,
# --interval, --json, --log-level, --skip-error, regex targeting) mirroring
# tests/containerd_global_flags.bats.
#
# Prerequisites:
#   * rootful or rootless podman (flag semantics don't touch netns/cgroup)
#   * `bats`, `podman`, `pumba` on PATH
#
# Run locally:
#   * macOS: podman machine ssh sudo bats tests/podman_global_flags.bats
#   * Linux: sudo bats tests/podman_global_flags.bats

load test_helper

setup() {
    require_podman
    podman_rm_force pdm_flag_victim pdm_flag_interval pdm_flag_victim_1 pdm_flag_victim_2
    sudo pkill -f "pumba.*pdm_flag" 2>/dev/null || true
}

teardown() {
    sudo pkill -f "pumba.*pdm_flag" 2>/dev/null || true
    podman_rm_force pdm_flag_victim pdm_flag_interval pdm_flag_victim_1 pdm_flag_victim_2
}

@test "Should run podman kill in dry-run mode" {
    cid=$(podman run -d --name pdm_flag_victim alpine top)
    full_id=$(podman inspect --format="{{.Id}}" $cid)

    run pumba --runtime podman --dry-run --log-level debug kill $full_id
    assert_success

    # Container should still be running (dry-run)
    [ "$(podman inspect -f '{{.State.Status}}' pdm_flag_victim)" = "running" ]
}

@test "Should target containers by regex pattern via podman" {
    podman run -d --name pdm_flag_victim_1 alpine top
    podman run -d --name pdm_flag_victim_2 alpine top
    sleep 1

    run pumba --runtime podman --log-level debug kill "re2:pdm_flag_victim_.*"
    assert_success

    wait_for 5 "podman inspect -f '{{.State.Status}}' pdm_flag_victim_1 | grep -q exited" "victim_1 to exit"
    wait_for 5 "podman inspect -f '{{.State.Status}}' pdm_flag_victim_2 | grep -q exited" "victim_2 to exit"
}

@test "Should select random container with --random flag via podman" {
    podman run -d --name pdm_flag_victim_1 alpine top
    podman run -d --name pdm_flag_victim_2 alpine top
    sleep 1

    run pumba --runtime podman --random --log-level debug kill "re2:pdm_flag_victim_.*"
    assert_success

    sleep 2
    local exited=0
    podman inspect -f '{{.State.Status}}' pdm_flag_victim_1 2>/dev/null | grep -q exited && exited=$((exited+1))
    podman inspect -f '{{.State.Status}}' pdm_flag_victim_2 2>/dev/null | grep -q exited && exited=$((exited+1))
    echo "Exited containers: $exited"
    [ "$exited" -eq 1 ]
}

@test "Should support --label filter with podman runtime" {
    podman run -d --name pdm_flag_victim_1 --label chaos=true alpine top
    podman run -d --name pdm_flag_victim_2 --label chaos=false alpine top
    sleep 1

    run pumba --runtime podman --log-level debug --label "chaos=true" kill "re2:pdm_flag_victim_.*"
    assert_success

    sleep 2
    wait_for 5 "podman inspect -f '{{.State.Status}}' pdm_flag_victim_1 | grep -q exited" "labeled container to exit"
    [ "$(podman inspect -f '{{.State.Status}}' pdm_flag_victim_2)" = "running" ]
}

@test "Should only affect containers matching ALL labels via podman runtime" {
    podman run -d --name pdm_flag_victim_1 --label app=web --label env=test alpine top
    podman run -d --name pdm_flag_victim_2 --label app=web alpine top
    sleep 1

    run pumba --runtime podman --log-level debug --label "app=web" --label "env=test" kill "re2:pdm_flag_victim_.*"
    assert_success

    wait_for 5 "podman inspect -f '{{.State.Status}}' pdm_flag_victim_1 | grep -q exited" "both-label container to exit"
    [ "$(podman inspect -f '{{.State.Status}}' pdm_flag_victim_1)" = "exited" ]

    [ "$(podman inspect -f '{{.State.Status}}' pdm_flag_victim_2)" = "running" ]
}

@test "Should run podman kill on interval with --interval flag" {
    podman run -d --name pdm_flag_interval alpine top
    full_id=$(podman inspect --format="{{.Id}}" pdm_flag_interval)

    [ "$(podman inspect -f '{{.State.Status}}' pdm_flag_interval)" = "running" ]

    pumba --runtime podman --interval=2s kill $full_id &
    PUMBA_PID=$!

    wait_for 5 "podman inspect -f '{{.State.Status}}' pdm_flag_interval | grep -q exited" "container to be killed in first interval"
    [ "$(podman inspect -f '{{.State.Status}}' pdm_flag_interval)" = "exited" ]

    podman start pdm_flag_interval
    wait_for 5 "podman inspect -f '{{.State.Status}}' pdm_flag_interval | grep -q running" "container to be running again"

    wait_for 5 "podman inspect -f '{{.State.Status}}' pdm_flag_interval | grep -q exited" "container to be killed in second interval"
    [ "$(podman inspect -f '{{.State.Status}}' pdm_flag_interval)" = "exited" ]

    kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true
}

@test "Should support --json logging parameter via podman runtime" {
    cid=$(podman run -d --name pdm_flag_victim alpine top)
    full_id=$(podman inspect --format="{{.Id}}" $cid)

    run pumba --runtime podman --json --log-level debug kill $full_id
    assert_success

    [[ "$output" =~ "{" ]] || [[ "$output" =~ "\"level\"" ]]
}

@test "Should support --log-level parameter via podman runtime" {
    cid=$(podman run -d --name pdm_flag_victim alpine top)
    full_id=$(podman inspect --format="{{.Id}}" $cid)

    run pumba --runtime podman --log-level info kill $full_id
    assert_success

    refute_output --partial "level=debug"
}

@test "Should support --skip-error parameter via podman runtime" {
    run pumba --runtime podman --skip-error --log-level debug kill nonexistent_skip_err_12345
    assert_success
}

@test "Should handle already-exited container via podman runtime" {
    podman run -d --name pdm_flag_victim alpine top
    full_id=$(podman inspect --format="{{.Id}}" pdm_flag_victim)
    podman stop pdm_flag_victim
    wait_for 5 "podman inspect -f '{{.State.Status}}' pdm_flag_victim | grep -q exited" "container to exit"

    run pumba --runtime podman --log-level debug kill $full_id
    echo "Kill exited container output: $output"
    # Pumba must terminate — graceful (0) or a handled non-zero — without
    # panicking. A panic shows up as a non-zero stack trace; assert no
    # runtime panic leaked.
    refute_output --partial "runtime error"
    refute_output --partial "goroutine"
}

@test "Should accept --podman-socket override" {
    cid=$(podman run -d --name pdm_flag_victim alpine top)
    full_id=$(podman inspect --format="{{.Id}}" $cid)

    # Derive the socket URI podman itself uses. On Linux this is typically
    # /run/podman/podman.sock (rootful) or $XDG_RUNTIME_DIR/podman/podman.sock
    # (rootless). The test skips if we cannot detect it so it is hermetic.
    sock=""
    sock_file=$(mktemp)
    if podman info --format '{{.Host.RemoteSocket.Path}}' >"$sock_file" 2>/dev/null; then
        sock=$(cat "$sock_file")
    fi
    rm -f "$sock_file"
    if [ -z "$sock" ]; then
        skip "could not resolve podman socket path via podman info"
    fi

    run pumba --runtime podman --podman-socket "unix://${sock}" --log-level debug kill $full_id
    assert_success

    wait_for 5 "podman inspect -f '{{.State.Status}}' $full_id | grep -q exited" "container to exit"
}
