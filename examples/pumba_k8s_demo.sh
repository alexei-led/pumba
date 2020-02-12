#!/bin/sh

set -o xtrace

trap ctrl_c INT

function ctrl_c() {
     kubectl delete -f ../deploy/pumba_kube.yml
}


kubectl create -f ../deploy/pumba_kube.yml

sleep 10
kubectl logs -l app=pumba --all-containers=true --max-log-requests=6 -f

