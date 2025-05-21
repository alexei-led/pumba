#!/bin/sh

set -o xtrace

pumba --runtime containerd \
  --containerd-address /run/containerd/containerd.sock \
  --containerd-namespace demo \
  --log-level=info --interval=20s \
  netem --duration=10s delay --time=300 ping
