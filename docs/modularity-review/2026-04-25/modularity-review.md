# Modularity Review

**Scope**: Pumba codebase — `cmd/` + `pkg/` (~12k LOC Go), excluding generated mocks and bats integration tests.
**Date**: 2026-04-25
**Goal stated by reviewer**: improve [modularity](https://coupling.dev/posts/core-concepts/modularity/) so AI agents can change Pumba with smaller context windows, narrower mocks, and more predictable change blast radius.

## Executive Summary

Pumba is a chaos-engineering CLI for container runtimes: it injects faults (kill, stop, exec, network emulation, iptables, stress-ng) into Docker, containerd, or Podman targets via runtime-agnostic action commands. Overall [modularity](https://coupling.dev/posts/core-concepts/modularity/) is **healthy at the core seam** — chaos action packages depend on narrow consumer-side sub-interfaces (`Lister + Netem`, `Lister + IPTables`, `Lister + Stressor`) that mock cleanly, and the Podman client demonstrates a textbook `apiBackend` test seam. The two systemic weaknesses are (1) a **package-level mutable global** `chaos.DockerClient` that creates implicit cross-package [coupling](https://coupling.dev/posts/core-concepts/coupling/) and forces every CLI builder to bind to a singleton, and (2) **leaky runtime-agnostic interfaces** (`NetemContainer`, `IPTablesContainer`) whose 11–12 positional parameters bake Docker-specific concepts (`tcimg`, `pull`) into a contract every runtime must implement. Beyond those, `pkg/runtime/docker/docker.go` is a 1,289-LOC monolith mixing seven responsibilities, and the per-action CLI builders carry ~80 lines of duplicated boilerplate each (already lint-suppressed with `//nolint:dupl`). The project sits at roughly **7.4 / 10** on AI-agent maintainability — solid bones, three high-leverage repairs.

## Modularity Score Table (0–10 per dimension)

Dimensions chosen for **AI-agent maintainability**: each dimension translates directly to context window size, mock-isolation cost, or change blast radius.

| Component                                                 | Module size¹ | Interface narrowness² | DI / inversion³ | Abstraction leakage⁴ | Testability⁵ | Cohesion⁶ | No globals⁷ | DRY⁸ | Boundary clarity⁹ | Doc density¹⁰ | **Avg** |
| --------------------------------------------------------- | -----------: | --------------------: | --------------: | -------------------: | -----------: | --------: | ----------: | ---: | ----------------: | ------------: | ------: |
| `pkg/container` (model + interfaces)                      |            9 |                     9 |              10 |                    9 |            9 |         9 |          10 |   10 |                 8 |             8 | **9.1** |
| `pkg/chaos` (Command base + scheduler)                    |            6 |                     7 |               3 |                    8 |            7 |         7 |           2 |    9 |                 7 |             8 | **6.4** |
| `pkg/chaos/{docker,netem,iptables,stress}` (action logic) |            8 |                     9 |               9 |                    7 |            9 |         8 |           9 |    7 |                 9 |             7 | **8.2** |
| `pkg/chaos/*/cmd` (CLI builders)                          |            7 |                     6 |               4 |                    6 |            5 |         6 |           3 |    4 |                 7 |             7 | **5.5** |
| `pkg/runtime/docker` (one 1.3k-LOC file)                  |            4 |                     8 |               8 |                    5 |            7 |         5 |           9 |    7 |                 5 |             7 | **6.5** |
| `pkg/runtime/containerd`                                  |            8 |                     8 |               8 |                    7 |            8 |         8 |           9 |    8 |                 8 |             7 | **7.9** |
| `pkg/runtime/podman` (model: embed + override)            |            8 |                     9 |               9 |                    8 |            9 |         8 |           9 |    8 |                 8 |             8 | **8.4** |
| `cmd/main.go` (CLI entry, runtime select, TLS)            |            6 |                   n/a |               8 |                    7 |            6 |         6 |           7 |    8 |                 7 |             7 | **6.9** |
| **Project weighted average**                              |              |                       |                 |                      |              |           |             |      |                   |               | **7.4** |

¹ **Module size / cognitive load** — 10 = ≤ 200 LOC focused; 5 = ~500 LOC; 0 = 2k+ LOC mixed-concern.
² **Interface narrowness** — does the consumer import only what it uses? 10 = consumer-side sub-interfaces; 0 = fat `Client`.
³ **DI / inversion** — dependencies via constructor or globals/init? 10 = constructor; 0 = singletons.
⁴ **Abstraction leakage** — does the abstraction expose runtime-specific concepts? 10 = clean; 0 = pass-through.
⁵ **Testability** — can a unit test run without a real socket? 10 = yes (mockery + narrow iface); 0 = needs Docker.
⁶ **Cohesion** — single reason to change per file/package? 10 = yes; 0 = grab-bag.
⁷ **No globals** — absence of mutable package-level state read across packages. 10 = none; 0 = pervasive.
⁸ **DRY** — 10 = no copy-paste; 0 = `//nolint:dupl` everywhere.
⁹ **Boundary clarity** — package name matches contract. 10 = self-evident; 0 = misleading.
¹⁰ **Doc density** — godoc on exported symbols + non-obvious WHY-comments.

## Coupling Overview Table

| Integration                                                                                            | [Strength](https://coupling.dev/posts/dimensions-of-coupling/integration-strength/)                                                             | [Distance](https://coupling.dev/posts/dimensions-of-coupling/distance/) | [Volatility](https://coupling.dev/posts/dimensions-of-coupling/volatility/) | [Balanced?](https://coupling.dev/posts/core-concepts/balance/)                                            |
| ------------------------------------------------------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------- | --------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------- |
| `pkg/chaos/*/cmd` → `pkg/chaos.DockerClient` global                                                    | [Intrusive](https://coupling.dev/posts/dimensions-of-coupling/integration-strength/) (mutable shared state, no contract)                        | Same module, sync                                                       | High (every new runtime + every test)                                       | **No — unbalanced and volatile**                                                                          |
| Chaos action (`netem.Command`) → `container.Netem` sub-interface                                       | [Contract](https://coupling.dev/posts/dimensions-of-coupling/integration-strength/)                                                             | Same module, sync                                                       | Medium                                                                      | Yes                                                                                                       |
| `container.Netem` / `container.IPTables` → runtime impls                                               | [Model](https://coupling.dev/posts/dimensions-of-coupling/integration-strength/) (leaks `tcimg`, `pull` Docker concepts; 11–12 positional args) | Same module, sync                                                       | High (every Podman / containerd quirk)                                      | **No — Docker-shaped contract for non-Docker runtimes**                                                   |
| `pkg/runtime/podman.podmanClient` → `pkg/runtime/docker.dockerClient` (struct embedding)               | [Intrusive](https://coupling.dev/posts/dimensions-of-coupling/integration-strength/) (override-by-shadow on embedded interface)                 | Same module, sync                                                       | High (Podman drift)                                                         | Tolerable — distance is short, but track                                                                  |
| `pkg/runtime/docker/docker.go` internal cohesion                                                       | [Functional](https://coupling.dev/posts/dimensions-of-coupling/integration-strength/) (1,289 LOC, ≥7 responsibilities one file)                 | Same file                                                               | High                                                                        | **No — low cohesion in hot-volatility code**                                                              |
| `pkg/chaos/*/cmd/*.go` per-subcommand boilerplate (8 netem + 5 docker + iptables/stress, each ~80 LOC) | [Functional](https://coupling.dev/posts/dimensions-of-coupling/integration-strength/)                                                           | Same package                                                            | Low (CLI flags rarely change once shipped)                                  | Tolerable — volatility low                                                                                |
| `cmd/main.go` → runtime factories (`var newDockerClient = docker.NewClient`, etc.)                     | [Contract](https://coupling.dev/posts/dimensions-of-coupling/integration-strength/) (function-typed seams)                                      | Same module, sync                                                       | Medium                                                                      | Yes                                                                                                       |
| Public CLI ↔ `urfave/cli v1`                                                                           | [Model](https://coupling.dev/posts/dimensions-of-coupling/integration-strength/) (flag definitions tied to v1 API)                              | Library-level, sync                                                     | Functional: low. Provider: high (v1 unmaintained)                           | **No — [generic-subdomain](https://coupling.dev/posts/dimensions-of-coupling/volatility/) provider risk** |
| Runtime impls → Docker SDK / containerd v2 SDK                                                         | [Model](https://coupling.dev/posts/dimensions-of-coupling/integration-strength/)                                                                | Library-level, sync                                                     | Medium                                                                      | Yes (concentrated inside `pkg/runtime/*`)                                                                 |

## Issues

## Issue 1: Mutable package-level global `chaos.DockerClient`

**Integration**: `pkg/chaos/*/cmd/*` → `pkg/chaos.DockerClient`
**Severity**: **Critical**

### Knowledge Leakage

`pkg/chaos/command.go:22` declares:

```go
var (
    // DockerClient Docker client instance
    // TODO(Phase 4): remove this global and inject client via dependency injection
    DockerClient container.Client
)
```

It is written exactly once in `cmd/main.go:143` (`chaos.DockerClient = client`) and read across **17 callsites** in `pkg/chaos/{docker,netem,iptables,stress}/cmd/*.go` (e.g. `pkg/chaos/docker/cmd/stop.go:72`, `pkg/chaos/netem/cmd/delay.go:73`). Every CLI builder pulls a singleton out of thin air; nothing in the function signature reveals the dependency. This is [intrusive coupling](https://coupling.dev/posts/dimensions-of-coupling/integration-strength/) — the cmd packages reach into another package's mutable state.

The `// TODO(Phase 4)` comment confirms the maintainer already classified this as accidental.

### Complexity Impact

- **Implicit dependency, hidden in package init order.** An AI agent reading `NewStopCLICommand` cannot tell what runtime client it binds to without grepping. The dependency is invisible at the constructor's call boundary.
- **Untestable in isolation.** A test for `pkg/chaos/docker/cmd/stop.go` cannot supply a mock without touching `pkg/chaos`'s package-level state — every test mutates the same global, blocking parallelism (`t.Parallel()`).
- **One process, one runtime.** Multi-runtime use cases (e.g. fault-inject containerd while observing the Docker daemon) are foreclosed by construction.

This stretches the [4±1 working-memory budget](https://coupling.dev/posts/core-concepts/complexity/): an agent reasoning about a chaos CLI must hold (a) the cli flag schema, (b) the `chaos.GlobalParams`, (c) the runtime selection logic in `before()`, and (d) the implicit fact that the global was already populated by the time `Action` fires. That last item should not exist.

### Cascading Changes

- Adding a new runtime requires writing `chaos.DockerClient = newRuntime(...)` somewhere in `before()`. Misnaming aside (the global is called `DockerClient` even though containerd and podman are assigned to it), the wiring is one line, but every test scenario for cmd/ files that wants to validate the wiring must also poke the global.
- Migrating any single CLI builder to constructor injection forces all 17 to migrate together, because they share the global through the same package boundary.

### Recommended Improvement

Replace the global with **constructor injection plus a CLI-context value**. Two concrete shapes:

**Option A — pass through `*cli.Context` Metadata** (smallest diff):

```go
// cmd/main.go
app.Metadata = map[string]any{"runtime": client}

// pkg/chaos/command.go
func RuntimeFrom(c *cli.Context) container.Client {
    return c.App.Metadata["runtime"].(container.Client)
}

// pkg/chaos/docker/cmd/stop.go
client := chaos.RuntimeFrom(c)
stopCommand := docker.NewStopCommand(client, params, restart, duration, waitTime, limit)
```

**Option B — pass a factory closure into the CLI command constructors** (most explicit):

```go
type Runtime func() container.Client

func NewStopCLICommand(ctx context.Context, runtime Runtime) *cli.Command { ... }
```

Either eliminates the global entirely. Option A is a surgical change (drop the global, replace 17 call sites, no constructor signature changes). Option B exposes the dependency in every signature, which is what an AI agent actually needs to read.

**Trade-off**: Option A keeps `*cli.Context` as a service locator (a smell, but a smaller one — at least it's a parameter, not a global). Option B is cleaner but rewrites 17 constructor signatures plus their tests. Recommend **Option B** for AI-agent maintainability — readability of constructor signatures is exactly the property the user is optimizing for.

---

## Issue 2: Runtime-agnostic chaos interfaces leak Docker-specific concepts

**Integration**: `container.Netem` / `container.IPTables` → all three runtime implementations
**Severity**: **Critical**

### Knowledge Leakage

The contract in `pkg/container/client.go:38–47`:

```go
type Netem interface {
    NetemContainer(ctx, *Container, netInterface string, netemCmd []string,
        ips []*net.IPNet, sports, dports []string, duration time.Duration,
        tcimg string, pull, dryrun bool) error
    StopNetemContainer(...)  // 9 args
}

type IPTables interface {
    IPTablesContainer(...)  // 12 args including img string, pull bool
    StopIPTablesContainer(...)
}
```

Two leaks:

1. **`tcimg` and `pull` are Docker concepts.** The chaos command (`pkg/chaos/netem/netem.go:90`) hands the runtime an OCI image reference and a "pull-or-not" flag. Containerd's implementation must invent a pull semantic; podman's reuses Docker's. The chaos _layer_ should not know whether the runtime needs a sidecar image at all — that's a runtime-specific implementation detail of how netem gets executed.
2. **Primitive obsession.** 11 positional args, mostly stringly-typed (`netInterface string`, `sports []string`, `dports []string` instead of `[]int`). The signature is so wide that every implementation has to defensively recheck argument order, and every test stub recapitulates the full surface.

This is [model coupling](https://coupling.dev/posts/dimensions-of-coupling/integration-strength/) — the abstraction's shape mirrors Docker's mental model and forces other runtimes to translate.

### Complexity Impact

- **Adding a runtime is more invasive than it should be.** The containerd implementation already had to ignore `pull`-style semantics that don't map cleanly. Future runtimes (CRI-O, gVisor, Kata) inherit the same vestigial parameters.
- **Cognitive load on test writers.** Every `mocks.Client.EXPECT().NetemContainer(...)` call lists 11 args; an AI agent cannot remember which slot is which and routinely produces compile errors. The `// Mock typed nils` gotcha in `CLAUDE.md` is direct evidence.
- **Parameter-order bugs are silent.** `sports []string` and `dports []string` have identical types — swapping them silently routes traffic at the wrong port. There's no compile-time guard.

### Cascading Changes

- A change to how netem chooses its sidecar image (e.g. switch from `nicolaka/netshoot` to a vendored image) requires editing every cmd builder that constructs the parameter — the image flows from CLI flag → command constructor → action → runtime, four hops.
- A new netem option (e.g. queueing discipline) means adding another positional argument to every interface implementation in three runtimes plus all mocks.

### Recommended Improvement

Replace the 11–12 positional args with a **value-object request** owned by `pkg/container`:

```go
// pkg/container/netem.go (new)
type NetemRequest struct {
    Container *Container
    Interface string
    Command   []string  // tc netem flags
    IPs       []*net.IPNet
    SPorts    []string  // or []int
    DPorts    []string
    Duration  time.Duration
    Sidecar   SidecarSpec  // implementation hint, runtime may ignore
    DryRun    bool
}

type SidecarSpec struct {
    Image string  // OCI ref; runtime may or may not honor
    Pull  bool
}

type Netem interface {
    NetemContainer(context.Context, NetemRequest) error
    StopNetemContainer(context.Context, NetemRequest) error
}
```

Two wins:

1. **`SidecarSpec` is now an explicit "implementation hint."** Containerd documents that it ignores the image field (it provisions sidecars differently); the type signature stays clean.
2. **Constructor is searchable.** A test or AI agent reads one struct, not 11 positional slots.

A second pass should consider extracting netem and iptables into a `Trafficshaper` capability that runtimes opt into — the unified `container.Client` aggregate is itself fat (see Issue 7 / minor below).

**Trade-off**: introducing struct request objects breaks every callsite and every mock. The migration is mechanical (sed-able) but large. Worth it: the resulting interfaces are 3× narrower at every consumer and 5× faster to mock.

---

## Issue 3: `pkg/runtime/docker/docker.go` is a 1,289-LOC monolith

**Integration**: internal cohesion of `pkg/runtime/docker`
**Severity**: **Significant**

### Knowledge Leakage

A single file (`pkg/runtime/docker/docker.go`) holds:

1. SDK client construction (`NewClient`, `NewAPIClient`, `NewFromAPI`)
2. Docker → runtime-agnostic conversion (`dockerInspectToContainer`)
3. Lifecycle (Start/Stop/Kill/Restart/Remove/Pause/Unpause)
4. Exec (`ExecContainer`, `execOnContainer`, `runExecAttached`)
5. tc / netem sidecar lifecycle (`tcContainerCommands`, `removeSidecar`)
6. iptables sidecar lifecycle (`ipTablesContainerCommands`)
7. Cgroup driver detection + path math (`cgroupDriver`, `containerLeafCgroup`, `defaultCgroupParent`, `inspectCgroupParent`, `stressResolveDriver`)
8. Stress-ng container build + attach + stream drain (`stressContainerConfig`, `stressContainerCommand`)
9. Image pull JSON parsing (`pullImage`, `imagePullResponse`)

The file mixes **infrastructure** (SDK glue), **domain** (chaos sidecar lifecycle), and **boundary translation** (cgroup math) into one type. This is the inverse of the [single responsibility](https://coupling.dev/posts/related-topics/module-coupling/) that `pkg/container/client.go`'s sub-interfaces were designed to enable.

### Complexity Impact

- **Reading any single concern means scrolling past the others.** AI agent context windows fill up: a one-line tweak to `removeSidecar` requires loading 1.3k lines to confirm there is no second `removeSidecar` elsewhere.
- **High implicit coupling between unrelated functions.** `tcContainerCommands` and `stressContainerCommand` share zero domain logic but live in the same file, sharing receiver, cluttering the symbol namespace.
- **Test files mirror the monolith.** `docker_test.go` becomes a parallel grab-bag.

### Cascading Changes

- A change in cgroup detection (e.g. cgroup v1 fallback) lives next to image-pull JSON parsing. They have nothing to do with each other and yet share a common test file and review burden.
- The Podman client embeds `ctr.Client` from this file (`pkg/runtime/podman/client.go:58`) — any signature change to a private method on `dockerClient` requires reasoning about whether Podman's override-by-shadow still works.

### Recommended Improvement

Split `docker.go` along the existing sub-interfaces, with one file per responsibility:

```
pkg/runtime/docker/
  client.go        — NewClient, NewAPIClient, NewFromAPI, struct, Close
  inspect.go       — dockerInspectToContainer, ListContainers
  lifecycle.go     — Start/Stop/Kill/Restart/Remove/Pause/Unpause/StopContainerWithID
  exec.go          — ExecContainer + execOnContainer + runExecAttached
  netem.go         — NetemContainer, StopNetemContainer, tc helpers, removeSidecar
  iptables.go      — IPTablesContainer + helpers (currently //nolint:dupl twin of netem)
  stress.go        — StressContainer + cgroup resolution + driver detection
  pull.go          — pullImage + imagePullResponse JSON
```

This is a pure cut-and-paste; no public surface changes. After the split, agent context per change drops to 100–250 LOC. As a follow-on, the duplicated tc/iptables sidecar plumbing (`//nolint:dupl // intentionally parallel`) becomes a single `sidecarExec` helper — the duplication is currently only acceptable because both halves live in one file.

**Trade-off**: more files in the package. Trivial cost — Go's package model means no import path changes and the file split is invisible to consumers.

---

## Issue 4: Per-subcommand CLI builder duplication

**Integration**: `pkg/chaos/{docker,netem,iptables,stress}/cmd/*.go`
**Severity**: **Significant**

### Knowledge Leakage

Files like `pkg/chaos/netem/cmd/delay.go`, `loss.go`, `loss_state.go`, `loss_ge.go`, `rate.go`, `corrupt.go`, `duplicate.go` (8 files for netem alone) and `pkg/chaos/docker/cmd/{stop,kill,restart,pause,remove,exec}.go` (5 files) follow a near-identical template:

```go
type fooContext struct{ context context.Context }

func NewFooCLICommand(ctx context.Context) *cli.Command {
    cmdContext := &fooContext{context: ctx}
    return &cli.Command{
        Name:   "foo",
        Flags:  []cli.Flag{ /* per-action flags */ },
        Usage:  "...",
        Action: cmdContext.foo,
    }
}

func (cmd *fooContext) foo(c *cli.Context) error {
    globalParams, err := chaos.ParseGlobalParams(c)
    if err != nil { ... }
    // parse params
    fooCommand, err := docker.NewFooCommand(chaos.DockerClient, globalParams, ...)
    if err != nil { ... }
    err = chaos.RunChaosCommand(cmd.context, fooCommand, globalParams)
    ...
}
```

Each file is `//nolint:dupl`-tagged because the linter correctly identifies the duplication as accidental. That suppression is technical debt acknowledged in code.

### Complexity Impact

- **Cross-file consistency drift.** A change in error-wrapping style or in `ParseGlobalParams` invocation must be replicated in 17 files. AI agents reliably miss one of them.
- **Each file is mostly noise around a 5-line core** (the `New*Command` constructor invocation). The signal-to-context ratio is low.

### Cascading Changes

- Adding a new global flag means visiting every `*Context.foo` method to thread it through.
- Renaming `chaos.DockerClient` (Issue 1) means 17 edits.

### Recommended Improvement

Extract a generic action-builder. Each subcommand becomes a small constructor:

```go
// pkg/chaos/cmd/builder.go (new)
type ParamParser[P any] func(c *cli.Context, gp *chaos.GlobalParams) (P, error)
type CommandFactory[P any] func(client container.Client, gp *chaos.GlobalParams, p P) (chaos.Command, error)

func NewAction[P any](
    ctx context.Context,
    name string,
    flags []cli.Flag,
    parse ParamParser[P],
    build CommandFactory[P],
) *cli.Command {
    return &cli.Command{
        Name:  name,
        Flags: flags,
        Action: func(c *cli.Context) error {
            gp, err := chaos.ParseGlobalParams(c)
            if err != nil { return err }
            p, err := parse(c, gp)
            if err != nil { return err }
            cmd, err := build(chaos.RuntimeFrom(c), gp, p)
            if err != nil { return err }
            return chaos.RunChaosCommand(ctx, cmd, gp)
        },
    }
}
```

Each `delay.go` shrinks to ~30 lines: flag list + parse function + factory call. Combined with Issue 1's fix, every CLI builder becomes a one-screen file an agent can read in one pass.

**Trade-off**: introduces generics (Go 1.21+ — already on 1.26 per `go.mod`, fine). Slight learning curve for contributors unfamiliar with generic builders, but the resulting code is mechanical.

---

## Issue 5: `urfave/cli v1` lock-in (deprecated provider)

**Integration**: `cmd/main.go` and every `pkg/chaos/*/cmd/*.go` ↔ `github.com/urfave/cli` v1
**Severity**: **Significant** (latent)

### Knowledge Leakage

The codebase imports `github.com/urfave/cli` (v1) — see `cmd/main.go:25` and every cmd builder. Upstream is on v3; v1 is in maintenance-only mode and will eventually stop receiving even security fixes. Every `cli.StringFlag`, `cli.Action`, `c.GlobalString(...)`, `c.GlobalBool(...)` invocation is tied to the v1 API surface.

This is a [generic-subdomain](https://coupling.dev/posts/dimensions-of-coupling/volatility/) coupling whose **functional volatility is low** (CLI flags rarely change once shipped) but whose **provider volatility is high** (v1 is unmaintained).

### Complexity Impact

This is not currently a working problem; it becomes one the first time a critical CVE lands in v1 and isn't backported.

### Cascading Changes

A v1→v3 migration would touch every flag definition file (≈ 20 files), with subtle semantic differences in `Bool` vs `BoolT`, `GlobalString` lookup, etc. Worse: pumba is also a library consumer (the `*Context` types in cmd builders are tightly bound to v1's `*cli.Context` shape).

### Recommended Improvement

Keep the dependency for now, but **isolate v1 behind a thin adapter** so the migration becomes mechanical:

```go
// pkg/chaos/cliflags/flags.go
type Flags interface {
    String(name string) string
    Bool(name string) bool
    Duration(name string) time.Duration
    Int(name string) int
    Args() []string
    Global() Flags  // wrap GlobalXxx
}
```

Every cmd builder's parser takes `Flags`, not `*cli.Context`. Migration becomes "rewrite one adapter file." This is a [contract](https://coupling.dev/posts/dimensions-of-coupling/integration-strength/) wrapper around a generic-subdomain provider — exactly the right level of abstraction for low-volatility, high-provider-risk integrations.

**Trade-off**: small upfront cost (~150 LOC adapter), low ongoing cost. Without it, the migration when it eventually happens will be a multi-day project; with it, half a day.

---

## Issue 6: Misleading package name `pkg/chaos/docker`

**Integration**: `pkg/chaos/docker` (lifecycle commands) — naming
**Severity**: **Minor**

### Knowledge Leakage

`pkg/chaos/docker/{stop,kill,pause,restart,remove,exec}.go` are the **runtime-agnostic** lifecycle chaos commands. They depend only on `container.Lifecycle` / `container.Lister` / `container.Executor` sub-interfaces — no Docker-specific imports (verified: `grep -l "github.com/docker" pkg/chaos/docker/*.go` returns nothing). Yet the package is named `docker`, predating the containerd and podman runtimes.

### Complexity Impact

An AI agent reading the package tree builds a false mental model: "there must be a `pkg/chaos/containerd` and `pkg/chaos/podman` for the other runtimes." There isn't, because none is needed. The misnomer wastes one round of investigation per agent.

### Cascading Changes

None until someone tries to add a containerd-specific chaos action and gets confused about where it goes.

### Recommended Improvement

Rename `pkg/chaos/docker` → `pkg/chaos/lifecycle` (and `pkg/chaos/docker/cmd` → `pkg/chaos/lifecycle/cmd`). The CLI subcommand names (`pumba kill`, `pumba stop`, etc.) stay; only the import path changes. No external API breakage — Pumba is a CLI, not a Go library.

**Trade-off**: a one-shot mechanical rename across ~20 files. Worth doing alongside Issue 1's fix.

---

## Notes on what's already good

These are explicit design wins worth preserving:

- **`pkg/container/client.go` decomposes `Client` into six narrow sub-interfaces** (`Lister`, `Lifecycle`, `Executor`, `Netem`, `IPTables`, `Stressor`). Every chaos action declares its own consumer-side interface composing only what it needs (`netemClient = Lister + Netem`, `stressClient = Lister + Stressor + StopContainerWithID`). This is exactly the [contract coupling](https://coupling.dev/posts/dimensions-of-coupling/integration-strength/) pattern that makes mocks small.
- **`pkg/runtime/podman` embeds `ctr.Client` and overrides only divergent methods.** The `apiBackend` interface in `client.go:25` is a clean private test seam — `*dockerapi.Client` satisfies it in production, mocks satisfy it in tests, no real socket needed.
- **`pkg/chaos/command.go`'s `Command` interface** is one method (`Run(ctx, random) error`). Maximum [contract narrowness](https://coupling.dev/posts/dimensions-of-coupling/integration-strength/).
- **Test seams in `cmd/main.go`** (`var newDockerClient = docker.NewClient`) allow main-level tests without real sockets.
- **Cleanup discipline using `context.WithoutCancel`** for sidecar removal (see `pkg/chaos/netem/netem.go`, `pkg/runtime/docker/sidecar.go:21`) is the right pattern for SIGTERM-survival of resource teardown — encoded in CLAUDE.md, applied consistently.

## Recommended order of execution

If repaired in this order, each step removes a constraint that simplifies the next:

1. **Issue 6** (rename `pkg/chaos/docker` → `pkg/chaos/lifecycle`). Cheap, no dependencies. Day-of.
2. **Issue 1** (kill the `chaos.DockerClient` global, switch to constructor injection). Unblocks Issue 4. ~1 day.
3. **Issue 4** (extract generic CLI builder). Multiplied savings after Issue 1. ~1 day.
4. **Issue 3** (split `docker.go` into per-concern files). Pure cut-and-paste. ~½ day.
5. **Issue 2** (introduce `NetemRequest` / `IPTablesRequest` value objects). Largest diff; do last. ~2 days.
6. **Issue 5** (CLI adapter). Optional; do when v1 lands a CVE.

Estimated total: **5 days of focused refactoring**. Project [modularity](https://coupling.dev/posts/core-concepts/modularity/) score moves from **7.4 → ~9.0** with no public CLI surface change.

---

_This analysis was performed using the [Balanced Coupling](https://coupling.dev) model by [Vlad Khononov](https://vladikk.com)._

---

**Resolved on 2026-04-26 by `docs/plans/completed/20260426-modularity-refactor.md`.** All six issues addressed: `pkg/chaos/docker` renamed to `pkg/chaos/lifecycle` (Issue 6); `chaos.DockerClient` global replaced by `chaos.Runtime` factory closure (Issue 1); generic `NewAction[P]` builder collapses 17 cmd files (Issue 4); `pkg/runtime/docker/docker.go` split into 10 per-concern files, all under 350 LOC (Issue 3); `Netem`/`IPTables` interfaces now take `*NetemRequest`/`*IPTablesRequest` value objects (Issue 2); `urfave/cli` v1 hidden behind `pkg/chaos/cliflags` adapter (Issue 5).
