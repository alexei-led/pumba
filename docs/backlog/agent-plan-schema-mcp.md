# MCP server

Tags: agent · Effort: M

## Why
Agents must drive Pumba as tools instead of shelling out.

Shipped: netem/iptables dry-run argv plans in text mode and `--output ndjson`
([contract](../agents.md#dry-run-plans), #363), and `pumba schema` CLI discovery
([contract](../agents.md#discovering-the-cli-pumba-schema)). Lifecycle/stress argv expansion is not planned here.

## Remaining approach
- `pumba mcp`: tools `list_targets`, `plan`, `inject`, `status`, `revert`, `verify_clean`. Mandatory duration and blast-radius cap on every call. Tool input schemas come from `pumba schema` (`pumba.cli/v1`).

## Open questions
- stdio only, or also HTTP.
- How much safety policy lives in the server vs the caller.
- Enums and constraints (signals, ranges) in `pumba.cli/v1`, needed for strict tool input schemas.

Depends on: [fault-lifecycle-run-id](fault-lifecycle-run-id.md). Builds on result events, dry-run plans and `pumba schema` ([docs/agents.md](../agents.md)).
