#!/usr/bin/env bats

@test "Should remove running docker container without any parameters" {
    # given (started container)
    docker run -dit --name victim alpine ping localhost

    # when (trying to remove container)
    run pumba rm victim

    # then (pumba exited successfully)
    [ $status -eq 0 ]

    # and (container has been removed)
    run bash -c "docker ps -a | grep victim"
    [ ! "$output" ]
}
