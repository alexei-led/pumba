#!/usr/bin/env bats

load test_helper

setup() {
    docker rm -f ctr_flag_victim >/dev/null 2>&1 || true
    docker rm -f ctr_flag_victim_1 ctr_flag_victim_2 >/dev/null 2>&1 || true
}

teardown() {
    docker rm -f ctr_flag_victim >/dev/null 2>&1 || true
    docker rm -f ctr_flag_victim_1 ctr_flag_victim_2 >/dev/null 2>&1 || true
}

@test "Should run containerd kill in dry-run mode" {
    cid=$(docker run -d --name ctr_flag_victim alpine top)
    full_id=$(docker inspect --format="{{.Id}}" $cid)

    run pumba --dry-run --log-level debug kill $full_id
    [ $status -eq 0 ]

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
    [ $status -eq 0 ]

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
    [ $status -eq 0 ]

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
    [ $status -eq 0 ]

    sleep 2
    # Only victim_1 (chaos=true) should be killed
    wait_for 5 "docker inspect -f '{{.State.Status}}' ctr_flag_victim_1 | grep -q exited" "labeled container to exit"
    [ "$(docker inspect -f '{{.State.Status}}' ctr_flag_victim_2)" = "running" ]
}
