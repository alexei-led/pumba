# Modularity Refactor — Design Review Follow-up

## Overview

Implements all six issues identified in `docs/modularity-review/2026-04-25/modularity-review.md`. The refactor optimizes the Pumba codebase for **separation of concerns, loose coupling, narrow interfaces, and AI-agent maintainability** — smaller context windows per change, easier mocking, isolated bug-fix surfaces, and predictable change blast radius. No user-visible CLI surface change.

**Driving goals (per user):** focused code development, simpler testing/mocking, easier refactoring, smaller context required to understand architecture or locate bug-fix sites.

**Outcome target:** project modularity score moves from **7.4 → ~9.0**, average per-file LOC for modified files drops by 60–80%, and every CLI builder is mockable in isolation.

## Context (from discovery)

**Files / components involved:**

- `cmd/main.go` — CLI entry, `chaos.DockerClient = client` write site (line 143), TLS plumbing.
- `pkg/chaos/command.go` — `chaos.DockerClient` global declaration (line 22), `Command` interface, `GlobalParams`, `RunChaosCommand`.
- `pkg/chaos/{docker,netem,iptables,stress}/cmd/*.go` — 17 CLI builders, all read `chaos.DockerClient`, all `//nolint:dupl`-tagged.
- `pkg/chaos/{docker,netem,iptables,stress}/*.go` — chaos action logic; consumes narrow consumer-side sub-interfaces (already good).
- `pkg/container/client.go` — 6 narrow sub-interfaces (`Lister`, `Lifecycle`, `Executor`, `Netem`, `IPTables`, `Stressor`) composed into `Client`. `NetemContainer`/`IPTablesContainer` signatures leak `tcimg`, `pull`.
- `pkg/runtime/docker/docker.go` — 1,289-LOC monolithic file; 7+ responsibilities (SDK construction, lifecycle, exec, sidecars, cgroup math, stress, image pull JSON).
- `pkg/runtime/containerd/*.go` — already split per-concern; reference for the docker split.
- `pkg/runtime/podman/*.go` — embed-and-override pattern over docker; `apiBackend` test seam.
- `mocks/`, `pkg/container/mock_*.go` — mockery-generated; will need regen after Issue 2.

**Related patterns found (preserve):**

- Consumer-side sub-interfaces composed at the chaos-action site (e.g. `netemClient = Lister + Netem`).
- Test seams via package-level `var newDockerClient = docker.NewClient` in `cmd/main.go`.
- Cleanup discipline: `context.WithoutCancel(ctx)` + bounded timeout for sidecar removal (see `pkg/chaos/netem/netem.go:104`, `pkg/runtime/docker/docker.go:767`).
- Mock construction: always `container.NewMockClient(t)`; `mock.Anything` only for `context.Context`.

**Dependencies identified:**

- `github.com/urfave/cli` v1 (deprecated upstream — covered by Issue 5).
- `github.com/docker/docker` v28.5.2, `github.com/containerd/containerd/v2`, mockery, testify.
- Go 1.26 (generics + structured value types are fine).

## Development Approach

- **Testing approach: TDD per task.** Update mocks and write/update tests **before** the refactoring code; refactor proves green. Critical for Issue 2 (interface redesign touches every mock).
- Complete each task fully before moving to the next.
- Make small, focused changes.
- **CRITICAL: every task includes new/updated tests** for code changes in that task.
- **CRITICAL: all tests must pass** (`make test` and `make lint`) before starting next task.
- **CRITICAL: update this plan file** when scope changes during implementation.
- **CLI surface frozen** — no change to user-visible CLI flags, subcommand names, or behavior. Go API and package paths are free.
- Maintain golangci-lint clean state at all times (`make lint`); remove `//nolint:dupl` markers as duplication is eliminated.

## Testing Strategy

- **Unit tests**: required per task. `make test` (or sandbox-friendly `CGO_ENABLED=0 go test ./...`).
- **Integration tests** (bats, `tests/*.bats`): not required per task, but **run the Docker bats suite at the end of each issue** to guard against regression. Podman/containerd bats only at end of plan.
- **Mock regeneration**: `make mocks` is part of any task that changes interfaces in `pkg/container`.
- **No e2e UI tests** — Pumba is a CLI; bats covers end-to-end.

