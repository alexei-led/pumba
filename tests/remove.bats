#!/usr/bin/env bats

# Load the test helper
load test_helper

setup() {
    # Clean any leftover containers from previous test runs
    cleanup_containers "rm_victim"
}

teardown() {
    # Clean up containers after each test
    cleanup_containers "rm_victim"
}

@test "Should remove running docker container" {
    # Given a running container
    create_test_container "rm_victim"
    
    # Verify container is running
    run docker inspect -f {{.State.Status}} rm_victim
    assert_output "running"
    
    # When removing the container with pumba
    run pumba rm rm_victim
    
    # Then pumba should exit successfully
    assert_success
    
    # And container should be removed
    run docker ps -a --filter "name=rm_victim" --format "{{.Names}}"
    echo "Remaining containers: $output"
    assert_output ""
}

@test "Should remove stopped docker container" {
    # Given a container that has been stopped
    create_test_container "rm_victim"
    docker stop rm_victim
    
    # Verify container is stopped but still exists
    run docker inspect -f {{.State.Status}} rm_victim
    assert_output "exited"
    
    # When removing the stopped container with pumba
    run pumba rm rm_victim
    
    # Then pumba should exit successfully
    assert_success
    
    # And container should be removed
    run docker ps -a --filter "name=rm_victim" --format "{{.Names}}"
    echo "Remaining containers: $output"
    assert_output ""
}

@test "Should remove stopped container matched by regex" {
    # Given a stopped container
    create_test_container "rm_victim"
    docker stop rm_victim
    
    # Verify container is stopped
    run docker inspect -f {{.State.Status}} rm_victim
    assert_output "exited"
    
    # When removing with regex pattern
    run pumba rm "re2:rm_vict.*"
    
    # Then pumba should exit successfully
    assert_success
    
    # And container should be removed
    run docker ps -a --filter "name=rm_victim" --format "{{.Names}}"
    echo "Remaining containers: $output"
    assert_output ""
}

@test "Should force-remove running container with --force" {
    create_test_container "rm_victim"
    assert_container_running "rm_victim"

    run pumba rm --force rm_victim
    assert_success

    run docker ps -a --filter "name=rm_victim" --format "{{.Names}}"
    assert_output ""
}

@test "Should remove container with --volumes flag" {
    docker run -d --name rm_victim -v /data alpine tail -f /dev/null
    sleep 1
    assert_container_running "rm_victim"

    run pumba rm --volumes rm_victim
    assert_success

    run docker ps -a --filter "name=rm_victim" --format "{{.Names}}"
    assert_output ""
}

@test "Should respect --limit when removing containers" {
    docker run -d --name rm_victim_1 alpine tail -f /dev/null
    docker run -d --name rm_victim_2 alpine tail -f /dev/null
    sleep 1

    assert_container_running "rm_victim_1"
    assert_container_running "rm_victim_2"

    run pumba rm --limit 1 "re2:rm_victim_.*"
    assert_success

    sleep 2
    local remaining=0
    docker inspect rm_victim_1 &>/dev/null && remaining=$((remaining+1))
    docker inspect rm_victim_2 &>/dev/null && remaining=$((remaining+1))
    echo "Remaining containers after limit=1 rm: $remaining"
    [ "$remaining" -eq 1 ]

    docker rm -f rm_victim_1 rm_victim_2 2>/dev/null || true
}
