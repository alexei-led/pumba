# Structured result output

Tags: agent · Effort: S

## Why
Today only logs are machine-readable (`--json`). Agents need a result contract, not log scraping.

## Approach
- `--output json|ndjson`: one event per action on stdout, separate from logs.
  `{action, target{id,name,runtime}, params, started, ended, status, error, cleanup{ok,residual}}`
- Emit from `chaos.RunOnContainers` (single fanout choke point).
- Stable exit codes: invalid args, no targets matched, injection failed, cleanup failed (residual state), partial success.
- Error messages state the fix (pattern already used for rootless podman).

## Open questions
- Event schema versioning.
- Interaction with `--interval` (stream events vs final summary).
