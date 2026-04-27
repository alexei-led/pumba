# Pumba Modularity Refactor

## Overview

Address the six findings from `docs/modularity-review/2026-04-25/modularity-review.md` (baseline score 7.4/10). The goal is loose coupling, narrow consumer-side interfaces, and small per-concern files so changes touch ≤ 300 LOC of context. Public CLI surface (flag names, command names) does not change.

Targeted score after refactor: **~9.0/10**. Estimated effort: **5 days** of focused work.

## Context (from discovery)

Files/components involved:

- `pkg/chaos/command.go` — owns the `chaos.DockerClient` package-level mutable global
- `pkg/chaos/docker/` — 6 lifecycle action files (`stop|kill|restart|pause|remove|exec.go`) — runtime-agnostic despite the `docker` package name
- `pkg/chaos/{docker,netem,iptables,stress}/cmd/` — 17 CLI builder files, each `//nolint:dupl`-tagged with ~80 LOC of duplicated boilerplate
- `pkg/container/client.go` — `Netem` and `IPTables` interfaces with 11–12 positional args including Docker-shaped `tcimg`/`pull`
- `pkg/runtime/docker/docker.go` — single 1,289-LOC file mixing 7+ responsibilities
- `pkg/runtime/{containerd,podman}/` — must follow any interface signature change
- `mocks/`, `pkg/container/mock_*.go` — mockery-generated, regenerate after each interface change
- `cmd/main.go` — wires runtime client, currently sets `chaos.DockerClient = client`

Related patterns found:

- Consumer-side sub-interfaces already used in `pkg/chaos/*/{netem,iptables,stress}.go` (e.g. `netemClient = Lister + Netem`) — preserve and extend
- Test seam pattern (`var newDockerClient = docker.NewClient`) in `cmd/main.go` — preserve
- `context.WithoutCancel` for sidecar cleanup — preserve
- Mockery `EXPECT()` style with `container.NewMockClient(t)` — preserve

Dependencies identified:

- Go 1.26 (generics available)
- `github.com/urfave/cli` v1 (deprecated; isolated by Task 6)
- `github.com/stretchr/testify` (assert + require)
- `github.com/vektra/mockery` (regenerate via `make mocks`)

## Development Approach

- **Testing approach**: Regular (refactor first, update tests in same task)
- Complete each task fully before moving to the next
- Make small, focused changes — prefer mechanical edits over creative restructuring
- **CRITICAL: every task MUST include new/updated tests** for code changes in that task
  - Unit tests for new functions / new abstractions
  - Update tests for changed signatures
  - Regenerate mocks via `make mocks` after every interface change
  - Tests cover both success and error scenarios
- **CRITICAL: all tests must pass before starting next task** — no exceptions
- **CRITICAL: update this plan file when scope changes during implementation**
- Run `make test` after each task. Run `make lint` before marking task done.
- Maintain backward compatibility for the CLI: no flag rename, no command rename, no behavior change.

## Testing Strategy

- **Unit tests**: required for every task. Run with `CGO_ENABLED=0 go test ./...` for fast inner loop, `make test` (race detector, requires CGO) before declaring task done.
- **Mocks**: `make mocks` regenerates after each interface signature change. Never hand-edit `mock_*.go` or `mocks/*.go`.
- **Integration tests**: bats tests in `tests/*.bats` — must still pass post-refactor since CLI surface is unchanged. Run only after Task 7 (verification).
  - Docker: `docker run --rm -v /var/run/docker.sock:/var/run/docker.sock --entrypoint bats pumba:test tests/*.bats`
  - Containerd: `colima ssh -- sudo bats tests/containerd_*.bats`
  - Podman: `podman machine ssh sudo bats tests/podman_*.bats`
- **No new behavioral tests** required — this is a refactor. Existing test coverage is the safety net.

## Progress Tracking

- Mark completed items with `[x]` immediately when done
- Add newly discovered tasks with ➕ prefix
- Document issues/blockers with ⚠️ prefix
- Update plan if implementation deviates from original scope

## Solution Overview

Six surgical refactors, ordered so each one removes a constraint that simplifies the next:

1. **Rename `pkg/chaos/docker` → `pkg/chaos/lifecycle`** (cosmetic, unblocks naming clarity)
2. **Remove `chaos.DockerClient` global, inject runtime via constructor** (unblocks Task 3 and `t.Parallel`)
3. **Extract generic `NewAction[P]` CLI builder** (kills 17 × 80 LOC of duplication, made cheaper by Task 2)
4. **Split `pkg/runtime/docker/docker.go` into 8 per-concern files** (no API change, pure cut-and-paste)
5. **Replace 11–12 positional args with `NetemRequest`/`IPTablesRequest` value objects** (de-leaks Docker concepts, narrowest interfaces)
6. **Wrap `urfave/cli` v1 behind `Flags` interface** (provider-isolation for the day v1 lands a CVE)

