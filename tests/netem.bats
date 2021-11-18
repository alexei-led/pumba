#!/usr/bin/env bats

@test "Should display netem help" {
  run pumba netem --help
  [ $status -eq 0 ]
}

@test "Should display netem delay help" {
  run pumba netem delay --help
  [ $status -eq 0 ]
}

@test "Should fail when Duration is unset for netem delay" {
  run pumba netem delay --time 100
  [ $status -eq 1 ]
  [[ ${lines[0]} =~ "unset or invalid duration value" ]]
}

@test "Should delay egress traffic from container" {
  run pumba netem --duration 200ms delay --time 100 test
  [ $status -eq 0 ]
  [[ $output =~ "no containers found" ]]
}

@test "Should delay egress traffic from container with external tc image" {
  # start ping container in background
  docker run -dit --name pingtest alpine ping 1.1.1.1
  cid=$(docker ps -q --filter "name=pingtest")
  while [ -z "$cid" ]; do
    sleep 1
    cid=$(docker ps -q --filter "name=pingtest")
  done
  # pull tc image beforehead
  docker pull gaiadocker/iproute2
  run pumba -l=info netem --duration 5s --tc-image gaiadocker/iproute2 delay --time 1000 pingtest
  [ $status -eq 0 ]
  [[ $output =~ "running netem on container" ]]
  [[ $output =~ "stopping netem on container" ]]
}

teardown() {
    docker rm -f pingtest || true
}
