# urfave/cli v1 to v3

Tags: stack, arch · Effort: L

## Approach
`pkg/chaos/cliflags` adapter limits the change mostly to `cmd/` and the `*/cmd/` builders: add `v3.go`, switch wiring in `cmd/main.go`. Candidate for a 2.0 release together with breaking CLI changes (AND selectors by default).

## Open questions
- v3 flag alias/env semantics vs current flags (`--duration, d`, `$DRY-RUN`).