## Technical Details

### Task 2: Constructor injection shape

```go
// pkg/chaos/runtime.go (new)
type RuntimeFactory func() container.Client

// pkg/chaos/<sub>/cmd/<action>.go
func NewStopCLICommand(ctx context.Context, runtime RuntimeFactory) *cli.Command { ... }
```

`cmd/main.go` constructs the runtime once, wraps it in a closure, and passes the same `RuntimeFactory` to every `NewXxxCLICommand`. The `chaos.DockerClient` package var is deleted.

### Task 3: Generic action builder

```go
// pkg/chaos/cli/builder.go (new)
type ParamParser[P any]    func(c *cli.Context, gp *chaos.GlobalParams) (P, error)
type CommandFactory[P any] func(client container.Client, gp *chaos.GlobalParams, p P) (chaos.Command, error)

func NewAction[P any](
    ctx     context.Context,
    runtime RuntimeFactory,
    name    string,
    usage   string,
    flags   []cli.Flag,
    parse   ParamParser[P],
    build   CommandFactory[P],
) *cli.Command
```

Each subcommand file shrinks to: flag list + `parse` function + `build` line — roughly 30 LOC total. The per-file `*Context` struct disappears.

### Task 4: docker.go split (file-per-responsibility)

```
pkg/runtime/docker/
  client.go        — NewClient, NewAPIClient, NewFromAPI, struct, Close
  inspect.go       — dockerInspectToContainer, ListContainers
  lifecycle.go     — Start/Stop/Kill/Restart/Remove/Pause/Unpause/StopContainerWithID
  exec.go          — ExecContainer, execOnContainer, runExecAttached
  netem.go         — NetemContainer, StopNetemContainer, tcContainerCommands, removeSidecar
  iptables.go      — IPTablesContainer, StopIPTablesContainer, ipTablesContainerCommands
  stress.go        — StressContainer, cgroup driver detection, defaultCgroupParent, stressContainerConfig
  pull.go          — pullImage, imagePullResponse
  http_client.go   — (unchanged)
```

Pure cut-and-paste — no method signatures change, no new types introduced. `dockerClient` struct definition stays in `client.go`; receivers move with their methods.

### Task 5: Value-object request shape

```go
// pkg/container/netem.go (new)
type NetemRequest struct {
    Container *Container
    Interface string
    Command   []string
    IPs       []*net.IPNet
    SPorts    []string
    DPorts    []string
    Duration  time.Duration
    Sidecar   SidecarSpec  // implementation hint, runtime may ignore
    DryRun    bool
}

type SidecarSpec struct {
    Image string
    Pull  bool
}

// pkg/container/iptables.go (new) — same shape with IPTablesRequest

// pkg/container/client.go
type Netem interface {
    NetemContainer(context.Context, NetemRequest) error
    StopNetemContainer(context.Context, NetemRequest) error
}
type IPTables interface {
    IPTablesContainer(context.Context, IPTablesRequest) error
    StopIPTablesContainer(context.Context, IPTablesRequest) error
}
```

Three runtime impls (`docker`, `containerd`, `podman`) and all callers in `pkg/chaos/{netem,iptables}` rewrite to construct/consume the request struct. Mocks regenerated. The `tcimg`/`pull`-as-positional-args leak is gone.

### Task 6: Flags adapter

```go
// pkg/chaos/cliflags/flags.go (new)
type Flags interface {
    String(name string) string
    Bool(name string) bool
    Duration(name string) time.Duration
    Int(name string) int
    Args() []string
    Global() Flags
}

// adapter for v1
type v1Flags struct{ c *cli.Context }
func (v v1Flags) String(n string) string { return v.c.String(n) }
// ... etc

func From(c *cli.Context) Flags { return v1Flags{c: c} }
```

Each subcommand `parse` function (introduced in Task 3) takes `Flags` instead of `*cli.Context`. Production wires `cliflags.From(c)`. Migration to v3 later: rewrite one file.

## What Goes Where

- **Implementation Steps** — every refactor task with code + tests
- **Post-Completion** — bats integration test rerun, mock-generation verification, score-table re-baseline (optional schedule)

## Implementation Steps

