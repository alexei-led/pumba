# Modularity Review — Post-Refactor Final Baseline

**Scope**: Pumba codebase — `cmd/` + `pkg/` (~10.1k LOC Go production files; mocks and bats integration tests excluded).
**Date**: 2026-04-27 (afternoon — re-runs the morning's mid-refactor review on post-refactor HEAD)
**Branch**: `modularity-refactor` @ `d244261`
**Goal stated by reviewer**: improve [modularity](https://coupling.dev/posts/core-concepts/modularity/) so AI agents can change Pumba with smaller context windows, narrower mocks, and predictable change blast radius.
**Compares against**: `docs/modularity-review/2026-04-25/modularity-review.md` (baseline 7.4 / 10) and the mid-refactor `docs/modularity-review/2026-04-27/` AM snapshot (8.5 / 10) — the mid-refactor flagged 5 issues that have all since landed.

## Executive Summary

The 8-task modularity refactor (`docs/plans/completed/20260427-modularity-refactor.md`) is fully merged. Every issue flagged in both prior reviews is now resolved:

- Mutable global `chaos.DockerClient` → explicit `chaos.Runtime func() container.Client` factory closure threaded through every CLI builder.
- 1,289-LOC `pkg/runtime/docker/docker.go` monolith → 11 per-concern files (max 219 LOC).
- 437-LOC `pkg/runtime/containerd/sidecar.go` → 5 per-concern files (max 222 LOC).
- `Netem` / `IPTables` / `Stressor` interfaces all take `*NetemRequest` / `*IPTablesRequest` / `*StressRequest` value objects.
- `Lifecycle.RemoveContainer` takes `RemoveOpts` instead of four naked booleans.
- 17 CLI builders share the generic `NewAction[P]` shape.
- `urfave/cli` v1 wrapped behind `pkg/chaos/cliflags.Flags` for value reads.
- `Params` ↔ `*Request` parallel data shapes inside chaos action packages: collapsed — `ParseRequestBase` in netem and iptables now constructs `*NetemRequest` / `*IPTablesRequest` directly; the per-action commands carry the request struct.

The codebase lands at **~9.0 / 10** for the [Balanced Coupling](https://coupling.dev/posts/core-concepts/balance/) lens. Five secondary findings remain — none critical, none [tightly coupled](https://coupling.dev/posts/core-concepts/balance/) by the balance rule. The most AI-agent-relevant remaining drag is **`Run()`-body [duplicated business logic](https://coupling.dev/posts/dimensions-of-coupling/integration-strength/) across 15 chaos actions** (Issue 1) — a list-then-fanout-then-collect-errors algorithm copy-pasted into every action file. Volatility is moderate (new chaos action = new copy) and [distance](https://coupling.dev/posts/dimensions-of-coupling/distance/) is not zero (15 files across 4 packages), so this is the single most impactful follow-up for AI-agent maintainability.

## AI-Agent Maintainability Lens

The dimensions below are the same set used in the 2026-04-25 baseline so per-dimension delta is directly comparable, but the meaning is grounded in what slows AI agents down on this codebase specifically:

| Dimension                | Why an AI agent cares                                                                            |
| ------------------------ | ------------------------------------------------------------------------------------------------ |
| **Module size**          | Smaller files = smaller context window per change. 10 = ≤ 200 LOC; 5 = ~500; 0 = 2k+.            |
| **Interface narrowness** | Per-consumer sub-interfaces shrink mock surface area. 10 = consumer-side; 0 = fat `Client`.      |
| **DI / inversion**       | Constructor injection reveals deps in the signature. 10 = explicit; 0 = singletons / globals.    |
| **Abstraction leakage**  | Domain vocabulary at boundaries. 10 = clean; 0 = runtime types ripple through every layer.       |
| **Testability**          | Pure-Go unit tests with mockery only. 10 = no socket needed; 0 = needs a real runtime.           |
| **Cohesion**             | One reason to change per file/package. 10 = single concern; 0 = grab-bag.                        |
| **No globals**           | Mutable package-level state read across packages. 10 = none; 0 = pervasive.                      |
| **DRY**                  | Shared algorithm extracted, no `//nolint:dupl`. 10 = extracted; 0 = copy-paste everywhere.       |
| **Boundary clarity**     | Package name predicts contract. 10 = self-evident; 0 = misleading (e.g. old `pkg/chaos/docker`). |
| **Doc density**          | Godoc on exported symbols + non-obvious **why** comments at invariants.                          |

## Modularity Score Table (0–10)

**Δ** column shows change vs the 2026-04-25 baseline.

| Component                                                               | Module size | Interface narrowness | DI / inversion | Abstraction leakage | Testability | Cohesion | No globals | DRY | Boundary clarity | Doc density | **Avg** |    **Δ** |
| ----------------------------------------------------------------------- | ----------: | -------------------: | -------------: | ------------------: | ----------: | -------: | ---------: | --: | ---------------: | ----------: | ------: | -------: |
| `pkg/container` (model + 6 narrow ifaces + request value objects)       |          10 |                   10 |             10 |                   9 |          10 |       10 |         10 |  10 |                9 |           9 | **9.7** | **+0.6** |
| `pkg/chaos` (Command + scheduler + Runtime closure)                     |          10 |                    9 |             10 |                   9 |          10 |        9 |         10 |   9 |                9 |           9 | **9.4** | **+3.0** |
| `pkg/chaos/cliflags` (urfave/cli v1 adapter)                            |          10 |                    9 |             10 |                   9 |          10 |       10 |         10 |  10 |               10 |           9 | **9.7** |  **new** |
| `pkg/chaos/cmd` (generic `NewAction[P]` builder)                        |          10 |                    9 |             10 |                   8 |           9 |       10 |         10 |  10 |                9 |           9 | **9.4** |  **new** |
| `pkg/chaos/lifecycle` (was `pkg/chaos/docker`)                          |           9 |                    9 |             10 |                   8 |           9 |        9 |         10 |   6 |               10 |           8 | **8.8** | **+0.6** |
| `pkg/chaos/{netem,iptables,stress}` (action logic)                      |           8 |                    9 |             10 |                   8 |           9 |        7 |         10 |   6 |                9 |           8 | **8.4** | **+0.2** |
| `pkg/chaos/{lifecycle,netem,iptables,stress}/cmd` (CLI builders)        |           9 |                    9 |             10 |                   8 |           9 |        9 |         10 |   8 |                9 |           8 | **8.9** | **+3.4** |
| `pkg/runtime/docker` (11 per-concern files, max 219 LOC)                |           9 |                    9 |              9 |                   8 |           9 |        9 |         10 |   9 |                9 |           8 | **8.9** | **+2.4** |
| `pkg/runtime/containerd` (split into 9 files, max 222 LOC)              |           9 |                    9 |              9 |                   8 |           9 |        9 |         10 |   8 |                9 |           8 | **8.8** | **+1.0** |
| `pkg/runtime/podman` (embed Docker delegate + override divergent paths) |           9 |                    9 |              9 |                   7 |           9 |        9 |          9 |   9 |                9 |           9 | **8.8** | **+0.4** |
| `cmd/main.go` (CLI entry, runtime select, TLS)                          |           6 |                  n/a |              9 |                   7 |           7 |        6 |          7 |   8 |                7 |           7 | **7.1** | **+0.2** |
| `pkg/util` (CIDR + port parsing)                                        |          10 |                  n/a |             10 |                  10 |          10 |       10 |         10 |  10 |               10 |           8 | **9.8** |  **n/a** |
| **Project weighted average**                                            |             |                      |                |                     |             |          |            |     |                  |             | **9.0** | **+1.6** |

### Per-dimension project totals

| Dimension            | 2026-04-25 | 2026-04-27 PM |    Δ |
| -------------------- | ---------: | ------------: | ---: |
| Module size          |        7.0 |           9.1 | +2.1 |
| Interface narrowness |        7.7 |           9.1 | +1.4 |
| DI / inversion       |        7.4 |           9.6 | +2.2 |
| Abstraction leakage  |        7.0 |           8.2 | +1.2 |
| Testability          |        7.7 |           9.2 | +1.5 |
| Cohesion             |        7.1 |           8.9 | +1.8 |
| No globals           |        7.4 |           9.6 | +2.2 |
| DRY                  |        7.6 |           8.6 | +1.0 |
| Boundary clarity     |        7.4 |           9.0 | +1.6 |
| Doc density          |        7.4 |           8.4 | +1.0 |

Largest movements: **DI/inversion (+2.2)** and **No globals (+2.2)** — directly attributable to killing `chaos.DockerClient` and routing the runtime through a `chaos.Runtime` closure. **Module size (+2.1)** reflects two monolith splits (`docker.go` and `containerd/sidecar.go`). **Cohesion (+1.8)** and **Boundary clarity (+1.6)** reflect both the per-concern file splits and the `pkg/chaos/docker` → `pkg/chaos/lifecycle` rename. **DRY (+1.0)** is the smallest gain — see Issue 1.

### Reading-list cost for common AI-agent tasks (post-refactor)

What an agent must load to safely make a change:

| Task                          | Files to load                                                                 | LOC budget |
| ----------------------------- | ----------------------------------------------------------------------------- | ---------: |
| Add a new netem action        | `netem/{netem,parse}.go` + 1 example action + matching `cmd/<action>.go`      |       ~350 |
| Add a new iptables action     | `iptables/{iptables,parse}.go` + `iptables/loss.go` + `iptables/cmd/loss.go`  |       ~430 |
| Add a new lifecycle action    | one existing `lifecycle/<action>.go` + `cmd/<action>.go` + `chaos/command.go` |       ~330 |
| Add a field to `NetemRequest` | `pkg/container/requests.go` + grep callsites in `pkg/runtime/{docker,podman}` |       ~250 |
| Add a new container runtime   | `pkg/container/{client,requests}.go` + `pkg/runtime/podman/*` (template)      |      ~1100 |
| Add a global CLI flag         | `cmd/main.go::globalFlags` + `chaos/command.go::ParseGlobalParams`            |       ~150 |
| Edit Docker netem behavior    | `pkg/runtime/docker/{netem,sidecar}.go` only                                  |       ~350 |
| Edit Podman cgroup resolution | `pkg/runtime/podman/{stress,cgroup}.go` only                                  |       ~470 |

Pre-refactor, the equivalent of the first three rows was ~700–900 LOC because the whole netem package shared one `Params` struct and the cmd builders carried duplicate boilerplate. The reading-list compression is the headline AI-agent win.

## Coupling Overview Table

| Integration                                                                             | [Strength](https://coupling.dev/posts/dimensions-of-coupling/integration-strength/)                                              | [Distance](https://coupling.dev/posts/dimensions-of-coupling/distance/) | [Volatility](https://coupling.dev/posts/dimensions-of-coupling/volatility/) | [Balanced?](https://coupling.dev/posts/core-concepts/balance/) |
| --------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------- | --------------------------------------------------------------------------- | -------------------------------------------------------------- |
| `pkg/chaos/*/cmd` → `chaos.Runtime` factory closure                                     | [Contract](https://coupling.dev/posts/dimensions-of-coupling/integration-strength/)                                              | Same module, sync                                                       | Low                                                                         | **Yes**                                                        |
| Chaos action → `container.{Netem,IPTables,Stressor,Lifecycle}` request-typed interfaces | [Contract](https://coupling.dev/posts/dimensions-of-coupling/integration-strength/)                                              | Same module, sync                                                       | Medium                                                                      | **Yes**                                                        |
| Chaos action `Run()` body algorithm (list → optional random → fanout → collect errors)  | [Functional](https://coupling.dev/posts/dimensions-of-coupling/integration-strength/) (duplicated across 15 files)               | 4 packages, 15 files                                                    | Medium (recurring on every new action)                                      | **No — Issue 1**                                               |
| `pkg/runtime/podman` ↔ `pkg/runtime/docker` (Go struct embedding)                       | [Intrusive](https://coupling.dev/posts/dimensions-of-coupling/integration-strength/) (override-by-shadow)                        | Same module, sync                                                       | Medium-high                                                                 | Tolerable — Issue 2                                            |
| `pkg/runtime/{docker,podman}` ↔ Docker SDK types                                        | [Model](https://coupling.dev/posts/dimensions-of-coupling/integration-strength/)                                                 | Library boundary, semver                                                | Medium (28.5.2 currently; SDK has bumped repeatedly)                        | Tolerable — Issue 3                                            |
| `pkg/runtime/containerd` ↔ containerd v2 SDK                                            | [Model](https://coupling.dev/posts/dimensions-of-coupling/integration-strength/)                                                 | Library boundary, semver                                                | Medium                                                                      | **Yes** (concentrated inside `pkg/runtime/containerd`)         |
| `cmd/main.go` ↔ `urfave/cli` v1 (global flag reads)                                     | [Functional](https://coupling.dev/posts/dimensions-of-coupling/integration-strength/) (bypasses cliflags adapter)                | Same package                                                            | Provider: high (v3 migration on roadmap). Functional: low                   | Tolerable — Issue 4                                            |
| `pkg/chaos/{netem,iptables}/parse.go` ↔ `reInterface` regex                             | [Functional](https://coupling.dev/posts/dimensions-of-coupling/integration-strength/) (identical regex constant in two packages) | 2 sibling packages                                                      | Low                                                                         | Tolerable — Issue 5                                            |
| `pkg/chaos` (`Re2Prefix`) ↔ all 17 CLI builders (`ArgsUsage` strings)                   | [Contract](https://coupling.dev/posts/dimensions-of-coupling/integration-strength/) (one read-only constant)                     | Same module, sync                                                       | Low                                                                         | **Yes** (constant has no better home)                          |
| Public CLI ↔ `urfave/cli` v1 (value reads via `cliflags.Flags`)                         | [Contract](https://coupling.dev/posts/dimensions-of-coupling/integration-strength/)                                              | Library, but adapter-isolated                                           | Provider: high. Functional: low                                             | **Yes** — adapter isolates v3 migration to one file            |

## Issues

### Issue 1: `Run()` body algorithm duplicated across 15 chaos action commands

**Integration**: every `*Command.Run(ctx, random)` in `pkg/chaos/{netem,iptables,lifecycle,stress}` (15 files)
**Severity**: **Significant**

#### Knowledge Leakage

Every chaos action's `Run` method follows the same shape (~30–50 lines):

```go
func (n *delayCommand) Run(ctx context.Context, random bool) error {
    log.Debug(...)
    log.WithFields(...).Debug("listing matching containers")
    containers, err := container.ListNContainers(ctx, n.client, n.gp.Names, n.gp.Pattern, n.gp.Labels, n.limit)
    if err != nil { return fmt.Errorf("error listing containers: %w", err) }
    if len(containers) == 0 { log.Warning("no containers found"); return nil }
    if random {
        if c := container.RandomContainer(containers); c != nil {
            containers = []*container.Container{c}
        }
    }
    // ... per-action logic (build command, fanout, collect errors)
}
```

The pattern is verified to repeat (verbatim or near-verbatim) in:
`pkg/chaos/netem/{corrupt,delay,duplicate,loss,loss_ge,loss_state,rate}.go`,
`pkg/chaos/iptables/loss.go`,
`pkg/chaos/lifecycle/{exec,kill,pause,remove,restart,stop}.go`,
`pkg/chaos/stress/stress.go`.

This is [implicit functional coupling](https://coupling.dev/posts/dimensions-of-coupling/integration-strength/): the same business algorithm — "list matching containers, optionally pick one at random, apply the action with fanout, collect first error" — exists in 15 places. There is no shared helper. Drift is already visible: some actions use `errgroup` (stress), some use `sync.WaitGroup` + `errs []error` slice (netem/iptables), some use a sequential `for` loop (lifecycle).

#### Complexity Impact

For an AI agent:

- **Reading**: understanding any one action means re-parsing 30 lines of boilerplate alongside the 5 lines of actual action logic. Signal-to-noise ratio is poor.
- **Writing a new action**: copy-paste a similar action and edit the per-action lines. Each copy is a fresh chance to drop a goroutine bug, swap `n.client` for `s.client`, forget to capture `i, c` in the closure, or forget the `if len(containers) == 0` guard.
- **Changing the algorithm itself** (e.g., adding a per-container timeout, parallelism cap, structured error reporting): touches 15 files, each in a slightly different shape. The existing fanout-style divergence (errgroup vs WaitGroup vs sequential) means no single edit pattern works.

This exceeds working-memory capacity (≈ 4 ± 1 chunks) — an AI must hold 15 file shapes in mind to confidently make an algorithm change without dropping one.

#### Cascading Changes

- A new chaos action (e.g. `netem reorder`) means a new copy of the boilerplate. There have been ~15 such copies; the next one is the same pattern.
- A bug in one variant (e.g. the netem `for _, err = range errs` reusing a loop variable) often re-emerges in a sibling because each was hand-written.
- Adding cross-cutting concerns (metrics, structured logging fields, per-container timeout) requires 15 edits.

#### Recommended Improvement

Extract the loop body into a single helper in `pkg/chaos`:

```go
// pkg/chaos/runner.go
type ContainerAction func(ctx context.Context, c *container.Container) error

func RunOnContainers(
    ctx context.Context,
    lister container.Lister,
    gp *GlobalParams,
    limit int,
    random bool,
    parallel bool,
    action ContainerAction,
) error {
    containers, err := container.ListNContainers(ctx, lister, gp.Names, gp.Pattern, gp.Labels, limit)
    if err != nil { return fmt.Errorf("listing: %w", err) }
    if len(containers) == 0 { log.Warning("no containers found"); return nil }
    if random {
        if c := container.RandomContainer(containers); c != nil {
            containers = []*container.Container{c}
        }
    }
    if !parallel {
        for _, c := range containers {
            if err := action(ctx, c); err != nil { return err }
        }
        return nil
    }
    var eg errgroup.Group
    for _, c := range containers {
        eg.Go(func() error { return action(ctx, c) })
    }
    return eg.Wait()
}
```

Each per-action `Run` then collapses to ~10 lines:

```go
func (n *delayCommand) Run(ctx context.Context, random bool) error {
    netemCmd := n.buildNetemCmd()
    return chaos.RunOnContainers(ctx, n.client, n.gp, n.limit, random, true,
        func(ctx context.Context, c *container.Container) error {
            req := *n.req
            req.Container = c
            req.Command = netemCmd
            return runNetem(ctx, n.client, &req)
        })
}
```

**Wins**: 15 files lose ~25 LOC each (~375 LOC removed total); algorithm changes land in one place; `errgroup` becomes the single fanout pattern; new actions become genuinely small.

**Trade-offs**: introduces a closure-passing pattern that's idiomatic Go but adds one layer of indirection. The closure's variable capture of `i` and `c` (currently a hand-rolled gotcha in netem files) becomes the runner's responsibility. Acceptable.

**Effort**: ~1 day across 15 files plus their tests. Mechanical — tests that verify NoContainers / DryRun / WithRandom paths already exist for each action and can be retargeted at the runner.

This is the **single most impactful follow-up for AI-agent maintainability**: it removes the largest remaining repeated-boilerplate footprint and reduces the per-action reading surface from ~120 LOC to ~70 LOC.

---

### Issue 2: `pkg/runtime/podman` embeds `pkg/runtime/docker` delegate

**Integration**: `pkg/runtime/podman.podmanClient` embeds `ctr.Client` returned by `docker.NewFromAPI`
**Severity**: **Minor** (tolerable, but document the invariant)

#### Knowledge Leakage

`pkg/runtime/podman/client.go:56` declares:

```go
type podmanClient struct {
    ctr.Client      // delegate from docker.NewFromAPI(api)
    api       apiBackend
    rootless  bool
    socketURI string
}
```

Methods are inherited from the docker delegate unless explicitly overridden (`NetemContainer`, `StopNetemContainer`, `IPTablesContainer`, `StopIPTablesContainer`, `Close`, `StressContainer`). This is [intrusive coupling](https://coupling.dev/posts/dimensions-of-coupling/integration-strength/) by Go-language semantics: Podman silently picks up every Docker method, every Docker bug fix, and every Docker behavior change.

#### Complexity Impact

For an AI agent reading `podmanClient.StopContainer(...)`:

- The method exists on `podmanClient` only via embedding. The agent must check the override list to confirm it's not overridden, then read `pkg/runtime/docker/lifecycle.go` to know what runs.
- A bug fix to `dockerClient.StopContainer` is silently inherited. Sometimes that's correct (Docker-compat socket genuinely accepts the same call). Sometimes it's not (Podman 4.9.x's `<scope>/container` race; rootless socket constraints; `StopSignal` defaults). The agent has to track which.
- Mock generation (`mockery`) generates one mock per interface, not per concrete type — but tests that exercise Podman's overrides still need the Docker mock under the hood. The test setup is harder to reason about than separate clients.

Distance is low (same module, sibling package), so the [balance rule](https://coupling.dev/posts/core-concepts/balance/) keeps this tolerable: high strength + low distance = high cohesion. They're meant to evolve together.

#### Cascading Changes

- A Docker SDK upgrade may silently change Podman's behavior on a method Podman doesn't override.
- Adding a new method to `ctr.Client` (e.g. a future `Snapshot` capability) automatically applies the Docker implementation to Podman — which may or may not be correct.

#### Recommended Improvement

**Don't decompose**. The alternative (explicit per-method forwarding from `podmanClient` to `dockerClient`) is more verbose with no concrete bug to fix, and it loses the embedding pattern's natural defaulting. The current shape is well-suited to "Podman is Docker-compat with three divergent paths."

What to do instead:

- **Add an invariant comment** at the top of `pkg/runtime/podman/client.go` listing which methods are explicitly overridden and why. Make the inheritance intentional rather than accidental.
- **When adding a method to `ctr.Client`**, add a Podman test that verifies the inherited Docker behavior is correct on Podman's compat socket — or override defensively. This is a process gate, not a code gate.

**Effort**: ~30 minutes (one godoc block).

---

### Issue 3: Docker SDK types vocabulary `pkg/runtime/podman`

**Integration**: `pkg/runtime/podman` directly imports `github.com/docker/docker/api/types/{container,image,network,system}`
**Severity**: **Minor**

#### Knowledge Leakage

`pkg/runtime/podman/stress.go` and `pkg/runtime/podman/client.go` use Docker SDK types as their working vocabulary:

```go
// pkg/runtime/podman/client.go:24
type apiBackend interface {
    Info(ctx context.Context) (system.Info, error)
    ContainerInspect(ctx context.Context, containerID string) (ctypes.InspectResponse, error)
    ImagePull(ctx context.Context, refStr string, options imagetypes.PullOptions) (io.ReadCloser, error)
    // ... 5 more Docker-SDK-shaped methods
}
```

```go
// pkg/runtime/podman/stress.go:200
func buildStressConfig(image string, ...) (ctypes.Config, ctypes.HostConfig) { ... }
```

This is [model coupling](https://coupling.dev/posts/dimensions-of-coupling/integration-strength/) — Docker's data model is Podman's working model. There is no anti-corruption layer. This is _structurally correct_ — Podman exposes a Docker-compatible API socket precisely so it can be programmed as Docker. But the coupling is invisible from the package boundary: nothing in `pkg/runtime/podman`'s name signals "depends on Docker SDK types."

Volatility is moderate: the Docker SDK has bumped major versions repeatedly (currently 28.5.2). Every bump can change Podman's compile surface even though no Podman semantics changed.

#### Complexity Impact

- A Docker SDK type rename (e.g. the `types/container.Config` history) requires synchronous edits in `pkg/runtime/{docker,podman}` plus the Podman test fakes.
- An AI agent doing a Docker SDK migration must know — without the package name hinting at it — that Podman is part of the blast radius.

#### Cascading Changes

- Docker SDK 30.x → adjustments in 2 packages plus mocks.
- Podman ever migrating to a native libpod SDK → requires either an [anti-corruption layer](https://coupling.dev/posts/dimensions-of-coupling/integration-strength/) (significant work) or full duplication of the runtime.

#### Recommended Improvement

**Don't introduce an anti-corruption layer pre-emptively** — Podman is committed to Docker-compat for the foreseeable future and the duplication cost is real. Two cheap mitigations:

1. **Add a package-level godoc** at `pkg/runtime/podman/doc.go` that names the Docker SDK as the working vocabulary and explains why (Podman exposes a Docker-compat socket).
2. **Re-export the relevant Docker types as type aliases inside `pkg/runtime/podman`** — e.g. `type ContainerConfig = ctypes.Config`. This lets the rest of the package use a Podman-namespaced vocabulary without runtime cost. AI agents reading Podman code see Podman names; the Docker dependency is concentrated in one alias file.

**Effort**: ~½ day for the alias indirection if desired; ~30 minutes for just the doc note.

---

### Issue 4: `cmd/main.go` bypasses the `cliflags` adapter

**Integration**: `cmd/main.go` reads `*cli.Context` directly via `c.GlobalString(...)` / `c.GlobalBool(...)`
**Severity**: **Minor**

#### Knowledge Leakage

The `cliflags.Flags` adapter (`pkg/chaos/cliflags/`) wraps `urfave/cli` v1's `*cli.Context` so that chaos cmd builders read flag values through an interface. A v3 migration becomes a one-file swap on the value-read side.

But `cmd/main.go` itself bypasses the adapter: `before(c *cli.Context)`, `createRuntimeClient(c *cli.Context)`, and `globalFlags(rootCertPath)` all use `c.GlobalString`, `c.GlobalBool`, and `cli.StringFlag{}` declarations directly. The flag _declarations_ likewise live on `cli.Flag` types throughout the chaos cmd builders — the adapter intentionally covers only value-read.

This is a partial migration. v3 day will require edits in `cmd/main.go::before`, `cmd/main.go::createRuntimeClient`, every `Spec.Flags` list across 17 builder files, plus a new `cliflags.NewV3` adapter.

#### Complexity Impact

- A v3 migration agent has three discrete edit zones (declarations across 17 files, value-reads in main.go, adapter swap) instead of one.
- Reading `cmd/main.go` requires the agent to know `c.GlobalString` is v1-specific.

#### Recommended Improvement

Defer until v3 migration is on the table. When it is:

1. Extend `cliflags.Flags` with the global-flag reads `cmd/main.go` needs (`Runtime`, `LogLevel`, `JSON`, `Slackhook`, `Slackchannel`, `DockerCertPath`, `Host`, `TLS*`).
2. Pass an adapter into `before` instead of `*cli.Context`.
3. Add `cliflags.NewV3` alongside `NewV1`; wire the v3 root in `main()`.

**Effort**: ~½ day, mechanical.

---

### Issue 5: `reInterface` regex duplicated across `netem` and `iptables`

**Integration**: identical `regexp.MustCompile` constant in `pkg/chaos/netem/parse.go:16` and `pkg/chaos/iptables/parse.go:17`
**Severity**: **Minor**

Pre-existing functional duplication; the two packages compile the same shell-injection-defense regex separately. Volatility is low (the regex hasn't changed). [Balance](https://coupling.dev/posts/core-concepts/balance/) tolerates this — same module, low strength, low volatility — but the duplication is worth eliminating opportunistically.

#### Recommended Improvement

Move to `pkg/util/util.go` as `ValidateInterfaceName(name string) error`. Both parsers call it. ~5 minutes plus tests.

---

## Notes on what's already strong

These are the structural wins that are now load-bearing for AI-agent maintainability:

- **`pkg/container/client.go` decomposes `Client` into six narrow sub-interfaces** (`Lister`, `Lifecycle`, `Executor`, `Netem`, `IPTables`, `Stressor`). Every chaos action declares its own [contract-coupled](https://coupling.dev/posts/dimensions-of-coupling/integration-strength/) consumer-side interface alias (`netemClient`, `killClient`, `stressClient`), composing only the methods it needs. Mock surface is minimal.
- **Request value objects** (`pkg/container/requests.go`): `NetemRequest`, `IPTablesRequest`, `StressRequest`, `StressResult`, `RemoveOpts`, `SidecarSpec`. Position-mistake risk eliminated; new fields land in one place.
- **`chaos.Runtime` factory closure** (`pkg/chaos/command.go:23`). Every CLI builder takes `runtime chaos.Runtime` as an explicit constructor parameter. Dependencies are visible at every call boundary.
- **Generic `NewAction[P]` builder** (`pkg/chaos/cmd/builder.go`). 17 cmd files share one shape: flag list + typed `ParamParser[P]` + `CommandFactory[P]`. Each cmd file is 50–80 LOC.
- **`cliflags.Flags` adapter** (`pkg/chaos/cliflags/`). `urfave/cli` v3 migration is a one-file swap on the value-read side of the chaos cmd builders.
- **`ParseRequestBase` shared parser** in netem (`pkg/chaos/netem/parse.go`) and iptables (`pkg/chaos/iptables/parse.go`). Per-action parsers consume it; no duplicate flag-reading logic.
- **`runSidecar` consolidation** (`pkg/runtime/docker/sidecar.go`). The previously-duplicated tc and iptables sidecar code is one helper. The exit-code check on `runSidecarExec:142` closed a silent-failure bug.
- **File-per-concern docker runtime**. 11 production files, max 219 LOC.
- **File-per-concern containerd runtime**. 9 production files, max 222 LOC. The 437-LOC `sidecar.go` monolith is gone.
- **Podman embeds Docker delegate, overrides only divergent methods.** The `apiBackend` test seam keeps unit tests off real sockets.
- **`context.WithoutCancel` cleanup discipline** propagated to both runtimes' sidecar reap paths and to chaos action cleanup. SIGTERM races on the netem/iptables/stress paths are bounded.

## Recommended order of execution

If repaired in this order:

1. **Issue 1** (`chaos.RunOnContainers` helper). ~1 day. The single most AI-agent-impactful follow-up — eliminates the largest remaining repeated-boilerplate footprint and makes future chaos actions genuinely small.
2. **Issue 5** (move `reInterface` to `pkg/util`). ~5 minutes. Bundle with anything else.
3. **Issue 2** (Podman embedding invariant doc). ~30 minutes. Process gate, not refactor.
4. **Issue 3** (Podman Docker-SDK aliasing OR doc). ~30 minutes (doc) or ~½ day (aliases). Defer until next Docker SDK bump.
5. **Issue 4** (extend cliflags to global flags). Defer to the v3 migration window.

Estimated total to push to **~9.5 / 10**: ~1.5 days, dominated by Issue 1.

The bones are now genuinely strong: a focused chaos-layer change rarely loads more than 350 LOC of context, every CLI builder declares its dependency in its constructor signature, every interface method takes a request value object, no mutable globals leak across packages, and the file-per-concern runtime split keeps any single edit under 250 LOC.

---

_This analysis was performed using the [Balanced Coupling](https://coupling.dev) model by [Vlad Khononov](https://vladikk.com)._

_Compares against `docs/modularity-review/2026-04-25/modularity-review.md` (baseline 7.4 / 10) and the morning's mid-refactor `docs/modularity-review/2026-04-27/` snapshot (8.5 / 10). The five issues that snapshot flagged have all since landed: `StressRequest`/`StressResult` value objects, `Params`-struct collapse in netem/iptables, `containerd/sidecar.go` per-concern split, docker `netem`/`iptables` internal helpers taking request structs, `RemoveOpts` for `RemoveContainer`._
