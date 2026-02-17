# Plan: Polish PR 291 (Stress Cgroups V2)

## Validation Commands
- `go test ./...`
- `golangci-lint run`

### Task 1: Review and Fix Cgroups V2 Implementation
- [ ] Review `pkg/cgroups/v2.go` (and related changes) for correctness and edge cases.
- [ ] Verify backward compatibility with cgroups v1.
- [ ] Fix any issues found during review.
- [ ] Ensure integration tests cover cgroups v2 logic.

### Task 2: Add Missing Tests
- [ ] Add unit tests for cgroups v2 logic in `pkg/cgroups/v2_test.go` (create if missing).
- [ ] Add integration tests for stress command with cgroups v2 in `tests/stress/cgroups_v2_test.go`.

### Task 3: Update Documentation
- [ ] Update README.md to document new cgroups v2 support.
- [ ] Add example usage of stress command with cgroups v2.
- [ ] Check if `docs/` needs updates.