### Task 1: Rename `pkg/chaos/docker` → `pkg/chaos/lifecycle`

**Files:**

- Modify (rename dir): `pkg/chaos/docker/` → `pkg/chaos/lifecycle/`
- Modify (rename dir): `pkg/chaos/docker/cmd/` → `pkg/chaos/lifecycle/cmd/`
- Modify: every importer of `github.com/alexei-led/pumba/pkg/chaos/docker` (find via `git grep`)
- Modify: every importer of `github.com/alexei-led/pumba/pkg/chaos/docker/cmd`

- [ ] `git mv pkg/chaos/docker pkg/chaos/lifecycle`
- [ ] update package declaration in all moved files: `package docker` → `package lifecycle`
- [ ] update package declaration in `cmd/` subdir: keep `package cmd`
- [ ] grep-and-replace import path `pkg/chaos/docker` → `pkg/chaos/lifecycle` repo-wide (excluding `pkg/runtime/docker`)
- [ ] update existing tests — only import paths change, no logic
- [ ] `make lint` — must pass (catches missed imports)
- [ ] `make test` — must pass before next task

### Task 2: Remove `chaos.DockerClient` global, inject runtime via constructor

**Files:**

- Create: `pkg/chaos/runtime.go` (defines `RuntimeFactory` type)
- Modify: `pkg/chaos/command.go` (delete the `DockerClient` var + TODO)
- Modify: `cmd/main.go` (build factory closure, pass to every CLI command constructor)
- Modify: all 17 CLI builders in `pkg/chaos/{lifecycle,netem,iptables,stress}/cmd/*.go`
- Modify: existing tests for those builders + `pkg/chaos/command_test.go`

- [ ] add `pkg/chaos/runtime.go` with `type RuntimeFactory func() container.Client`
- [ ] delete `DockerClient` var from `pkg/chaos/command.go`
- [ ] update `cmd/main.go` `before()` to build the factory closure once and pass to every `New*CLICommand`
- [ ] update each `NewXxxCLICommand` signature to accept `runtime chaos.RuntimeFactory`
- [ ] replace every `chaos.DockerClient` read with `runtime()` inside the Action
- [ ] update `pkg/chaos/command_test.go` if it touches the deleted var
- [ ] update each `*_test.go` next to a renamed CLI builder (signature change)
- [ ] `make lint` — must pass
- [ ] `make test` — must pass before next task

### Task 3: Extract generic `NewAction[P]` CLI builder

**Files:**

- Create: `pkg/chaos/cli/builder.go` (`NewAction[P]` + `ParamParser[P]` + `CommandFactory[P]`)
- Create: `pkg/chaos/cli/builder_test.go`
- Modify: all 17 CLI builders in `pkg/chaos/{lifecycle,netem,iptables,stress}/cmd/*.go` — collapse `*Context` struct + duplicated boilerplate, keep flag list + parse fn + build call only
- Modify: `pkg/chaos/{lifecycle,netem,iptables,stress}/cmd/*_test.go`

- [ ] write `pkg/chaos/cli/builder.go` with `NewAction[P]`, `ParamParser[P]`, `CommandFactory[P]`
- [ ] write `pkg/chaos/cli/builder_test.go` — table-driven cases for happy path, parse error, build error, run error
- [ ] convert `pkg/chaos/lifecycle/cmd/stop.go` to use `NewAction[stopParams]` (pilot — confirm shape)
- [ ] convert remaining 5 lifecycle subcommands (`kill`, `restart`, `pause`, `remove`, `exec`)
- [ ] convert 8 netem subcommands (`delay`, `loss`, `loss_ge`, `loss_state`, `rate`, `corrupt`, `duplicate`, plus `netem.go` parent)
- [ ] convert iptables `loss.go` + parent `iptables.go`
- [ ] convert `stress/cmd/stress.go`
- [ ] delete `//nolint:dupl` annotations now that the duplication is gone
- [ ] update each subcommand's `*_test.go` — expectations stay the same, constructor invocation changes
- [ ] `make lint` — must pass (no `dupl` complaints, no unused imports)
- [ ] `make test` — must pass before next task

### Task 4: Split `pkg/runtime/docker/docker.go` into per-concern files

**Files:**

- Modify (split): `pkg/runtime/docker/docker.go` → `client.go`, `inspect.go`, `lifecycle.go`, `exec.go`, `netem.go`, `iptables.go`, `stress.go`, `pull.go`
- Modify (split): `pkg/runtime/docker/docker_test.go` → mirror per-file test files
- Keep unchanged: `pkg/runtime/docker/http_client.go`, `dockerinspect_test.go`, `mock_conn.go`, `mockengine_responses.go`, `stress_test.go`