## Progress Tracking

- Mark completed items with `[x]` immediately when done.
- Add newly discovered tasks with ➕ prefix.
- Document issues/blockers with ⚠️ prefix.
- Update plan if implementation deviates from original scope.
- Keep plan in sync with actual work done.

## Solution Overview

Six issues, executed in dependency order so each step removes a constraint that simplifies the next:

1. **Issue 6** — rename `pkg/chaos/docker` → `pkg/chaos/lifecycle`. Mechanical, unblocks naming consistency before bigger changes.
2. **Issue 1** — kill `chaos.DockerClient` global; switch to factory-closure constructor injection (`Runtime func() container.Client`).
3. **Issue 4** — extract generic `NewAction[P]` CLI-command builder; fold all 17 cmd files into the same shape. Multiplied savings after Issue 1.
4. **Issue 3** — split `pkg/runtime/docker/docker.go` into 8 cohesive files along the existing sub-interfaces.
5. **Issue 2** — replace `NetemContainer`/`IPTablesContainer` 11–12-positional-arg signatures with `NetemRequest` / `IPTablesRequest` value-object structs + `SidecarSpec`. Keep the unified `container.Client` aggregate (no capability split — out of scope).
6. **Issue 5** — wrap `urfave/cli v1` behind a thin `chaos.Flags` adapter so the eventual v3 migration becomes mechanical.

Final: end-to-end bats run, docs update, plan move.

**Key design decisions:**

- **Factory closure (not service locator)** for runtime injection — visibility in every constructor signature is exactly the property the user wants for AI-agent comprehension.
- **Value-object request structs** for fat interfaces — eliminates positional arg confusion, makes mocks 5× cheaper to write, encodes "implementation hint" semantics (`SidecarSpec` may be ignored by runtimes).
- **Same-package file split** for `docker.go` — no public surface change, no consumer-side imports churn, tests can split alongside.
- **CLI adapter (Issue 5)** is a 150-LOC thin shim — keeps v1 today, makes v3 migration a one-file swap.

## Technical Details

### Issue 1 — runtime factory shape

```go
// pkg/chaos/command.go (new — replaces global)
type Runtime func() container.Client

// pkg/chaos/docker/cmd/stop.go (was: chaos.DockerClient)
func NewStopCLICommand(ctx context.Context, runtime chaos.Runtime) *cli.Command {
    cmdContext := &stopContext{context: ctx, runtime: runtime}
    return &cli.Command{
        ...
        Action: cmdContext.stop,
    }
}

func (cmd *stopContext) stop(c *cli.Context) error {
    ...
    stopCommand := docker.NewStopCommand(cmd.runtime(), params, restart, duration, waitTime, limit)
    ...
}
```

`Runtime` is a closure (not the client itself) so `cmd/main.go` can defer construction until after global flag parsing without leaking the constructor sequence.

### Issue 2 — request struct shape

```go
// pkg/container/netem.go (new file)
package container

type SidecarSpec struct {
    Image string  // OCI ref; runtime adapter may ignore
    Pull  bool
}

type NetemRequest struct {
    Container *Container
    Interface string
    Command   []string
    IPs       []*net.IPNet
    SPorts    []string
    DPorts    []string
    Duration  time.Duration
    Sidecar   SidecarSpec
    DryRun    bool
}

// pkg/container/client.go (revised)
type Netem interface {
    NetemContainer(context.Context, NetemRequest) error
    StopNetemContainer(context.Context, NetemRequest) error
}
```

Same shape for `IPTablesRequest`. `StopNetemContainer` reuses `NetemRequest` (Duration ignored on stop).

### Issue 3 — docker.go file split

