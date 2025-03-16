#!/bin/bash

# This script demonstrates using both netem and iptables commands together
# to create more complex and realistic network chaos scenarios.

# Common image for both commands
NETTOOLS_IMAGE="ghcr.io/alexei-led/pumba-alpine-nettools:latest"

# Pull the image in advance (optional)
docker pull $NETTOOLS_IMAGE

# Create asymmetric network degradation scenario:
# 1. Add delay to outgoing traffic (slow uploads) with netem
# 2. Drop incoming packets (simulate packet loss) with iptables

# Add delay to outgoing traffic (simulates slow upload)
../bin/pumba --log-level info netem \
  --duration 5m \
  --interface eth0 \
  --tc-image $NETTOOLS_IMAGE \
  delay \
  --time 500 \
  --jitter 50 \
  combined-server &

NETEM_PID=$!

# Wait a moment to ensure the first command starts properly
sleep 2

# Drop incoming packets (simulates packet loss)
../bin/pumba --log-level info iptables \
  --duration 5m \
  --protocol tcp \
  --dst-port 80 \
  --iptables-image $NETTOOLS_IMAGE \
  loss \
  --probability 0.3 \
  combined-server &

IPTABLES_PID=$!

# Wait for both processes to complete
wait $NETEM_PID
wait $IPTABLES_PID

echo "Network chaos complete!"