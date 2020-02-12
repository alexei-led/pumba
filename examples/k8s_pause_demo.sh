#!/bin/sh

set -o xtrace

kubectl run test-1 --rm -it --restart=Never --image alpine -- sh -c "while true; do date +%T; sleep 1; done"