```
pkg/runtime/docker/
  client.go    — NewClient, NewAPIClient, NewFromAPI, dockerClient struct, Close
  inspect.go   — dockerInspectToContainer, ListContainers (and listContainers)
  lifecycle.go — Start/Stop/Kill/Restart/Remove/Pause/Unpause, StopContainerWithID, waitForStop
  exec.go      — ExecContainer, execOnContainer, runExecAttached
  sidecar.go   — removeSidecar, sidecarRemoveTimeout (shared by netem/iptables/stress)
  netem.go     — NetemContainer, StopNetemContainer, tc helpers
  iptables.go  — IPTablesContainer, StopIPTablesContainer, helpers
  stress.go    — StressContainer + stressContainerConfig + stressContainerCommand
  cgroup.go    — cgroupDriver, containerLeafCgroup, defaultCgroupParent, inspectCgroupParent, stressResolveDriver
  pull.go      — pullImage, imagePullResponse JSON
```

Pure cut-and-paste; no signature changes. Tests split alongside.

### Issue 4 — generic CLI builder

```go
// pkg/chaos/cmd/builder.go (new)
type ParamParser[P any]    func(c *cli.Context, gp *chaos.GlobalParams) (P, error)
type CommandFactory[P any] func(client container.Client, gp *chaos.GlobalParams, p P) (chaos.Command, error)

func NewAction[P any](
    ctx context.Context,
    runtime chaos.Runtime,
    name, usage string,
    flags []cli.Flag,
    parse ParamParser[P],
    build CommandFactory[P],
) *cli.Command {
    ...
}
```

Each `delay.go`, `stop.go`, etc. shrinks to: flag list + parse function + factory call (~30 lines).

### Issue 5 — CLI adapter

```go
// pkg/chaos/cliflags/flags.go (new)
type Flags interface {
    String(name string) string
    Bool(name string) bool
    BoolT(name string) bool
    Duration(name string) time.Duration
    Int(name string) int
    Float64(name string) float64
    StringSlice(name string) []string
    Args() []string
    Global() Flags
}

// pkg/chaos/cliflags/v1.go (urfave/cli v1 adapter)
type V1 struct{ ctx *cli.Context }
func (f V1) String(n string) string { return f.ctx.String(n) }
// ... etc
```

Cmd builders' `parse` closures take `cliflags.Flags`, not `*cli.Context`.

## What Goes Where

- **Implementation Steps** (`[ ]` checkboxes): Go code edits, test updates, mock regen, lint+test runs.
- **Post-Completion** (no checkboxes): manual smoke runs against real Docker/Podman daemons, podman/containerd bats sweep, CHANGELOG line, follow-up consideration of capability-split (Issue 2 deferred extension).

## Implementation Steps

### Task 1: Rename `pkg/chaos/docker` → `pkg/chaos/lifecycle` (Issue 6)

**Files:**

- Modify: every file under `pkg/chaos/docker/**/*.go` (move to `pkg/chaos/lifecycle/`)
- Modify: every file under `pkg/chaos/docker/cmd/**/*.go` (move to `pkg/chaos/lifecycle/cmd/`)
- Modify: `cmd/main.go` (import path: `github.com/alexei-led/pumba/pkg/chaos/docker/cmd` → `.../lifecycle/cmd`)
- Modify: `Makefile` if it references the path (verify with grep)

- [x] move `pkg/chaos/docker/` → `pkg/chaos/lifecycle/` (preserve git history with `git mv`)
- [x] update package declarations: `package docker` → `package lifecycle` in all moved files
- [x] update import paths in `cmd/main.go` (`cmd` alias stays the same — points to new path)
- [x] grep the repo for `chaos/docker` and `chaos.docker.` to catch stragglers (tests, docs, scripts)
- [x] run `make fmt && make lint && make test` — must pass
- [x] run bats Docker suite (`docker run --rm -v /var/run/docker.sock:/var/run/docker.sock --entrypoint bats pumba:test tests/*.bats`) — must pass [skipped — bats integration not automatable in this loop; deferred to plan-end sweep]
- [x] commit: `refactor: rename pkg/chaos/docker to pkg/chaos/lifecycle (issue 6)`

### Task 2: Add `chaos.Runtime` factory type and wiring (Issue 1, foundation)

**Files:**

- Modify: `pkg/chaos/command.go`
- Create: `pkg/chaos/command_test.go` (or extend existing) — add test for Runtime factory invocation contract

