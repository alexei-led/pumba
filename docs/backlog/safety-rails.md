# Safety rails

Tags: safety · Effort: S

## Approach
- Global `--max-targets`.
- Deny-list label (`pumba.skip=true`; sidecar skip label exists already).
- `--duration` required for every reversible fault (already for netem/iptables/stress; see #360 for validation gaps).
- `--confirm` prompt when matching more than N containers.

## Open questions
- Defaults vs opt-in (compat with existing scripts).
