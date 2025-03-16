FROM alpine:3.21

LABEL com.gaiaadm.pumba.skip=true
LABEL org.opencontainers.image.source="https://github.com/alexei-led/pumba"
LABEL org.opencontainers.image.description="Alpine-based image with iproute2 (tc) and iptables for Pumba network chaos testing"

# Install required packages
RUN apk --no-cache add iproute2 iptables

# Create symlink needed for tc on Alpine
RUN ln -s /usr/lib/tc /lib/tc

# Use neutral entrypoint to keep container running
ENTRYPOINT ["tail", "-f", "/dev/null"]
