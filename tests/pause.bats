#!/usr/bin/env bats

@test "Should pause running container" {
    # given (started container)
    docker run -d --name pausing_victim alpine tail -f /dev/null

    # when (trying to pause container)
    run pumba pause --duration 3s pausing_victim &
    sleep 2

    # then (container has been paused)
    run docker inspect -f {{.State.Status}} pausing_victim
    [ $output == "paused" ]

    # and (container has been resumed)
    sleep 4
    run docker inspect -f {{.State.Status}} pausing_victim
    [ $output = "running" ]
}

teardown() {
    docker rm -f pausing_victim || true
}