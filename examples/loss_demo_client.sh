#!/bin/sh

echo "# [client container] send datagrams to the server
COPY/PASTE into container ==> 'iperf -c server -u'

"

set -o xtrace

# run client container
echo "running client docker container"
docker run -it --name client --network testnet --rm alpine sh -c "apk add --no-cache iperf; sh"

