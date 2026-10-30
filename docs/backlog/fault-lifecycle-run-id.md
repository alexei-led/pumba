# Fault lifecycle: revert and handle refactor

Tags: arch, agent, safety · Effort: M

## Why
Cleanup is embedded per action. `pumba status --check` finds what a crashed run left behind (tagged iptables rules, `504d:` qdiscs, labeled sidecars), but removing it is still manual. Foundation for MCP and rollback.

Shipped: `--run-id`, result events (`--output ndjson`), artifact tagging and `pumba status [--check]` (see [docs/agents.md](../agents.md)).

## Approach
- Uniform `Inject(ctx) (Handle, error)`; `Handle.Revert(ctx)`, `Handle.Verify(ctx)`.
- `pumba revert [--run-id]`: remove what `status` finds, using the same tc/iptables builders as inject.
- More event subscribers on the existing `chaos.EventSink`: MCP, OTel.

## Open questions
- Reverting an unattributed qdisc: tc has no comment field, so a `504d:` qdisc cannot be tied to a run. Revert it only without a run filter, or only when no other Pumba run targets the container?
- Pre-1.5 iptables rules carry no comment; match them by spec, or leave them manual?
- Containerd sidecar IDs are a per-process counter (`pumba-sidecar-<n>`, `pumba-stress-<n>`); a leftover sidecar or snapshot makes the next run collide on the name. Add a pid or random suffix.
- Containerd stress by exec (`--stress-image ""`) leaves no container; nothing to find or revert.
