# Plan: Narrow chaos command interfaces (Phase 2.4)

Each chaos command currently accepts `container.Client` (full interface with ~15 methods), but only uses 2-3 methods. Narrow each command to accept the smallest focused interface it needs. This is the key step enabling multi-runtime support — commands will work with any runtime that implements the narrow interface.

**Branch:** `refactor/narrow-interfaces`
**Base:** `master` (after PR #296 is merged)

## Validation Commands

- `make lint`
- `make test`

### Task 1: Narrow docker chaos commands (stop, kill, restart, pause, remove, exec)

Each command in `pkg/chaos/docker/` accepts `container.Client` but only needs `container.Lifecycle` (or `container.Executor` for exec). Change:

- [x] **stop.go**: `client container.Client` → `client stopClient` (local interface: `Lister + Lifecycle`)
- [x] **kill.go**, **restart.go**, **pause.go**, **remove.go** — same pattern, `Lister + Lifecycle`
- [x] **exec.go** — `execClient` interface: `Lister + Executor`
- [x] `cmd/*.go` files — no changes needed, `Client` satisfies all narrow interfaces
- [x] Tests — existing mocks implement `Client`, tests pass unchanged
- [x] `make lint && make test` — verified

### Task 2: Narrow netem commands (delay, loss, corrupt, duplicate, rate, loss_ge, loss_state)

All netem commands use `container.Client` but only need `container.Lister + container.Netem`.

- [x] In `pkg/chaos/netem/netem.go`: define local `netemClient` interface, change `netemCommand` struct field and `newNetemCommand`/`runNetem` params
- [x] Update all `New*Command` functions in delay.go, loss.go, corrupt.go, duplicate.go, rate.go, loss_ge.go, loss_state.go to accept `netemClient`
- [x] Refactor corrupt.go and duplicate.go to embed `netemCommand` (consistent with other netem commands)
- [x] `make lint && make test` — verified

### Task 3: Narrow iptables and stress commands

- [ ] **pkg/chaos/iptables/iptables.go**: `ipTablesCommand` struct and `newIPTablesCommand` need `Lister + IPTables`. Define local `type iptablesClient interface { container.Lister; container.IPTables }`.
- [ ] **pkg/chaos/iptables/loss.go**: `NewLossCommand` — update to accept narrow interface.
- [ ] **pkg/chaos/stress/stress.go**: `stressCommand` needs `Lister + Stressor + Lifecycle`. Define local `type stressClient interface { container.Lister; container.Stressor; container.Lifecycle }`.
- [ ] `make lint && make test` — verified

### Task 4: Narrow the global DockerClient and update cmd/main.go

- [ ] In `pkg/chaos/command.go`: add TODO comment on `DockerClient` noting it should be removed in Phase 4 (dependency injection).
- [ ] In `cmd/main.go`: verify all `New*Command(chaos.DockerClient, ...)` calls compile.
- [ ] `make lint && make test` — verified
- [ ] Verify no remaining direct use of `container.Client` in `pkg/chaos/` except `command.go`.

### Task 5: Add integration-level test for interface satisfaction

- [ ] Add compile-time interface satisfaction check in `pkg/runtime/docker/docker_test.go`: `var _ container.Client = (*dockerClient)(nil)`
- [ ] `make lint && make test` — verified