- [ ] create `pkg/runtime/docker/client.go` — move `dockerClient` struct, `NewClient`, `NewAPIClient`, `NewFromAPI`, `Close`
- [ ] create `pkg/runtime/docker/inspect.go` — move `dockerInspectToContainer`, `ListContainers`
- [ ] create `pkg/runtime/docker/lifecycle.go` — move `Start`/`Stop`/`Kill`/`Restart`/`Remove`/`Pause`/`Unpause`/`StopContainerWithID`
- [ ] create `pkg/runtime/docker/exec.go` — move `ExecContainer`, `execOnContainer`, `runExecAttached`
- [ ] create `pkg/runtime/docker/netem.go` — move `NetemContainer`, `StopNetemContainer`, `tcContainerCommands`, `removeSidecar`
- [ ] create `pkg/runtime/docker/iptables.go` — move `IPTablesContainer`, `StopIPTablesContainer`, `ipTablesContainerCommands`
- [ ] create `pkg/runtime/docker/stress.go` — move `StressContainer`, `cgroupDriver`, `containerLeafCgroup`, `defaultCgroupParent`, `inspectCgroupParent`, `stressResolveDriver`, `stressContainerConfig`, `stressContainerCommand`
- [ ] create `pkg/runtime/docker/pull.go` — move `pullImage`, `imagePullResponse`
- [ ] delete `pkg/runtime/docker/docker.go` (now empty)
- [ ] reassign `docker_test.go` cases to matching per-concern test files (`netem_test.go`, `iptables_test.go`, etc.) — leave `docker_test.go` only if any cross-concern setup remains
- [ ] consolidate the parallel `tcContainerCommands` / `ipTablesContainerCommands` duplication into one `sidecarExec` helper in `netem.go` (now safe — both consumers in the same package, prior `//nolint:dupl` was only justified by their being in one file)
- [ ] `make lint` — must pass (no unused imports, no orphaned helpers)
- [ ] `make test` — must pass before next task

### Task 5: `NetemRequest` + `IPTablesRequest` value objects

**Files:**

- Create: `pkg/container/netem.go` (`NetemRequest` + `SidecarSpec`)
- Create: `pkg/container/iptables.go` (`IPTablesRequest`)
- Modify: `pkg/container/client.go` (interface signatures shrink to one `context.Context` + one request struct)
- Modify: `pkg/runtime/docker/netem.go`, `pkg/runtime/docker/iptables.go` (consume request struct)
- Modify: `pkg/runtime/containerd/client.go` and any netem/iptables impls in `pkg/runtime/containerd/`
- Modify: `pkg/runtime/podman/` (Podman embeds Docker — verify override-by-shadow still works on new signature)
- Modify: `pkg/chaos/netem/netem.go`, `pkg/chaos/iptables/iptables.go` (build the request, pass to interface)
- Modify: every `pkg/chaos/{netem,iptables}/cmd/*.go` parse function (still constructs the same flag values, now packs into request)
- Regenerate: `pkg/container/mock_Netem.go`, `pkg/container/mock_IPTables.go`, `pkg/container/mock_Client.go`
- Modify: every `*_test.go` that uses `EXPECT().NetemContainer(...)` / `EXPECT().IPTablesContainer(...)`

- [ ] add `pkg/container/netem.go` with `NetemRequest`, `SidecarSpec`
- [ ] add `pkg/container/iptables.go` with `IPTablesRequest`
- [ ] update `Netem` and `IPTables` interfaces in `pkg/container/client.go`
- [ ] `make mocks` — regenerate
- [ ] update `pkg/runtime/docker/netem.go` to accept `NetemRequest` (unpack to existing internal vars; minimal body change)
- [ ] update `pkg/runtime/docker/iptables.go` to accept `IPTablesRequest`
- [ ] update `pkg/runtime/containerd/` netem + iptables impls
- [ ] verify `pkg/runtime/podman/` override surface (no public surface change there since Podman delegates to Docker for netem/iptables — confirm nothing breaks)
- [ ] update `pkg/chaos/netem/netem.go` to construct `NetemRequest` from existing fields and pass to interface
- [ ] update `pkg/chaos/iptables/iptables.go` similarly
- [ ] update each `pkg/chaos/{netem,iptables}/cmd/*.go` parse function — flag values flow into request struct
- [ ] update every `EXPECT().NetemContainer(mock.Anything, mock.AnythingOfType("container.NetemRequest"))` style call in tests — test bodies stay the same logic, mock matcher changes
- [ ] update test docs in `CLAUDE.md` if the typed-nil gotchas shift (the `[]string(nil)` / `[]*net.IPNet(nil)` traps move into the struct)
- [ ] `make lint` — must pass
- [ ] `make test` — must pass before next task

