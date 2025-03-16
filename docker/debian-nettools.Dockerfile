FROM debian:stable-slim

LABEL com.gaiaadm.pumba.skip=true
LABEL org.opencontainers.image.source="https://github.com/alexei-led/pumba"
LABEL org.opencontainers.image.description="Debian-based image with iproute2 (tc) and iptables for Pumba network chaos testing"

# Update package lists and install iproute2 and iptables
RUN apt-get update && \
    apt-get install -y \
    iproute2 \
    iptables && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Use neutral entrypoint to keep container running
ENTRYPOINT ["tail", "-f", "/dev/null"]