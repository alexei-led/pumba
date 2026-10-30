# Fault lifecycle: artifact tagging, status/revert/verify-clean

Tags: arch, agent, safety · Effort: M

## Why
Cleanup is embedded per action. Crashed runs (including agent crashes) leave qdiscs, iptables rules or sidecars with no way to find or remove them. Foundation for MCP and rollback.

Shipped: `--run-id` and result events (`--output ndjson`, see [docs/agents.md](../agents.md)).

## Approach
- Tag artifacts with the run ID: `iptables -m comment --comment pumba:<run-id>`; `pumba.run-id` label on sidecars.
- Uniform `Inject(ctx) (Handle, error)`; `Handle.Revert(ctx)`, `Handle.Verify(ctx)`.
- Commands: `pumba status`, `pumba revert [--run-id]`, `pumba verify-clean`.
- More event subscribers on the existing `chaos.EventSink`: webhook, MCP, OTel.

## Open questions
- Tagging tc qdiscs (handles have no comment field).
- Where state lives when pumba has no persistent store (derive from target netns).
