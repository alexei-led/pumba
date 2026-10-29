# Host chaos

Tags: host, safety · Effort: L

## Why
Pumba is container-scoped. Host faults need a separate, explicit trust level.

## Approach
- Opt-in `--allow-host`, separate command group.
- Host netem/iptables on a physical interface; sysctl perturbation (conntrack size); fill `/var/lib/docker`.
- Container daemon restart (dockerd/containerd) to test live-restore: highest value, rarely tested.
- Out of scope: host reboot, kernel panic (other tools).

## Open questions
- Host clock skew: too dangerous to ship?
