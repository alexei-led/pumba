# Pumba Containerd Support Plan (Phase 3)

## Goal

Implement `pkg/container.Client` interface using `github.com/containerd/containerd` to support Containerd runtime.

## Context

The `pkg/container.Client` interface defines focused sub-interfaces (Lister, Lifecycle, Executor, Netem, IPTables, Stressor). The Docker implementation lives in `pkg/runtime/docker/`. We need a parallel containerd implementation in `pkg/runtime/containerd/`.

## Validation

- `make build` compiles clean
- `make test` all tests pass
- `make lint` zero issues

## Steps

### Task 1: Dependencies and Package Structure

- [x] Add `github.com/containerd/containerd/v2` to go.mod and run `go mod tidy`
- [x] Create `pkg/runtime/containerd/` directory
- [x] Create `pkg/runtime/containerd/client.go` with ContainerdClient struct skeleton
- [x] Create `pkg/runtime/containerd/container.go` with containerd-to-pumba container conversion
- [x] Verify `make build` passes

### Task 2: Lister Interface - ListContainers

- [x] Implement `ListContainers` on ContainerdClient
- [x] Connect to containerd socket (default `/run/containerd/containerd.sock`)
- [x] List containers from configurable namespace (default `k8s.io`)
- [x] Convert containerd containers to `pkg/container.Container`
- [x] Apply FilterFunc to results
- [x] Write unit tests with mocked containerd client
- [x] Verify `make test` and `make lint` pass

### Task 3: Lifecycle Interface

- [ ] Implement `StopContainer`, `KillContainer`, `StartContainer`
- [ ] Implement `RestartContainer`, `RemoveContainer`
- [ ] Implement `PauseContainer`, `UnpauseContainer`
- [ ] Implement `StopContainerWithID`
- [ ] Handle task/process lifecycle via containerd API
- [ ] Write unit tests for lifecycle methods
- [ ] Verify `make test` and `make lint` pass

### Task 4: Executor and Network Interfaces

- [ ] Implement `ExecContainer` using containerd task.Exec
- [ ] Implement `NetemContainer` and `StopNetemContainer` wrapping ExecContainer
- [ ] Implement `IPTablesContainer` and `StopIPTablesContainer` wrapping ExecContainer
- [ ] Implement `StressContainer` wrapping ExecContainer
- [ ] Write unit tests for executor and network methods
- [ ] Verify `make test` and `make lint` pass

### Task 5: Final Verification

- [ ] Ensure ContainerdClient fully implements `container.Client` (compile-time check)
- [ ] Run full `make all` pipeline
- [ ] Verify no regressions in existing Docker tests
