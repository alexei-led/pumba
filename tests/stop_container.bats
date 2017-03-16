#!/usr/bin/env bats

@test "Should stop running container" {
    # given (started container)
    docker run -dit --name stopping_victim alpine ping localhost

    # when (trying to stop container)
    run pumba stop stopping_victim

    # then (pumba exited successfully)
    [ $status -eq 0 ]

    # and (container has been stopped)
    run bash -c "docker inspect stopping_victim | grep Running"
    [[ $output == *"false"* ]]

    # cleanup
    docker rm -f stopping_victim || true
}