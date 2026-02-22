#!/usr/bin/env bats

load test_helper

setup() {
    docker rm -f containerd_victim >/dev/null 2>&1 || true
}

teardown() {
    docker rm -f containerd_victim >/dev/null 2>&1 || true
}

@test "Should kill container via containerd runtime using ID" {
    cid=$(docker run -d --rm --name containerd_victim alpine top)
    full_id=$(docker inspect --format="{{.Id}}" $cid)
    
    [ "$(docker inspect -f '{{.State.Status}}' $full_id)" = "running" ]
    
    run pumba --log-level debug kill $full_id
    [ $status -eq 0 ]
    
    wait_for 5 "! docker inspect $full_id >/dev/null 2>&1" "container to be removed"
}

@test "Should restart running container via containerd runtime using ID" {
    # Start container via ctr (pure containerd)
    # This avoids Docker bundle issues
    sudo ctr i pull docker.io/library/alpine:latest >/dev/null 2>&1
    sudo ctr run -d docker.io/library/alpine:latest test-restart-ctr top >/dev/null 2>&1
    
    # Verify running
    [ "$(sudo ctr t ls | grep test-restart-ctr | awk '{print $3}')" = "RUNNING" ]
    
    # RESTART (running -> stop -> start)
    # We use the ID "test-restart-ctr"
    run pumba --log-level debug restart test-restart-ctr
    
    if [ $status -ne 0 ]; then
        echo "Pumba restart output: $output"
    fi
    [ $status -eq 0 ]
    
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
    run pumba --log-level debug exec --command "touch /tmp/pumba_exec" $full_id
    
    if [ $status -ne 0 ]; then
        echo "Pumba exec output: $output"
    fi
    [ $status -eq 0 ]
    
    # Verify file created
    run docker exec $full_id ls /tmp/pumba_exec
    [ $status -eq 0 ]
}
