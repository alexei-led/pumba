#!/bin/sh

echo "# [server container] run a UDP Server listening on UDP port 5001
COPY/PASTE into container ==> 'iperf -s -u -i 1'

"

set -o xtrace

# create docker network
echo "creating docker network ..."
docker network create -d bridge testnet

# run server
echo "running server docker container ..."
docker run -it --name server --network testnet --rm alpine sh -c "apk add --no-cache iperf; sh"
