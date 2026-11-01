# CLI schema and MCP server

Tags: agent · Effort: M (schema S, MCP M)

## Why
Agents must discover the CLI and preview effects without parsing `--help` or running the fault.

Shipped: netem/iptables dry-run argv plans in text mode and `--output ndjson`
([contract](../agents.md#dry-run-plans), #363). Lifecycle/stress argv expansion is not planned here.

## Remaining approach
- `pumba schema`/`describe`: all commands, flags, types, defaults, constraints, examples as JSON Schema.
- `pumba mcp`: tools `list_targets`, `plan`, `inject`, `status`, `revert`, `verify_clean`. Mandatory duration and blast-radius cap on every call.

## Open questions
- stdio only, or also HTTP.
- How much safety policy lives in the server vs the caller.

Depends on: [fault-lifecycle-run-id](fault-lifecycle-run-id.md). Builds on result events ([docs/agents.md](../agents.md)).
