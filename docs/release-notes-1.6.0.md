# Pumba 1.6.0

## Highlights

- Added opt-in `--match-all` for netem and iptables selectors: address/port
  dimensions combine with AND while values within each dimension remain OR.
  Expansion is bounded to 256 combinations and 64 KiB per rendered script;
  over-limit requests fail before injection or rollback.
- Added run-scoped `pumba revert --run-id FILTER` for validated tagged iptables
  rules and labeled netem, iptables, and stress sidecars, including eligible
  orphans. Recovery requires stopping the original writer and reports
  incomplete inventory/namespace coverage as unresolved rather than claiming
  the environment is clean. It does not remove qdiscs, untagged rules, targets,
  volumes, or lifecycle faults.
- Recovery emits additive artifact results/counts for orphan work; it does not
  fabricate target events.
- Added a bounded auth-dependency outage scenario and foreground shell wrapper
  for staging TCP/UDP replies, with AND-scoped address/port selectors and
  dry-run as the default. It demonstrates event-based admission and post-run
  status checks; it does not guarantee fault activation or successful cleanup.
- Updated dependency pins: containerd/api 1.12.0, containerd/v2 2.4.2,
  go-connections 0.8.2, and gRPC 1.85.0.

See [the auth outage rehearsal](auth-outage-rehearsal.md), [user guide](guide.md),
[network chaos guide](network-chaos.md), and [agent event contract](agents.md)
for operation and event details. Runtime and
remote CI validation remain necessary; local offline tests do not establish
runtime recovery success.
