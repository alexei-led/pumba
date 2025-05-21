#!/bin/sh

set -o xtrace

# create namespace if not exists
ctr ns create demo 2>/dev/null || true

# pull and run a simple ping container using containerd
ctr -n demo image pull docker.io/library/alpine:latest
ctr -n demo run -d --name ping docker.io/library/alpine:latest ping 1.1.1.1
