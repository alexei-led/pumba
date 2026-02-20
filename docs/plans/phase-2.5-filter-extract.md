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

- [ ] Update `pkg/runtime/docker/client.go` to use new `pkg/container` filter logic
- [ ] Remove old filter code from Docker runtime
- [ ] Fix any compilation errors
- [ ] Verify `make test` passes

### Task 3: Update CLI commands

- [ ] Check `cmd/` for any direct usage of old filter logic
- [ ] Update to use new `pkg/container` filter
- [ ] Verify all commands build and run