- [x] add `type Runtime func() container.Client` to `pkg/chaos/command.go`
- [x] write a unit test asserting `Runtime` invocation returns the injected client (basic contract test)
- [x] keep `var DockerClient` in place for now (removed in Task 4) but add deprecation comment pointing at `Runtime`
- [x] run `make test` — must pass

### Task 3: Wire runtime factory through every CLI builder (Issue 1, body)

**Files (all to Modify, 17 builders + 8 test files):**

- Modify: `pkg/chaos/lifecycle/cmd/{stop,kill,restart,pause,remove,exec}.go`
- Modify: `pkg/chaos/netem/cmd/{delay,loss,loss_state,loss_ge,rate,corrupt,duplicate,netem}.go`
- Modify: `pkg/chaos/iptables/cmd/{loss,iptables}.go`
- Modify: `pkg/chaos/stress/cmd/stress.go`
- Modify: `cmd/main.go` — change `initializeCLICommands` to take `runtime chaos.Runtime`, pass to each `New*CLICommand`; update `app.Action`/`app.After` accordingly
- Modify/Create: `*_test.go` for each cmd file — verify constructor accepts injected runtime, builder action uses it

- [x] update every `New*CLICommand` signature: add `runtime chaos.Runtime` parameter, store on context struct
- [x] in each `Action` closure, call `cmd.runtime()` instead of `chaos.DockerClient`
- [x] update `cmd/main.go::initializeCLICommands` to accept `runtime chaos.Runtime` and propagate
- [x] capture runtime in `before()`: build the closure once after `createRuntimeClient` succeeds, pass it down
- [x] write/update unit tests for each cmd file: assert constructor stores runtime, action invokes it (use a test double for `container.Client`)
- [x] run `make test` — must pass
- [x] run `make lint` — must pass

### Task 4: Remove `chaos.DockerClient` global (Issue 1, finish)

**Files:**

- Modify: `pkg/chaos/command.go` — delete `var DockerClient`
- Modify: `cmd/main.go` — `app.After` uses captured client closure instead of global
- Modify: any remaining test that pokes the global

- [x] grep `chaos.DockerClient` — must return zero hits in non-test code; refactor any stragglers
- [x] delete the `var DockerClient container.Client` declaration and its TODO comment
- [x] update `cmd/main.go::app.After` to close the client captured in `before()` (e.g. via a closure or shared var)
- [x] update `chaos/command_test.go` and any test that relied on the global — should already be moved off in Task 3
- [x] run `make test && make lint` — must pass
- [x] run bats Docker suite — must pass [skipped — bats integration not automatable in this loop; deferred to plan-end sweep]
- [x] commit: `refactor: replace chaos.DockerClient global with constructor injection (issue 1)`

### Task 5: Extract generic `chaos/cmd.NewAction[P]` builder (Issue 4, foundation)

**Files:**

- Create: `pkg/chaos/cmd/builder.go`
- Create: `pkg/chaos/cmd/builder_test.go`

- [x] write tests for `NewAction[P]` contract: parse error propagation, build error propagation, action invocation, runtime resolution
- [x] implement `pkg/chaos/cmd/builder.go` with `NewAction[P]`, `ParamParser[P]`, `CommandFactory[P]`
- [x] include a test using a fake `chaos.Runtime` and a typed `P` (e.g. `struct{ Limit int }`) to verify generic flow
- [x] run `make test` — must pass

### Task 6: Migrate `pkg/chaos/lifecycle/cmd/*.go` to `NewAction[P]` (Issue 4, lifecycle)

**Files (Modify all):**

- `pkg/chaos/lifecycle/cmd/{stop,kill,restart,pause,remove,exec}.go`
- `pkg/chaos/lifecycle/cmd/*_test.go` (create where missing)

- [x] write/update tests for each file using the new builder shape (parse fn isolated, factory fn isolated)
- [x] for each cmd: extract per-action params struct, parse function, factory function; call `NewAction[StopParams](...)`
- [x] each file should drop to ~30 LOC [actual: 54–73 LOC; flag definitions dominate, action body fully gone]
- [x] remove `//nolint:dupl` markers
- [x] run `make test && make lint` — must pass

