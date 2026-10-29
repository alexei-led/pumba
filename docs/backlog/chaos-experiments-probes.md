# Experiment files, steady-state probes, scenario library

Tags: chaos, agent · Effort: L

## Why
Pumba is a primitive engine; scenarios (e.g. #362 auth outage) live as long docs and shell scripts (#363).

## Approach
- YAML experiments: steps, parallel groups, abort conditions, automatic rollback.
- Probes: `--probe http://...:200 --abort-on-fail` (turns "break it" into "verify it survives").
- Fault-mix / random-walk mode with weighted faults per interval.
- `examples/scenarios/*.yaml`: auth outage, DNS outage, slow DB, partition, noisy neighbor.
- Testcontainers (Go/Java) helper for chaos in integration tests.

## Open questions
- Own schema vs reuse an existing one.
- Probe types beyond HTTP and command.
