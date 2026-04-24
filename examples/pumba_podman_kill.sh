#!/bin/sh

# Kill a random `killme-*` container every 3 s via the Podman runtime.
# Pair with podman_kill_demo.sh which launches 10 target containers.
#
# Requires rootful Podman (lifecycle commands work rootless too, but we keep
# the demos consistent with the netem/stress examples that need rootful).
set -o xtrace

pumba --runtime podman kill --help
printf "Press enter to continue"
read -r _

sudo pumba --runtime podman --log-level=info --interval=3s --random kill 're2:^killme-'
