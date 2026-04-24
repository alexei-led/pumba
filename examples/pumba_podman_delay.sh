#!/bin/sh

# Add 3000 ms (±20%) delay to the `ping-target` container's egress traffic
# via the Podman runtime. Pair with a long-running container, e.g.:
#
#   podman run -d --name ping-target --privileged nicolaka/netshoot \
#     sh -c "ping 1.1.1.1"
#
# Requires rootful Podman — netem needs CAP_NET_ADMIN on the target's netns.
set -o xtrace

sudo pumba --runtime podman --log-level=info \
	netem --duration 60s --tc-image ghcr.io/alexei-led/pumba-alpine-nettools:latest \
	delay --time 3000 --jitter 20 ping-target
