# Machine-readable plan, schema, MCP server

Tags: agent · Effort: M (plan/schema S, MCP M)

## Why
Agents must discover the CLI and preview effects without parsing `--help` or running the fault.

## Approach
- `--dry-run --output json`: resolved targets plus exact tc/iptables argv that would run (dry-run today never prints rules; see #363).
- `pumba schema`/`describe`: all commands, flags, types, defaults, constraints, examples as JSON Schema.
- `pumba mcp`: tools `list_targets`, `plan`, `inject`, `status`, `revert`, `verify_clean`. Mandatory duration and blast-radius cap on every call.

## Open questions
- stdio only, or also HTTP.
- How much safety policy lives in the server vs the caller.

Depends on: [fault-lifecycle-run-id](fault-lifecycle-run-id.md). Builds on result events ([docs/agents.md](../agents.md)).
