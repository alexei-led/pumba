#!/usr/bin/env bats

@test "Netem Help" {
  run pumba netem --help
  [ $status -eq 0 ]
}

@test "Netem Delay Help" {
  run pumba netem delay --help
  [ $status -eq 0 ]
}

@test "Netem Delay Undefined Duration" {
  run pumba netem delay --time 100
  [ $status -eq 1 ]
  [[ ${lines[0]} =~ "Undefined duration interval" ]]
}

@test "Netem Delay 200ms" {
  run pumba netem --duration 200ms delay --time 100
  [ $status -eq 0 ]
  [[ $output =~ "netem: dealy for containers" ]]
}

@test "Netem Delay 200ms External Image" {
  # start ping container in background
  docker run -dit --name pingtest alpine ping www.example.com
  cid=$(docker ps -q --filter "name=pingtest")
  while [ -z "$cid" ]; do
    sleep 1
    cid=$(docker ps -q --filter "name=pingtest")
  done
  # pull tc image beforehead
  docker pull gaiadocker/iproute2
  run pumba netem --duration 5s --tc-image gaiadocker/iproute2 delay --time 1000 pingtest
  [ $status -eq 0 ]
  [[ $output =~ "Start netem for container $cid" ]]
  [[ $output =~ "Stop netem for container $cid" ]]
  # cleanup
  docker stop pingtest
}
