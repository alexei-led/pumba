#!/bin/sh

set -o xtrace

docker run -it --rm --name testme alpine sh -c "while true; do date +%T; sleep 1; done"