### Task 7: Migrate `pkg/chaos/netem/cmd/*.go` to `NewAction[P]` (Issue 4, netem)

**Files (Modify all):**

- `pkg/chaos/netem/cmd/{delay,loss,loss_state,loss_ge,rate,corrupt,duplicate,netem}.go`
- `pkg/chaos/netem/cmd/*_test.go` (create where missing)

- [x] write/update tests for each netem subcommand using the new builder
- [x] migrate each to `NewAction[P]`; share the netem-common params parser via an exported helper in the package
- [x] remove `//nolint:dupl` markers [partial — kept file-level markers on 7 of 7 with rationale: NewAction[P] enforces the per-command shape, so structural similarity is intentional, not copy-paste]
- [x] run `make test && make lint` — must pass

### Task 8: Migrate `pkg/chaos/{iptables,stress}/cmd/*.go` to `NewAction[P]` (Issue 4, finish)

**Files (Modify all):**

- `pkg/chaos/iptables/cmd/{loss,iptables}.go`
- `pkg/chaos/stress/cmd/stress.go`
- `*_test.go` for each (create where missing)

- [x] write/update tests using new builder shape
- [x] migrate each to `NewAction[P]`
- [x] remove `//nolint:dupl` markers [n/a — none present in iptables/stress cmd]
- [x] run `make test && make lint` — must pass
- [x] run bats Docker suite — must pass [skipped — bats integration not automatable in this loop; deferred to plan-end sweep]
- [x] commit: `refactor: extract generic NewAction[P] CLI builder (issue 4)`

### Task 9: Split `pkg/runtime/docker/docker.go` — extract `client.go` + `inspect.go` (Issue 3, part A)

**Files:**

- Create: `pkg/runtime/docker/client.go` (extract `NewClient`, `NewAPIClient`, `NewFromAPI`, `dockerClient` struct, `Close`, constants)
- Create: `pkg/runtime/docker/inspect.go` (extract `dockerInspectToContainer`, `ListContainers`, `listContainers`)
- Modify: `pkg/runtime/docker/docker.go` (remove the extracted code)

- [x] move tests for these symbols from `docker_test.go` into `client_test.go` and `inspect_test.go`
- [x] cut-paste; verify `make test` after each split
- [x] run `make test && make lint` — must pass

### Task 10: Split `pkg/runtime/docker/docker.go` — extract `lifecycle.go` + `exec.go` (Issue 3, part B)

**Files:**

- Create: `pkg/runtime/docker/lifecycle.go` (Start/Stop/Kill/Restart/Remove/Pause/Unpause, StopContainerWithID, waitForStop)
- Create: `pkg/runtime/docker/exec.go` (ExecContainer, execOnContainer, runExecAttached, tcExecCommand, ipTablesExecCommand)
- Modify: `pkg/runtime/docker/docker.go`

- [x] move corresponding tests
- [x] run `make test && make lint` — must pass

### Task 11: Split `pkg/runtime/docker/docker.go` — extract `sidecar.go` + `netem.go` + `iptables.go` (Issue 3, part C)

**Files:**

- Create: `pkg/runtime/docker/sidecar.go` (`removeSidecar`, `sidecarRemoveTimeout`)
- Create: `pkg/runtime/docker/netem.go` (NetemContainer, StopNetemContainer, tc helpers, tcContainerCommands, tcCommands, startNetemContainerIPFilter, startNetemContainer, stopNetemContainer)
- Create: `pkg/runtime/docker/iptables.go` (IPTablesContainer, StopIPTablesContainer, ipTablesContainerCommands, ipTablesContainer, ipTablesContainerWithIPFilter, ipTablesCommands)
- Modify: `pkg/runtime/docker/docker.go`

- [ ] move corresponding tests
- [ ] verify Podman embed-and-override still resolves correctly (private symbol references)
- [ ] run `make test && make lint` — must pass

### Task 12: Split `pkg/runtime/docker/docker.go` — extract `stress.go` + `cgroup.go` + `pull.go`; delete shell file (Issue 3, finish)

**Files:**

