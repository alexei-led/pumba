#!/usr/bin/env bats

load test_helper

setup() {
    docker rm -f containerd_victim containerd_victim_2 >/dev/null 2>&1 || true
    sudo ctr t kill -s SIGKILL test-restart-ctr >/dev/null 2>&1 || true
    sudo ctr c rm test-restart-ctr >/dev/null 2>&1 || true
}

teardown() {
    docker rm -f containerd_victim containerd_victim_2 >/dev/null 2>&1 || true
    sudo ctr t kill -s SIGKILL test-restart-ctr >/dev/null 2>&1 || true
    sudo ctr c rm test-restart-ctr >/dev/null 2>&1 || true
}

@test "Should kill container via containerd runtime using ID" {
    cid=$(docker run -d --rm --name containerd_victim alpine top)
    full_id=$(docker inspect --format="{{.Id}}" $cid)
    
    [ "$(docker inspect -f '{{.State.Status}}' $full_id)" = "running" ]
    
    run pumba --log-level debug kill $full_id
    assert_success
    
    wait_for 5 "! docker inspect $full_id >/dev/null 2>&1" "container to be removed"
}

@test "Should restart running container via containerd runtime using ID" {
    require_containerd
    # Start container via ctr (pure containerd)
    # This avoids Docker bundle issues
    ctr_pull_image default docker.io/library/alpine:latest
    sudo ctr run -d docker.io/library/alpine:latest test-restart-ctr top >/dev/null 2>&1
    
    # Verify running
    [ "$(sudo ctr t ls | grep test-restart-ctr | awk '{print $3}')" = "RUNNING" ]
    
    # RESTART (running -> stop -> start)
    # We use the ID "test-restart-ctr"
    run pumba --runtime containerd --containerd-namespace default --log-level debug restart test-restart-ctr
    
    if [ $status -ne 0 ]; then
        echo "Pumba restart output: $output"
    fi
    assert_success
    
    # It should still be running
    sleep 2
    status_out=$(sudo ctr t ls | grep test-restart-ctr | awk '{print $3}')
    echo "Status after restart: $status_out"
    [ "$status_out" = "RUNNING" ]
    
    # Cleanup
    sudo ctr t kill -s SIGKILL test-restart-ctr >/dev/null 2>&1
    sudo ctr c rm test-restart-ctr >/dev/null 2>&1
}

@test "Should exec command in container via containerd runtime" {
    cid=$(docker run -d --name containerd_victim alpine top)
    full_id=$(docker inspect --format="{{.Id}}" $cid)
    
    [ "$(docker inspect -f '{{.State.Status}}' $full_id)" = "running" ]
    
    # EXEC
    # We use a simple command that definitely exists
    run pumba --log-level debug exec --command "touch" --args "/tmp/pumba_exec" $full_id
    
    if [ $status -ne 0 ]; then
        echo "Pumba exec output: $output"
    fi
    assert_success
    
    # Verify file created
    run docker exec $full_id ls /tmp/pumba_exec
    assert_success
}

@test "Should kill container with SIGTERM via containerd runtime" {
    cid=$(docker run -d --name containerd_victim alpine top)
    full_id=$(docker inspect --format="{{.Id}}" $cid)

    [ "$(docker inspect -f '{{.State.Status}}' $full_id)" = "running" ]

    run pumba --log-level debug kill --signal SIGTERM $full_id
    assert_success

    wait_for 5 "docker inspect -f '{{.State.Status}}' $full_id | grep -q exited" "container to exit"
    [ "$(docker inspect -f '{{.State.Status}}' $full_id)" = "exited" ]
}

@test "Should respect --limit when killing containers via containerd runtime" {
    docker run -d --name containerd_victim alpine top
    docker run -d --name containerd_victim_2 alpine top
    sleep 1

    id1=$(docker inspect --format="{{.Id}}" containerd_victim)
    id2=$(docker inspect --format="{{.Id}}" containerd_victim_2)

    [ "$(docker inspect -f '{{.State.Status}}' containerd_victim)" = "running" ]
    [ "$(docker inspect -f '{{.State.Status}}' containerd_victim_2)" = "running" ]

    # Kill with limit=1 — only one should be killed
    run pumba --log-level debug kill --limit 1 "re2:containerd_victim"
    assert_success

    sleep 2
    local running=0
    docker inspect -f '{{.State.Status}}' containerd_victim 2>/dev/null | grep -q running && running=$((running+1))
    docker inspect -f '{{.State.Status}}' containerd_victim_2 2>/dev/null | grep -q running && running=$((running+1))
    echo "Running containers after limit=1 kill: $running"
    [ "$running" -eq 1 ]

    docker rm -f containerd_victim_2 2>/dev/null || true
}

@test "Should execute command with multiple arguments via containerd runtime" {
    cid=$(docker run -d --name containerd_victim alpine top)
    full_id=$(docker inspect --format="{{.Id}}" $cid)

    [ "$(docker inspect -f '{{.State.Status}}' $full_id)" = "running" ]

    # Use repeated --args flags for multiple arguments
    run pumba --log-level debug exec --command "sh" --args "-c" --args "echo hello > /tmp/multi_args" $full_id

    if [ $status -ne 0 ]; then
        echo "Pumba exec output: $output"
    fi
    assert_success

    # Verify file was created with expected content
    run docker exec $full_id cat /tmp/multi_args
    assert_success
    assert_output --partial "hello"
}

@test "Should respect exec --limit parameter via containerd runtime" {
    docker run -d --name containerd_victim alpine top
    docker run -d --name containerd_victim_2 alpine top
    sleep 1

    # Exec with limit=1 targeting both via regex
    run pumba --log-level debug exec --limit 1 --command "touch" --args "/tmp/exec_limit_test" "re2:containerd_victim"
    assert_success

    # Count which containers have the file
    local found=0
    docker exec containerd_victim ls /tmp/exec_limit_test 2>/dev/null && found=$((found+1))
    docker exec containerd_victim_2 ls /tmp/exec_limit_test 2>/dev/null && found=$((found+1))
    echo "Containers with exec file: $found"
    [ "$found" -eq 1 ]

    docker rm -f containerd_victim_2 2>/dev/null || true
}

@test "Should handle exec on non-existent container via containerd runtime" {
    run pumba --log-level debug exec --command "echo" --args "test" nonexistent_container_12345
    # Pumba should handle gracefully — exit 0 (no matching containers found)
    assert_success
}

@test "Should restart container with timeout via containerd runtime" {
    require_containerd
    ctr_pull_image default docker.io/library/alpine:latest
    sudo ctr run -d docker.io/library/alpine:latest test-restart-ctr top >/dev/null 2>&1

    [ "$(sudo ctr t ls | grep test-restart-ctr | awk '{print $3}')" = "RUNNING" ]

    run pumba --runtime containerd --containerd-namespace default --log-level debug restart --timeout 3s test-restart-ctr

    if [ $status -ne 0 ]; then
        echo "Pumba restart output: $output"
    fi
    assert_success

    sleep 2
    status_out=$(sudo ctr t ls | grep test-restart-ctr | awk '{print $3}')
    echo "Status after restart with timeout: $status_out"
    [ "$status_out" = "RUNNING" ]
}
