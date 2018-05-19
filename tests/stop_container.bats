#!/usr/bin/env bats

@test "Should stop running container" {
    # given (started container)
    docker run -d --name stopping_victim alpine tail -f /dev/null

    # when (trying to stop container)
    run pumba stop /stopping_victim

    # then (pumba exited successfully)
    [ $status -eq 0 ]

    # and (container has been stopped)
    run docker inspect -f {{.State.Status}} stopping_victim
    [[ $output == "exited" ]]

    # cleanup
    docker rm -f stopping_victim || true
}

@test "Should (re)start a previously stopped container" {
    # given (stopped container)
    docker run -d --name starting_victim alpine tail -f /dev/null

    # when (trying to stop container)
    run pumba stop --restart=true --duration=5s /starting_victim

    # then (pumba exited successfully)
    [ $status -eq 0 ]

    # and (container has been (re)started)
    run docker inspect -f {{.State.Status}} starting_victim
    [[ $output == "running" ]]

    # cleanup
    docker rm -f starting_victim || true
}