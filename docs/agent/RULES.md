# Pumba Agent Rules

## Golden Standards

### Code Style
1.  **Format**: Always run `go fmt` and `goimports` (or `gci`).
2.  **Lint**: `golangci-lint` must pass with default config (`.golangci.yaml`).
3.  **Imports**: Group standard lib, then 3rd party, then local.

### Testing
1.  **Unit Tests**: Every new feature needs a `_test.go` file.
    *   Use `testify/assert` and `testify/mock`.
    *   Mocks are in `mocks/`. Generate with `make mocks`.
2.  **Integration Tests**: Critical paths must be tested in `tests/*.bats`.
3.  **Stress Tests**: Verify `cg-inject` logic if touching stress.

### Workflow
1.  **Plan first**: Before coding, read/create a plan in `docs/agent/plans/active/`.
2.  **Self-Correction**: If build fails, run `make agent-fix` (if available) or `go mod tidy` + `go fmt ./...`.
3.  **No Blind Edits**: Read file -> Edit -> Verify.

### Runtime Abstraction (Specific)
- **Do NOT** import `github.com/docker/docker/...` in `pkg/chaos/`. Use it ONLY in `pkg/runtime/docker/`.
- `pkg/chaos` should be runtime-agnostic.
