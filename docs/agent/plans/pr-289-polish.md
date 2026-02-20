# Plan: Polish PR 289 (Rm Stopped Containers)

## Validation Commands
- `go test ./...`
- `golangci-lint run`

### Task 1: Review and Verify Fix
- [ ] Review implementation of finding stopped containers in `pkg/chaos/docker/remove.go` and `pkg/container/util.go`.
- [ ] Verify correctness and edge cases.
- [ ] Ensure backward compatibility.
- [ ] Fix any issues found.

### Task 2: Add Missing Tests
- [ ] Add unit tests for finding stopped containers in `pkg/chaos/docker/remove_test.go`.
- [ ] Add integration test case for removing a stopped container.

### Task 3: Update Documentation
- [ ] Update README.md to mention `rm` command now works on stopped containers.
- [ ] Verify `docs/` is consistent.