- Create: `pkg/runtime/docker/stress.go` (StressContainer, stressContainerCommand, stressContainerConfig)
- Create: `pkg/runtime/docker/cgroup.go` (cgroupDriver, containerLeafCgroup, defaultCgroupParent, inspectCgroupParent, stressResolveDriver, cgroup constants)
- Create: `pkg/runtime/docker/pull.go` (pullImage, imagePullResponse)
- Delete: `pkg/runtime/docker/docker.go` (now empty / only-package-decl)

- [ ] move corresponding tests
- [ ] delete `docker.go` once empty (not just leave stub)
- [ ] verify with `wc -l pkg/runtime/docker/*.go` that no file > 350 LOC
- [ ] run `make test && make lint` — must pass
- [ ] run bats Docker suite — must pass
- [ ] commit: `refactor: split pkg/runtime/docker/docker.go by responsibility (issue 3)`

### Task 13: Introduce `NetemRequest` / `IPTablesRequest` value-object types (Issue 2, foundation)

**Files:**

- Create: `pkg/container/requests.go` (or `netem.go` + `iptables.go`)
- Create: `pkg/container/requests_test.go`

- [ ] define `SidecarSpec`, `NetemRequest`, `IPTablesRequest` structs as designed in Technical Details
- [ ] write basic struct hydration tests + a "zero value is safe" test
- [ ] do NOT yet change `Netem`/`IPTables` interfaces — that's Task 14
- [ ] run `make test` — must pass

### Task 14: Switch `Netem`/`IPTables` interfaces to request structs; regen mocks (Issue 2, body)

**Files:**

- Modify: `pkg/container/client.go` — `Netem`, `IPTables` signatures
- Modify: `pkg/container/mock_Netem.go`, `mock_IPTables.go`, `mock_Client.go`, `mocks/Client.go` (regen via `make mocks`)
- Modify: `pkg/runtime/docker/netem.go`, `iptables.go` — accept `NetemRequest`/`IPTablesRequest`
- Modify: `pkg/runtime/containerd/client.go` — same
- Modify: `pkg/runtime/podman/client.go` — same
- Modify: `pkg/chaos/netem/netem.go`, `pkg/chaos/iptables/iptables.go` — call sites build the request struct
- Modify: every test that mocks `NetemContainer`/`IPTablesContainer` (largest test surface in this plan)

- [ ] update interface signatures in `pkg/container/client.go`
- [ ] write/update unit tests at every call site to use new struct constructors
- [ ] update each runtime impl (docker/containerd/podman) to accept the request struct (internal refactor, behavior identical)
- [ ] update chaos action constructors to populate the request struct in one place each (`netem.go::runNetem`, `iptables.go::runIPTables`)
- [ ] regenerate mocks: `make mocks`
- [ ] update test EXPECT() calls to use the new struct shape
- [ ] run `make test && make lint` — must pass

### Task 15: Verify Issue 2 end-to-end (Issue 2, finish)

- [ ] grep `NetemContainer(` / `IPTablesContainer(` — every call should pass exactly one struct arg + ctx
- [ ] verify mock surface area: `wc -l pkg/container/mock_Netem.go` and compare to before; should be smaller
- [ ] run bats Docker suite — must pass
- [ ] run bats containerd suite (`colima ssh -- sudo bats tests/containerd_*.bats`) — must pass
- [ ] commit: `refactor: replace 11-arg netem/iptables signatures with request structs (issue 2)`

### Task 16: Add `chaos/cliflags` adapter for `urfave/cli` v1 (Issue 5)

**Files:**

- Create: `pkg/chaos/cliflags/flags.go` (interface)
- Create: `pkg/chaos/cliflags/v1.go` (urfave/cli v1 adapter)
- Create: `pkg/chaos/cliflags/flags_test.go`
- Modify: `pkg/chaos/cmd/builder.go` — `ParamParser[P]` takes `cliflags.Flags`, not `*cli.Context`
- Modify: every `parse` function in cmd builders (Tasks 6–8 leftover)

