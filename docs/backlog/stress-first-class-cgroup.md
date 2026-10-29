# Stress: cgroup limits and telemetry

Tags: stress · Effort: M

## Approach
- Temporarily lower `memory.max` / `cpu.max` on the target: simulates noisy neighbor or throttled pod without burning CPU.
- Ramp memory until PSI `memory.pressure` reaches X.
- Report target cgroup stats before/during/after in structured output.
- Fix Podman 4.9 cg-inject ENOENT race (retry in stress-ng sibling repo), then move `tests/skip_ci/podman_stress_inject_cgroup.bats` back.

## Open questions
- Restoring original limits if pumba dies (see [fault-lifecycle-run-id](fault-lifecycle-run-id.md)).
