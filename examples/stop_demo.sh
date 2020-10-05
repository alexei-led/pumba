#!/bin/sh

set -o xtrace

trap ctrl_c INT

function ctrl_c() {
  for i in $(seq 1 10); do docker kill stopme-${i}; done
}

for i in $(seq 1 10); do docker run -d --name=stopme-${i} alpine tail -f /dev/null; done

watch docker ps -a

