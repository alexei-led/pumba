# DNS and L7 (HTTP/gRPC) faults

Tags: network, chaos · Effort: M (DNS) / L (L7)

## Why
DNS is a top real-world outage cause. L7 faults are the most-requested higher-level chaos.

## Approach
- DNS: sidecar CoreDNS/dnsmasq plus DNAT of :53 in the target netns; NXDOMAIN, slow answers, wrong answers per name.
- L7: transparent proxy sidecar (Envoy fault filter or small Go proxy) with abort %, delay, status codes per path.

## Open questions
- Sidecar image ownership and size.
- Transparent redirect vs explicit proxy config.
