#!/usr/bin/env bats

@test "Should stop running container" {
    # given (started container)
    run docker run -d --name stopping_victim alpine tail -f /dev/null

    # when (trying to stop container)
    run pumba stop /stopping_victim

    # then (pumba exited successfully)
    [ $status -eq 0 ]

    # and (container has been stopped)
    run docker inspect -f {{.State.Status}} stopping_victim
    [ $output = "exited" ]

    # cleanup
    docker rm -f stopping_victim || true
}

@test "Should stop running container with SIGTERM" {
    # given (started container)
    run docker run -d --name stopping_victim alpine sh -c "trap : TERM INT; tail -f /dev/null & wait"

    # when (trying to stop container)
    run pumba stop /stopping_victim

    # then (pumba exited successfully)
    [ $status -eq 0 ]

    # and (container has been stopped)
    run docker inspect -f {{.State.Status}} stopping_victim
    [ $output = "exited" ]

    # cleanup
    docker rm -f stopping_victim || true
}

@test "Should (re)start a previously stopped container" {
    # given (stopped container)
    run docker run -d --name starting_victim alpine sh -c "trap : TERM INT; tail -f /dev/null & wait"
    run docker inspect -f {{.State.Status}} starting_victim
    [ $output = "running" ]

    # when (trying to stop container)
    run pumba stop --restart=true --duration=5s /starting_victim
    # then (pumba exited successfully)
    [ "$status" -eq 0 ]

    # and (container has been (re)started)
    run docker inspect -f {{.State.Status}} starting_victim
    [ $output = "running" ]

    # cleanup
    docker rm -f starting_victim || true
}