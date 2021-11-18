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
    [ $output = "exited" ]
}

@test "Should kill running labeled container with default signal" {
    # given (started containers)
    docker run -dit --label test=true --name killing_victim_1 alpine tail -f /dev/null
    docker run -dit --label test=true --name killing_victim_2 alpine tail -f /dev/null
    docker run -dit --label test=false --name killing_victim_3 alpine tail -f /dev/null

    # when (trying to kill container)
    run pumba --label test=true kill "re2:^killing_victim*"

    # then (pumba exited successfully)
    [ $status -eq 0 ]

    # and (container has been killed)
    run docker inspect -f {{.State.Status}} killing_victim_1
    [ $output = "exited" ]
    # and (container has been killed)
    run docker inspect -f {{.State.Status}} killing_victim_2
    [ $output = "exited" ]
    # and (container has not been killed)
    run docker inspect -f {{.State.Status}} killing_victim_3
    [ $output = "running" ]
}

teardown() {
    docker rm -f killing_victim || true
    docker rm -f killing_victim_1 killing_victim_2 killing_victim_3 || true
}