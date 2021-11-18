#!/usr/bin/env bats

@test "Should stress running container" {
    # given (started container)
    docker run -d --name stress_victim alpine tail -f /dev/null

    # check number of stress-ng processes
    stress_count=$(docker top stress_victim -o pid,command | grep stress-ng | wc -l)
    [ "$stress_count" -eq 0 ]

    # pull stress-ng image
    docker pull alexeiled/stress-ng:latest-ubuntu

    # when (trying to stress container)
    run pumba stress --duration=20s --stressors="--cpu 1 --timeout 10s" stress_victim &
    # wait a bit for stress test to start (download image and inject side container)
    sleep 10

    # check number of stress-ng processes
    stress_count=$(docker top stress_victim -o pid,command | grep stress-ng | wc -l)
    [ "$stress_count" -eq 2 ]

    # sleep till stress test is completed
    sleep 10

    # check number of stress-ng processes
    stress_count=$(docker top stress_victim -o pid,command | grep stress-ng | wc -l)
    [ "$stress_count" -eq 0 ]
}

teardown() {
    docker rm -f stress_victim || true
}