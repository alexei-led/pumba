# Fault lifecycle: revert and handle refactor

Tags: arch, agent, safety · Effort: M

## Why

Fault cleanup is owned by each action. `pumba revert --run-id FILTER` now recovers a bounded set of tagged rules and labeled sidecars. Qdiscs, legacy untagged rules, lifecycle undo, and a uniform action handle remain separate work.

Shipped: `--run-id`, result events (`--output ndjson`), artifact tagging, `pumba status [--check]`, and bounded run-scoped `pumba revert --run-id FILTER` for tagged iptables rules and labeled netem/iptables/stress sidecars (including eligible orphans). See [docs/agents.md](../agents.md) for coverage limits and safety requirements.

Still deferred: the generic Inject/Handle lifecycle refactor, MCP/OTel subscribers, qdisc recovery, untagged-rule recovery, and lifecycle-operation undo.

## Shipped

- [x] `pumba revert --run-id FILTER`: bounded recovery of eligible tagged rules and sidecars; qdisc and lifecycle recovery are excluded.

## Deferred design work

- Uniform `Inject(ctx) (Handle, error)`; `Handle.Revert(ctx)`, `Handle.Verify(ctx)`.
- More event subscribers on the existing `chaos.EventSink`: MCP, OTel.

## Deferred work

- Reverting an unattributed qdisc remains out of scope: tc has no comment field to identify a run. `pumba status --check` can report suspicious/owned topology; manual recovery is separate.
- Pre-1.5 iptables rules have no Pumba run comment and are not eligible for run-scoped recovery.
- Containerd stress by exec (`--stress-image ""`) leaves no container and no discoverable artifact.
- A generic Inject/Handle API and additional event subscribers remain separate architectural work.
