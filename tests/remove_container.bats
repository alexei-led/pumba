#!/usr/bin/env bats

@test "Should remove running docker container without any parameters" {
    if [ "$SKIP_RM" = true ]; then
      skip "Skip remove container test..."
    fi
    # given (started container)
    docker run -d --name victim alpine tail -f /dev/null

    # when (trying to remove container)
    run pumba rm /victim

    # then (pumba exited successfully)
    [ $status -eq 0 ]

    # and (container has been removed)
    run bash -c "docker ps -a | grep victim"
    [ ! "$output" ]
}
