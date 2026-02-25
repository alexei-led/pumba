#!/usr/bin/env bats

load test_helper

setup() {
    docker rm -f ctr_flag_victim ctr_flag_interval >/dev/null 2>&1 || true
    docker rm -f ctr_flag_victim_1 ctr_flag_victim_2 >/dev/null 2>&1 || true
    sudo pkill -f "pumba.*ctr_flag" 2>/dev/null || true
}

teardown() {
    sudo pkill -f "pumba.*ctr_flag" 2>/dev/null || true
    docker rm -f ctr_flag_victim ctr_flag_interval >/dev/null 2>&1 || true
    docker rm -f ctr_flag_victim_1 ctr_flag_victim_2 >/dev/null 2>&1 || true
}

@test "Should run containerd kill in dry-run mode" {
    cid=$(docker run -d --name ctr_flag_victim alpine top)
    full_id=$(docker inspect --format="{{.Id}}" $cid)

    run pumba --dry-run --log-level debug kill $full_id
    assert_success

    # Container should still be running (dry-run)
    [ "$(docker inspect -f '{{.State.Status}}' ctr_flag_victim)" = "running" ]
}

@test "Should target containers by regex pattern via containerd" {
    docker run -d --name ctr_flag_victim_1 alpine top
    docker run -d --name ctr_flag_victim_2 alpine top
    sleep 1

    id1=$(docker inspect --format="{{.Id}}" ctr_flag_victim_1)
    id2=$(docker inspect --format="{{.Id}}" ctr_flag_victim_2)

    # Kill both via regex
    run pumba --log-level debug kill "re2:ctr_flag_victim_.*"
    assert_success

    # Both should be killed
    wait_for 5 "docker inspect -f '{{.State.Status}}' ctr_flag_victim_1 | grep -q exited" "victim_1 to exit"
    wait_for 5 "docker inspect -f '{{.State.Status}}' ctr_flag_victim_2 | grep -q exited" "victim_2 to exit"
}

@test "Should select random container with --random flag via containerd" {
    docker run -d --name ctr_flag_victim_1 alpine top
    docker run -d --name ctr_flag_victim_2 alpine top
    sleep 1

    # Kill one random container matching the regex
    run pumba --random --log-level debug kill "re2:ctr_flag_victim_.*"
    assert_success

    sleep 2
    # Exactly one should be killed (exited), one still running
    local exited=0
    docker inspect -f '{{.State.Status}}' ctr_flag_victim_1 2>/dev/null | grep -q exited && exited=$((exited+1))
    docker inspect -f '{{.State.Status}}' ctr_flag_victim_2 2>/dev/null | grep -q exited && exited=$((exited+1))
    echo "Exited containers: $exited"
    [ "$exited" -eq 1 ]
}

@test "Should support --label filter with containerd runtime" {
    docker run -d --name ctr_flag_victim_1 --label chaos=true alpine top
    docker run -d --name ctr_flag_victim_2 --label chaos=false alpine top
    sleep 1

    # Kill only containers with matching label
    run pumba --log-level debug --label "chaos=true" kill "re2:ctr_flag_victim_.*"
    assert_success

    sleep 2
    # Only victim_1 (chaos=true) should be killed
    wait_for 5 "docker inspect -f '{{.State.Status}}' ctr_flag_victim_1 | grep -q exited" "labeled container to exit"
    [ "$(docker inspect -f '{{.State.Status}}' ctr_flag_victim_2)" = "running" ]
}

@test "Should run containerd kill on interval with --interval flag" {
    docker run -d --name ctr_flag_interval alpine top
    full_id=$(docker inspect --format="{{.Id}}" ctr_flag_interval)

    [ "$(docker inspect -f '{{.State.Status}}' ctr_flag_interval)" = "running" ]

    # Run pumba with interval in background
    pumba --interval=2s kill $full_id &
    PUMBA_PID=$!

    # Wait for first interval to kill the container
    wait_for 5 "docker inspect -f '{{.State.Status}}' ctr_flag_interval | grep -q exited" "container to be killed in first interval"
    [ "$(docker inspect -f '{{.State.Status}}' ctr_flag_interval)" = "exited" ]

    # Restart container and wait for second interval
    docker start ctr_flag_interval
    wait_for 5 "docker inspect -f '{{.State.Status}}' ctr_flag_interval | grep -q running" "container to be running again"

    wait_for 5 "docker inspect -f '{{.State.Status}}' ctr_flag_interval | grep -q exited" "container to be killed in second interval"
    [ "$(docker inspect -f '{{.State.Status}}' ctr_flag_interval)" = "exited" ]

    kill $PUMBA_PID 2>/dev/null || true
    wait $PUMBA_PID 2>/dev/null || true
}

@test "Should support --json logging parameter via containerd runtime" {
    cid=$(docker run -d --name ctr_flag_victim alpine top)
    full_id=$(docker inspect --format="{{.Id}}" $cid)

    run pumba --json --log-level debug kill $full_id
    assert_success

    # Output should contain JSON-formatted log lines
    [[ "$output" =~ "{" ]] || [[ "$output" =~ "\"level\"" ]]
}

@test "Should support --log-level parameter via containerd runtime" {
    cid=$(docker run -d --name ctr_flag_victim alpine top)
    full_id=$(docker inspect --format="{{.Id}}" $cid)

    run pumba --log-level info kill $full_id
    assert_success

    # With info level, debug messages should NOT appear
    refute_output --partial "level=debug"
}

@test "Should support --skip-error parameter via containerd runtime" {
    # Kill a non-existent container with --skip-error — should succeed
    run pumba --skip-error --log-level debug kill nonexistent_skip_err_12345
    assert_success
}

@test "Should handle already-exited container via containerd runtime" {
    docker run -d --name ctr_flag_victim alpine top
    full_id=$(docker inspect --format="{{.Id}}" ctr_flag_victim)
    docker stop ctr_flag_victim
    wait_for 5 "docker inspect -f '{{.State.Status}}' ctr_flag_victim | grep -q exited" "container to exit"

    # Trying to kill an already-exited container
    run pumba --log-level debug kill $full_id
    echo "Kill exited container output: $output"
    # Should handle gracefully (may warn but not crash)
}
