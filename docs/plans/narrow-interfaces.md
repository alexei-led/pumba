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

1. In `pkg/chaos/netem/netem.go`: change `netemCommand` struct field and `newNetemCommand` function param from `container.Client` to a local interface `type netemClient interface { container.Lister; container.Netem }`.

2. Update `runNetem` function signature similarly.

3. Update all `New*Command` functions in delay.go, loss.go, corrupt.go, duplicate.go, rate.go, loss_ge.go, loss_state.go — they all call `newNetemCommand` so they need to accept the same narrow type.

4. Run `make lint && make test`.

### Task 3: Narrow iptables and stress commands

1. **pkg/chaos/iptables/iptables.go**: `ipTablesCommand` struct and `newIPTablesCommand` need `Lister + IPTables`. Define local `type iptablesClient interface { container.Lister; container.IPTables }`.

2. **pkg/chaos/iptables/loss.go**: `NewLossCommand` — update to accept narrow interface.

3. **pkg/chaos/stress/stress.go**: `stressCommand` needs `Lister + Stressor + Lifecycle` (it calls `ListNContainers`, `StressContainer`, and `StopContainerWithID`). Define local `type stressClient interface { container.Lister; container.Stressor; container.Lifecycle }`.

4. Run `make lint && make test`.

### Task 4: Narrow the global DockerClient and update cmd/main.go

1. In `pkg/chaos/command.go`: the global `DockerClient container.Client` is used by `cmd/main.go` to pass to all commands. Since each command now accepts its own narrow interface, and `container.Client` satisfies all of them, the global var can stay as `container.Client` for now. **But** add a TODO comment noting it should be removed in Phase 4 (dependency injection instead of global var).

2. In `cmd/main.go`: verify all `New*Command(chaos.DockerClient, ...)` calls still compile (they should, since `Client` embeds all narrow interfaces).

3. Run full validation: `make lint && make test`.

4. Verify no remaining direct use of `container.Client` in `pkg/chaos/` except the global var in `command.go`. Run: `grep -rn "container\.Client" pkg/chaos/ --include="*.go" ! --include="*_test.go"` — should only show `command.go`.

### Task 5: Add integration-level test for interface satisfaction

1. In `pkg/container/client_test.go` (or create if needed): add compile-time interface satisfaction checks:

   ```go
   var _ Client = (*docker.DockerClient)(nil) // if docker.DockerClient is exported
   ```

   If `docker.DockerClient` is unexported (it is — `dockerClient`), add the check in `pkg/runtime/docker/docker_test.go` instead:

   ```go
   var _ container.Client = (*dockerClient)(nil)
   ```

2. Run `make lint && make test`.
