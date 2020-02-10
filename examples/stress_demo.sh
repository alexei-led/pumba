#!/bin/sh

set -o xtrace

docker run -d --rm --name testme alpine tail -f /dev/null
docker stats
docker kill testme