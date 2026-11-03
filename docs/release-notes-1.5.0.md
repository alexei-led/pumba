## Summary

Pumba 1.5.0 adds structured results, fault-ownership checks, network-fault previews, CLI discovery, and event delivery for automation.

## Changes

- `--output ndjson` reports versioned run and target events. `--run-id` tags sidecars and iptables rules; `pumba status --check` reports Pumba faults.
- `pumba schema` describes the CLI without a runtime socket.
- Network `--dry-run` previews exact netem/iptables apply/revert commands without injection.
- `--webhook` sends ordered, best-effort events without retries. Events include container identities and action parameters; use HTTPS and a trusted receiver.
- `--max-duration` limits active run time, not total wall time: admitted fault commands and cleanup can finish afterward.
- Stress waits for owned workloads and bounded cleanup before terminal results. Cleanup failures exit `1` unless suppressed by `--skip-error`.

## Upgrade

- Before rebuilding Go integrations, implement `InspectNetns`, `PlanNetemContainer` and `PlanIPTablesContainer` on `container.Client`; otherwise compilation fails.
- Before running iptables faults, provide the comment match (`xt_comment` for legacy, nft comment support otherwise). Injection fails closed without ownership comments.
- Before upgrading automation, update dry-run parsing/timing: all dry-runs stop waiting for `--duration`; network text plans appear on stdout.

Status cannot find pre-1.5.0 untagged iptables rules. It classifies `504d:` qdiscs but never attributes them to a run, even with a run filter.

## Security

Acceptance covers only GO-2026-4887 and GO-2026-4883 for `github.com/docker/docker@v28.5.2+incompatible`; it is **not a vulnerability-free scan**. Affected daemon implementations are absent from the audited Pumba executable. This does not establish installed Docker Engine safety; update Engine to a patched release, especially with authorization or Docker plugins. See [the security applicability report](https://github.com/alexei-led/pumba/blob/1.5.0/docs/security.md).
