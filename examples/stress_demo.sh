#!/bin/sh

set -o xtrace

trap ctrl_c INT

function ctrl_c() {
  docker rm -f testme
}

docker run -d --rm --name testme alpine tail -f /dev/null
docker stats
