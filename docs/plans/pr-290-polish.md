# Plan: Polish PR 290 (Require Args)

## Validation Commands
- `go test ./...`
- `golangci-lint run`

### Task 1: Review and Verify Fix
- [ ] Review implementation of requiring container arguments for `kill`, `stop`, and `rm` commands.
- [ ] Verify correctness and edge cases (e.g. `pumba kill` without args fails).
- [ ] Ensure error messages are clear.
- [ ] Fix any issues found.

### Task 2: Add Missing Tests
- [ ] Add unit tests for argument validation in `pkg/chaos/docker/kill_test.go`, `stop_test.go`, `remove_test.go`.
- [ ] Add integration test case verifying error on missing args.

### Task 3: Update Documentation
- [ ] Update README.md to clarify arguments are required.
- [ ] Verify `docs/` is consistent.
