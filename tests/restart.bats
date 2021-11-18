#!/usr/bin/env bats

@test "Should restart running container" {
    # given (started container)
    docker run -d --name restart_victim alpine tail -f /dev/null

    # capture container started time
    started_time="$(docker inspect -f '{{.State.StartedAt}}' restart_victim)"

    # when (trying to restart container)
    run pumba restart --timeout 3s restart_victim &
    sleep 5

    # then (container has been restarted)
    restarted_time="$(docker inspect -f '{{.State.StartedAt}}' restart_victim)"

    [ "$started_time" != "$restarted_time" ]

    # and (container has been restarted)
    run docker inspect -f {{.State.Status}} restart_victim
    [ $output = "running" ]
}

teardown() {
    docker rm -f restart_victim || true
}