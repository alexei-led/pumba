#!/usr/bin/env bats

# Load the test helper
load test_helper

setup() {
    # Clean any leftover containers from previous test runs
    cleanup_containers "stopping_victim"
    cleanup_containers "starting_victim"
}

teardown() {
    # Clean up containers after each test
    sudo pkill -f "pumba.*stopping_victim" 2>/dev/null || true
    cleanup_containers "stopping_victim"
    cleanup_containers "starting_victim"
}

@test "Should stop running container" {
    # Given a running container
    create_test_container "stopping_victim"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} stopping_victim
    assert_output "running"
    
    # When stopping the container
    run pumba stop stopping_victim
    
    # Then pumba should exit successfully
    assert_success
    
    # And container should be stopped (status changed to exited)
    wait_for 5 "docker inspect -f '{{.State.Status}}' stopping_victim | grep -q exited" "container to be stopped"
    
    run docker inspect -f {{.State.Status}} stopping_victim
    echo "Container status after stop: $output"
    assert_output "exited"
    
    # Additional verification of container exit code
    run docker inspect -f {{.State.ExitCode}} stopping_victim
    echo "Container exit code: $output"
    [ -n "$output" ] # Any exit code is fine
}

@test "Should stop running container with signal handling" {
    # Given a container
    # Use a simpler command
    create_test_container "stopping_victim" "alpine" "sleep 60"
    
    # Wait for container to be running
    wait_for 5 "docker inspect -f '{{.State.Status}}' stopping_victim | grep -q running" "container to start"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} stopping_victim
    echo "Initial container status: $output"
    assert_output "running"
    
    # When stopping the container
    run pumba stop stopping_victim
    
    # Then pumba should exit successfully 
    assert_success
    
    # And container should be stopped (status changed to exited)
    wait_for 5 "docker inspect -f '{{.State.Status}}' stopping_victim | grep -q exited" "container to be stopped"
    
    run docker inspect -f {{.State.Status}} stopping_victim
    echo "Container status after stop: $output"
    assert_output "exited"
}


@test "Should stop container with custom timeout" {
    # Given a running container
    create_test_container "stopping_victim"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} stopping_victim
    assert_output "running"
    
    # When stopping the container with custom timeout
    run pumba stop --time 5 stopping_victim
    
    # Then pumba should exit successfully
    assert_success
    
    # And container should be stopped (status changed to exited)
    wait_for 5 "docker inspect -f '{{.State.Status}}' stopping_victim | grep -q exited" "container to be stopped"
    
    run docker inspect -f {{.State.Status}} stopping_victim
    echo "Container status after stop with timeout: $output"
    assert_output "exited"
}

@test "Should stop and restart container with --restart flag" {
    create_test_container "stopping_victim" "alpine" "top"
    assert_container_running "stopping_victim"

    local start_time
    start_time=$(docker inspect -f '{{.State.StartedAt}}' stopping_victim)

    pumba stop --restart --duration 5s stopping_victim &
    PUMBA_PID=$!

    wait_for 10 "docker inspect -f '{{.State.Status}}' stopping_victim | grep -q exited" "container to be stopped"
    assert_container_exited "stopping_victim"

    wait_for 15 "docker inspect -f '{{.State.Status}}' stopping_victim | grep -q running" "container to be restarted"
    assert_container_running "stopping_victim"

    local new_start_time
    new_start_time=$(docker inspect -f '{{.State.StartedAt}}' stopping_victim)
    [ "$start_time" != "$new_start_time" ]

    wait $PUMBA_PID 2>/dev/null || true
}

@test "Should stop and restart container with custom grace period" {
    create_test_container "stopping_victim" "alpine" "top"
    assert_container_running "stopping_victim"

    local start_time
    start_time=$(docker inspect -f '{{.State.StartedAt}}' stopping_victim)

    pumba stop --restart --duration 3s --time 2 stopping_victim &
    PUMBA_PID=$!

    wait_for 10 "docker inspect -f '{{.State.Status}}' stopping_victim | grep -q exited" "container to be stopped"
    assert_container_exited "stopping_victim"

    wait_for 15 "docker inspect -f '{{.State.Status}}' stopping_victim | grep -q running" "container to be restarted"
    assert_container_running "stopping_victim"

    local new_start_time
    new_start_time=$(docker inspect -f '{{.State.StartedAt}}' stopping_victim)
    [ "$start_time" != "$new_start_time" ]

    wait $PUMBA_PID 2>/dev/null || true
}

@test "Should respect --limit when stopping containers" {
    docker run -d --name stopping_victim_1 alpine top
    docker run -d --name stopping_victim_2 alpine top
    sleep 1

    assert_container_running "stopping_victim_1"
    assert_container_running "stopping_victim_2"

    run pumba stop --limit 1 "re2:stopping_victim_.*"
    assert_success

    sleep 2
    local running=0
    docker inspect -f '{{.State.Status}}' stopping_victim_1 2>/dev/null | grep -q running && running=$((running+1))
    docker inspect -f '{{.State.Status}}' stopping_victim_2 2>/dev/null | grep -q running && running=$((running+1))
    echo "Running containers after limit=1 stop: $running"
    [ "$running" -eq 1 ]

    docker rm -f stopping_victim_1 stopping_victim_2 2>/dev/null || true
}
