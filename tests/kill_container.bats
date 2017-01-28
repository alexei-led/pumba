#!/usr/bin/env bats

@test "Should kill running container with default signal" {
    # given (started container)
    docker run -dit --name killing_victim alpine ping localhost

    # when (trying to kill container)
    run pumba kill killing_victim

    # then (pumba exited successfully)
    [ $status -eq 0 ]

    # and (container has been killed)
    run bash -c "docker inspect killing_victim | grep Running"
    [[ $output == *"false"* ]]

    # cleanup
    docker rm -f killing_victim
}