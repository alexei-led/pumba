# Machine-readable plan, MCP server

Tags: agent · Effort: M (plan S, MCP M)

## Why
Agents must preview effects without running the fault, and drive Pumba as tools instead of shelling out.

## Approach
- `--dry-run --output json`: resolved targets plus exact tc/iptables argv that would run (dry-run today never prints rules; see #363).
- `pumba mcp`: tools `list_targets`, `plan`, `inject`, `status`, `revert`, `verify_clean`. Mandatory duration and blast-radius cap on every call. Tool input schemas come from `pumba schema` (`pumba.cli/v1`, shipped; see [docs/agents.md](../agents.md#discovering-the-cli-pumba-schema)).

## Open questions
- stdio only, or also HTTP.
- How much safety policy lives in the server vs the caller.
- Enums and constraints (signals, ranges) in `pumba.cli/v1`, needed for strict tool input schemas.

Depends on: [fault-lifecycle-run-id](fault-lifecycle-run-id.md). Builds on result events and `pumba schema` ([docs/agents.md](../agents.md)).