### Task 6: Wrap `urfave/cli` v1 behind `Flags` interface

**Files:**

- Create: `pkg/chaos/cliflags/flags.go` (`Flags` interface + `v1Flags` adapter + `From(*cli.Context) Flags`)
- Create: `pkg/chaos/cliflags/flags_test.go`
- Modify: `pkg/chaos/cli/builder.go` — `ParamParser[P]` takes `cliflags.Flags`, not `*cli.Context`
- Modify: every parse function in `pkg/chaos/{lifecycle,netem,iptables,stress}/cmd/*.go` to take `cliflags.Flags`
- Modify: `pkg/chaos/command.go` — `ParseGlobalParams` takes `cliflags.Flags`

- [ ] write `pkg/chaos/cliflags/flags.go` — interface, `v1Flags` adapter wrapping `*cli.Context`, `From()` constructor, `Global()` returning a wrapped global view
- [ ] write `pkg/chaos/cliflags/flags_test.go` — verify each adapter method delegates correctly with a stub `*cli.Context` (or via fake `cli.App` invocation)
- [ ] modify `pkg/chaos/cli/builder.go` — `NewAction[P]` constructs `cliflags.From(c)` once, passes to `parse`
- [ ] modify `pkg/chaos/command.go` `ParseGlobalParams` to take `cliflags.Flags`
- [ ] modify every parse function in the 17 CLI builders — drop `*cli.Context` param, take `cliflags.Flags`
- [ ] update parse-function tests if they call directly (most go through `NewAction`)
- [ ] grep verify `*cli.Context` is now imported only in `cmd/main.go`, `pkg/chaos/cli/builder.go`, `pkg/chaos/cliflags/flags.go`
- [ ] `make lint` — must pass
- [ ] `make test` — must pass before next task

### Task 7: Verify acceptance criteria

- [ ] verify `git grep -n 'chaos.DockerClient'` returns zero results
- [ ] verify `git grep -n '//nolint:dupl' pkg/chaos/` returns zero results
- [ ] verify `wc -l pkg/runtime/docker/*.go` — each file ≤ 350 LOC
- [ ] verify `Netem` and `IPTables` interface methods take exactly `(context.Context, <Request>)` — two args max
- [ ] verify `git grep -n 'urfave/cli' pkg/chaos/{lifecycle,netem,iptables,stress}` returns zero results outside `cliflags/`
- [ ] run `make test` — full unit test suite passes
- [ ] run `make test-coverage` — coverage stays ≥ baseline
- [ ] run `make lint` — clean
- [ ] run `make build` — binary builds
- [ ] manual smoke: `pumba --help`, `pumba kill --help`, `pumba netem delay --help` — no behavior change
- [ ] run bats integration tests (Docker, containerd, Podman) — all pass

### Task 8: Update documentation

- [ ] update `CLAUDE.md` Architecture section: `pkg/chaos/docker` → `pkg/chaos/lifecycle`; `chaos.DockerClient` removed; `NetemRequest`/`IPTablesRequest` mentioned
- [ ] update `CLAUDE.md` Code Conventions: mockery `EXPECT().NetemContainer(ctx, container.NetemRequest{...})` example
- [ ] update `CLAUDE.md` Gotchas if typed-nil traps shifted (likely simpler now — the request struct holds nil slices uniformly)
- [ ] regenerate `docs/modularity-review/` for 2026-04-30 (or schedule for later) to confirm score moved 7.4 → ~9.0
- [ ] move this plan to `docs/plans/completed/`

## Post-Completion

**Manual verification:**

- Run a full bats integration sweep on each runtime (Docker, containerd, Podman) on a real host before tagging a release. The refactor is non-behavioral but the integration tests are the only check that interface migrations didn't break runtime wiring.
- Smoke `pumba` against a live Docker daemon for kill / netem / iptables / stress — confirm no flag regressions.

**External system updates:**

- None. Pumba is a CLI binary, not a Go library; no downstream importers.

**Optional follow-up:**

- Schedule a `/loop` agent in 2 weeks to re-baseline the modularity review and confirm the 7.4 → ~9.0 score change held after the refactor PRs landed.