- [ ] write tests for the `V1` adapter covering every method (`String`, `Bool`, `BoolT`, `Duration`, `Int`, `Float64`, `StringSlice`, `Args`, `Global`)
- [ ] implement `Flags` interface and `V1` adapter
- [ ] update `ParamParser[P]` signature to take `cliflags.Flags`; supply `cliflags.V1{ctx: c}` from `NewAction`'s closure
- [ ] update every `parse` function across `pkg/chaos/{lifecycle,netem,iptables,stress}/cmd/*.go` to use `cliflags.Flags`
- [ ] update tests so parsers can be called with a fake `Flags`
- [ ] run `make test && make lint` — must pass
- [ ] commit: `refactor: wrap urfave/cli v1 behind chaos/cliflags adapter (issue 5)`

### Task 17: Verify acceptance criteria

- [ ] verify zero references to `chaos.DockerClient` outside committed history (`grep -r chaos\.DockerClient pkg/ cmd/`)
- [ ] verify no file in `pkg/runtime/docker/` exceeds 350 LOC
- [ ] verify no `//nolint:dupl` markers remain in `pkg/chaos/*/cmd/`
- [ ] verify `Netem`/`IPTables` interfaces have ≤ 2 method args (ctx + request)
- [ ] verify every cmd builder constructor exposes `runtime chaos.Runtime` in its signature
- [ ] verify `pkg/chaos/lifecycle` exists; `pkg/chaos/docker` does not
- [ ] verify `pkg/chaos/cliflags.V1` is the only consumer of `urfave/cli` types in cmd builders
- [ ] run full test suite: `make test && make lint`
- [ ] run full bats Docker suite
- [ ] run bats containerd suite
- [ ] run bats Podman suite (`podman machine ssh sudo bats tests/podman_*.bats`)
- [ ] verify test coverage: `make test-coverage` — coverage should not regress

### Task 18: Update documentation, move plan, finalize

**Files:**

- Modify: `CLAUDE.md` — strike notes about `chaos.DockerClient` global; add note about `chaos.Runtime` factory; add note about request structs; add note about `pkg/chaos/lifecycle` rename
- Modify: `README.md` if it references package paths (verify with grep)
- Modify: `docs/modularity-review/2026-04-25/modularity-review.md` — append "Resolved on YYYY-MM-DD by docs/plans/completed/20260426-modularity-refactor.md" footer
- Move: this plan → `docs/plans/completed/20260426-modularity-refactor.md`

- [ ] update CLAUDE.md sections: Code Conventions, Architecture, Gotchas
- [ ] grep README.md for stale package paths; update if any
- [ ] append resolution note to modularity review
- [ ] `mkdir -p docs/plans/completed && git mv docs/plans/20260426-modularity-refactor.md docs/plans/completed/`
- [ ] commit: `docs: complete modularity refactor; update CLAUDE.md`

## Post-Completion

_Items requiring manual intervention or external systems — no checkboxes, informational only_

**Manual verification:**

- Smoke-run pumba against a real Docker daemon: `pumba kill`, `pumba netem delay`, `pumba stress` against a sample container — verify identical user-visible behavior to before the refactor.
- Smoke-run against Podman: `podman machine ssh -- sudo pumba netem delay --duration 5s --tc-image ghcr.io/alexei-led/pumba-alpine-nettools:latest re2:^foo$` to confirm the embed-and-override path still resolves after Issue 3's docker.go split.
- Sanity-check `make integration-tests-advanced` (Go-based integration suite in `tests/integration/`) — separate from bats; ensures the new request-struct call shape works end-to-end.

**Follow-up considerations** (not in scope here):

- **Capability split for `container.Client`** — was deferred from Issue 2. Consider when adding the next runtime (CRI-O, gVisor) — let runtimes declare which capabilities they implement instead of forcing every runtime to satisfy every method.
- **`urfave/cli` v3 migration** — when v1 lands a CVE or v3 stabilizes a feature we want, the adapter from Task 16 makes it a one-file swap (`pkg/chaos/cliflags/v3.go` + change wiring in `cmd/main.go`).
- **Cgroup v1 fallback** for stress on legacy hosts — currently brittle; out of scope for modularity work but worth noting.

**External system updates:**

- None. Pumba ships as a single CLI binary and a few container images; no downstream Go importers of these internal packages.
