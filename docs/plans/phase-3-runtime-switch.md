# Plan: Phase 3 - Runtime Selection

## Goal

Support runtime selection via `--runtime` flag (defaulting to "docker"). Prepare the codebase for `containerd` implementation.

## Validation Commands

- `make test`
- `make build`
- `pumba --help` (check for --runtime flag)
- `pumba --runtime=docker ...` (should work)
- `pumba --runtime=containerd ...` (should fail gracefully with "not implemented")

### Task 1: Create generic Runtime factory

- [x] Create `pkg/runtime/factory.go` (or similar)
- [x] Define `NewClient(runtime string, ...)` function
- [x] Move Docker client creation logic into the factory
- [x] Add `containerd` stub implementation (returns error "not implemented")

### Task 2: Add --runtime flag

- [x] Update `cmd/main.go` to add `--runtime` string flag (default: "docker")
- [x] Update `Before` hook in `main.go` to read `--runtime` flag
- [x] Use `runtime.NewClient` instead of `docker.NewClient`
- [x] Verify `chaos.DockerClient` variable is renamed/replaced with `chaos.ContainerClient` (interface type)

### Task 3: Rename global client variable

- [ ] Rename `chaos.DockerClient` to `chaos.ContainerClient` in `pkg/chaos/command.go`
- [ ] Update all references in `pkg/chaos/` to use `chaos.ContainerClient`
- [ ] Verify compilation and tests pass

### Task 4: Verify Runtime Switch

- [ ] Run `pumba --runtime=docker` integration tests (should pass)
- [ ] Run `pumba --runtime=containerd` (should fail with specific error)
