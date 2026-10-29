# Ingress shaping, partitions, TCP faults, profiles

Tags: network · Effort: M each

## Why
netem shapes egress only, so inbound replies cannot be selected. Common real faults are missing.

## Approach
- IFB device: redirect ingress to `ifb`, apply netem there.
- `pumba partition A B`: bidirectional drop between two container sets.
- TCP faults: RST (`REJECT --reject-with tcp-reset`), `connlimit`, SYN-only drop.
- Presets: 3G, satellite, lossy wifi.
- IPv6 `--target` support (not supported today per #363).

## Open questions
- IFB availability in Colima/podman machine kernels.
