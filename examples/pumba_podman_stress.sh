#!/bin/sh

# Stress-test `stress-target` for 60 s using Podman runtime in child-cgroup
# mode. Pumba launches a short-lived stress-ng sidecar placed in the target's
# cgroup so the load counts against the target's limits. Pair with e.g.:
#
#   podman run -d --name stress-target alpine sleep infinity
#
# Requires rootful Podman.
set -o xtrace

sudo pumba --runtime podman --log-level=info stress \
	--duration 60s \
	--stress-image ghcr.io/alexei-led/stress-ng:latest \
	--stressors "--cpu 4 --timeout 60s" \
	stress-target
