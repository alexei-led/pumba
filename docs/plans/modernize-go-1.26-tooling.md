# Plan: Modernize to Go 1.26 and golangci-lint v2

Upgrade Pumba from Go 1.24 to Go 1.26 and modernize all tooling. This includes migrating the golangci-lint config to v2 format, replacing deprecated github.com/pkg/errors with stdlib, and adopting Go 1.26 stdlib improvements.

## Context

- CLAUDE.md in project root has full project context — READ IT FIRST
- Default branch: master
- golangci-lint v2.9.0 is installed but rejects current .golangci.yaml (v1 format)
- 39 non-mock Go files use github.com/pkg/errors
- Do NOT modify mock files (mocks/\_.go, pkg/container/mock\_\_.go)
- Do NOT add AI co-author to commits

## Validation Commands

- `TARGETOS=darwin TARGETARCH=amd64 make build`
- `CGO_ENABLED=1 TARGETOS=darwin TARGETARCH=amd64 go test -timeout 15s ./...`
- `golangci-lint run -v -c .golangci.yaml ./...`

### Task 1: Migrate golangci-lint config to v2 format

- [x] Run `golangci-lint migrate` to auto-migrate .golangci.yaml to v2 format
- [x] Verify with `golangci-lint run -v -c .golangci.yaml ./...` — fix any config errors
- [x] Review migrated config: remove deprecated linters, ensure all enabled linters are valid in v2
- [x] Note: some linters may have been renamed or removed in v2. Check migration output carefully.

### Task 2: Replace github.com/pkg/errors with stdlib errors

- [ ] In ALL non-mock .go files (including tests): replace github.com/pkg/errors with stdlib errors and fmt
- [ ] Migration patterns:
  - errors.Wrap(err, "msg") → fmt.Errorf("msg: %w", err)
  - errors.Wrapf(err, "fmt %s", arg) → fmt.Errorf("fmt %s: %w", arg, err) (note: %w MUST be last verb)
  - errors.Errorf("fmt", args...) → fmt.Errorf("fmt", args...)
  - errors.New("msg") → errors.New("msg") (stdlib errors package)
  - errors.Cause(err) → use errors.As() or errors.Unwrap() as appropriate
  - errors.WithMessage(err, "msg") → fmt.Errorf("msg: %w", err)
  - errors.WithStack(err) → just return err (no stack traces needed)
- [ ] Also update test files that import pkg/errors
- [ ] Run go mod tidy to clean up go.mod/go.sum
- [ ] Verify: go build ./... and go test ./... both pass

### Task 3: Update Go version to 1.26 everywhere

- [ ] Update go.mod: change go 1.24 to go 1.26
- [ ] Update docker/Dockerfile: change golang:1.24 to golang:1.26
- [ ] Update .github/workflows/build.yaml: change go-version 1.24 to 1.26
- [ ] Update .github/workflows/release.yaml: same
- [ ] Update .github/workflows/codeql-analysis.yml: same
- [ ] Run go mod tidy

### Task 4: Apply Go 1.26 stdlib improvements

- [ ] Use slices package functions where manual slice operations exist (sort, contains, etc.)
- [ ] Use maps package where applicable
- [ ] Use range-over-int (for i := range n) where applicable instead of for i := 0; i < n; i++
- [ ] Replace any deprecated stdlib API calls with modern equivalents
- [ ] Keep changes minimal and safe — don't refactor working logic, only modernize patterns
- [ ] Run tests after changes
