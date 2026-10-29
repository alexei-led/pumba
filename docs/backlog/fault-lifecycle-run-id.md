# Fault lifecycle, run ID, status/revert/verify-clean

Tags: arch, agent, safety · Effort: M

## Why
Cleanup is embedded per action. Crashed runs (including agent crashes) leave qdiscs, iptables rules or sidecars with no way to find or remove them. Foundation for JSON output, MCP and rollback.

## Approach
- Run ID per invocation.
- Tag artifacts: `iptables -m comment --comment pumba:<run-id>`; `pumba.run-id` label on sidecars.
- Uniform `Inject(ctx) (Handle, error)`; `Handle.Revert(ctx)`, `Handle.Verify(ctx)`.
- Commands: `pumba status`, `pumba revert [--run-id]`, `pumba verify-clean`.
- Typed event bus: Slack/webhook, JSON output, MCP and OTel become subscribers.

## Open questions
- Tagging tc qdiscs (handles have no comment field).
- Where state lives when pumba has no persistent store (derive from target netns).
