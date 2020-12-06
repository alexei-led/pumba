#!/bin/sh

set -o xtrace

trap ctrl_c INT

function ctrl_c() {
     kubectl delete pod test-stress
}

kubectl run test-stress --restart=Never --image alpine -- sh -c "tail -f /dev/null"

watch -n 2 "kubectl top pods"