#!/usr/bin/env bats

@test "Should kill running container with default signal" {
    # given (started container)
    docker run -dit --name killing_victim alpine tail -f /dev/null

    # when (trying to kill container)
    run pumba kill /killing_victim

    # then (pumba exited successfully)
    [ $status -eq 0 ]

    # and (container has been killed)
    run docker inspect -f {{.State.Status}} killing_victim
    [[ $output == "exited" ]]

    # cleanup
    docker rm -f killing_victim || true
}