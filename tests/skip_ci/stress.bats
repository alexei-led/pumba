#!/usr/bin/env bats

@test "Should stress running container" {
    # given (started container)
    docker run -d --name stress_victim alpine tail -f /dev/null

    # pull stress-ng image
    run docker pull ghcr.io/alexei-led/stress-ng:latest

    # when (trying to stress container)
    run pumba --log-level=debug stress --duration=20s --stressors="--cpu 1 --timeout 10s" stress_victim
    [ $status -eq 0 ]
    [[ $output =~ "stress testing container" ]]

    # check number of stress-ng processes
    stress_count=$(docker top stress_victim -o pid,command | grep stress-ng | wc -l)
    [ "$stress_count" -eq 0 ]
}

teardown() {
    docker rm -f stress_victim || true
}