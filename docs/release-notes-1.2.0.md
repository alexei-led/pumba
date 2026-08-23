# Pumba 1.2.0

## Overview

Pumba 1.2.0 adds safer network emulation and simpler container targeting.
It also adds one command for several network effects.

## Highlights

### Target containers by name

Use a running container name, full ID, or unique ID prefix with `--target`.
Pumba resolves the target at each command run, including each interval run.
Pumba adds all IPv4 addresses from the target container.
Pumba rejects ambiguous selectors and IPv6 targets with a clear error.

### Combine network effects

Use `netem combine` to apply two or more effects in one qdisc.
Use a separate switch for each effect.
Use `--` before the container selectors.

Example:

```bash
pumba netem --duration 5m combine \
  --delay --delay-time 100 \
  --loss --loss-percent 20 \
  -- mydb
```

Existing single-effect commands remain unchanged.

### Protect netem qdiscs

Pumba now owns its netem qdisc with the reserved `504d:` handle.
Pumba refuses to change a foreign or stale qdisc.
Pumba rolls back partial setup.
Pumba removes the owned qdisc only after it checks the complete topology.
The Docker, containerd, and Podman paths use the same ownership rules.

### Improve containerd test support

The repository now generates containerd SDK mocks with Mockery v3.
External SDK mocks stay in test-only files.
Mock generation is reproducible.

## Compatibility notes

- Netem target selectors support IPv4 addresses and IPv4 CIDR values.
- Netem does not support IPv6 target selectors.
- `netem combine` requires at least two effect switches.
- Empty `re2:` selectors keep their match-all behavior when combined with exact names.

## Dependencies and CI

- Update containerd to 2.2.5.
- Update gRPC and `golang.org/x/text` security fixes.
- Update the Alpine base image and Bats.
- Add file locks for Podman integration tests.

## Contributors

Thanks to all contributors who helped with this release:

- [Alexei Ledenev](https://github.com/alexei-led)
- [Dependabot](https://github.com/dependabot)

Pumba is an open-source project. Contributions, bug reports, and test reports help improve it.
