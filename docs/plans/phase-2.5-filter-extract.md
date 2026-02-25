# Plan: Extract Filter Logic

## Validation Commands

- `make test`
- `golangci-lint run`

### Task 1: Create filter package

- [x] Create `pkg/container/filter.go`
- [x] Define `Filter` interface or struct in `pkg/container`
- [x] Move `applyContainerFilter`, `matchNames`, `matchPattern` logic from `pkg/runtime/docker/client.go` (or where it resides) to `pkg/container/filter.go`
- [x] Ensure logic uses `Container` struct, not Docker-specific types
- [x] Add unit tests for filter logic in `pkg/container/filter_test.go`

### Task 2: Refactor Docker runtime

- [x] Update `pkg/runtime/docker/client.go` to use new `pkg/container` filter logic
- [x] Remove old filter code from Docker runtime
- [x] Fix any compilation errors
- [x] Verify `make test` passes

### Task 3: Update CLI commands

- [x] Check `cmd/` for any direct usage of old filter logic
- [x] Update to use new `pkg/container` filter
- [x] Verify all commands build and run
