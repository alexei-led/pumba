#!/bin/sh

set -o xtrace

trap ctrl_c INT

function ctrl_c() {
  for i in $(seq 1 10); do docker rm -f killme-${i}; done
}
 
# run test containers
for i in $(seq 1 10); do docker run -d --name killme-${i} alpine tail -f /dev/null; done

watch docker ps -a
