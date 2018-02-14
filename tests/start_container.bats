#!/usr/bin/env bats

@test "Should start a previously stopped container" {
    # given (stopped container)
    docker run -d --name starting_victim alpine tail -f /dev/null
    docker stop starting_victim

    # when (trying to start container)
    run pumba start starting_victim

    # then (pumba exited successfully)
    [ $status -eq 0 ]

    # and (container has been stopped)
    run bash -c "docker inspect starting_victim | grep Running"
    [[ $output == *"true"* ]]

    # cleanup
    docker stop starting_victim
    docker rm -f starting_victim || true
}
