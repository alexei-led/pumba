# Modularity Review (post-refactor re-baseline)

**Scope**: Pumba codebase — `cmd/` + `pkg/` (~10.6k LOC Go, prod files only; mocks and bats tests excluded).
**Date**: 2026-04-27
**Branch**: `modularity-refactor`
**Goal stated by reviewer**: improve [modularity](https://coupling.dev/posts/core-concepts/modularity/) so AI agents can change Pumba with smaller context windows, narrower mocks, and predictable change blast radius.
**Compares against**: `docs/modularity-review/2026-04-25/modularity-review.md` (baseline 7.4 / 10).

## Executive Summary

The 8-task refactor (`docs/plans/completed/20260427-modularity-refactor.md`) closed all six findings from the 2026-04-25 baseline. The mutable global `chaos.DockerClient` was replaced by a `chaos.Runtime func() container.Client` factory closure threaded through every CLI builder. The 1,289-LOC `pkg/runtime/docker/docker.go` monolith was split into 11 per-concern files (max 219 LOC). The `Netem` and `IPTables` interfaces now take `*NetemRequest` / `*IPTablesRequest` value objects instead of 11–12 positional args. The 17 CLI builders share a generic `NewAction[P]` action shape. `urfave/cli` v1 is wrapped behind `pkg/chaos/cliflags.Flags` for value reads. The `pkg/chaos/docker` misnomer is now `pkg/chaos/lifecycle`.

The project lands at **~8.5 / 10** — short of the plan's aspirational 9.0, because the refactor stopped at the public-interface boundary and three follow-on cleanups are still warranted: (1) the `Stressor` interface still carries 8 positional args + 4 return channels with `image`/`pull`/`injectCgroup` Docker leakage — the same problem class fixed for `Netem`/`IPTables`; (2) the chaos action packages now hold **two parallel data shapes** for the same concept (a private `Params` struct _and_ the cross-package `*NetemRequest`/`*IPTablesRequest`) — a knowledge-duplication artifact created by the refactor itself, AI-agent-harmful; (3) `pkg/runtime/containerd/sidecar.go` is 437 LOC mixing overlay-fs, exec, and sidecar lifecycle — the containerd analogue of the old `docker.go` monolith, just smaller. None of these are critical; all three are mechanical follow-ups in the 1–2 day range each.

The bones are now genuinely strong: every chaos action declares its own consumer-side narrow interface (`netemClient = Lister + Netem`), every CLI builder reveals its dependency in its constructor signature, the file-per-concern split makes 100–250 LOC the typical agent context for a focused change, and the cliflags adapter makes a `urfave/cli` v3 migration a one-file swap on the value-read side.

## Modularity Score Table (0–10 per dimension)

Same dimensions as the baseline so per-dimension delta is visible. **Δ** column shows change vs 2026-04-25.

| Component                                                        | Module size¹ | Interface narrowness² | DI / inversion³ | Abstraction leakage⁴ | Testability⁵ | Cohesion⁶ | No globals⁷ | DRY⁸ | Boundary clarity⁹ | Doc density¹⁰ | **Avg** |    **Δ** |
| ---------------------------------------------------------------- | -----------: | --------------------: | --------------: | -------------------: | -----------: | --------: | ----------: | ---: | ----------------: | ------------: | ------: | -------: |
| `pkg/container` (model + interfaces + requests)                  |            9 |                    10 |              10 |                    8 |           10 |         9 |          10 |   10 |                 9 |             9 | **9.4** | **+0.3** |
| `pkg/chaos` (Command base + scheduler + Runtime factory)         |            9 |                     9 |              10 |                    9 |            9 |         9 |          10 |    9 |                 9 |             9 | **9.2** | **+2.8** |
| `pkg/chaos/lifecycle` (was `pkg/chaos/docker`)                   |            9 |                     9 |              10 |                    8 |            9 |         9 |          10 |    8 |                10 |             8 | **9.0** | **+0.8** |
| `pkg/chaos/{netem,iptables,stress}` (action logic)               |            8 |                     9 |              10 |                    7 |            9 |         7 |          10 |    7 |                 9 |             8 | **8.4** | **+0.2** |
| `pkg/chaos/cmd` (generic `NewAction[P]` builder)                 |           10 |                     9 |              10 |                    7 |            9 |        10 |          10 |    9 |                 9 |             9 | **9.2** |  **new** |
| `pkg/chaos/cliflags` (urfave/cli v1 adapter)                     |           10 |                     9 |              10 |                    9 |           10 |        10 |          10 |   10 |                10 |             9 | **9.7** |  **new** |
| `pkg/chaos/{lifecycle,netem,iptables,stress}/cmd` (CLI builders) |            9 |                     8 |              10 |                    7 |            8 |         9 |          10 |    7 |                 9 |             7 | **8.4** | **+2.9** |
| `pkg/runtime/docker` (11 per-concern files, max 219 LOC)         |            9 |                     9 |               9 |                    7 |            9 |         9 |          10 |    8 |                 9 |             8 | **8.7** | **+2.2** |
| `pkg/runtime/containerd`                                         |            6 |                     8 |               9 |                    7 |            8 |         7 |          10 |    7 |                 8 |             7 | **7.5** | **−0.4** |
| `pkg/runtime/podman` (embed + override)                          |            8 |                     9 |               9 |                    8 |            9 |         8 |           9 |    9 |                 9 |             9 | **8.7** | **+0.3** |
| `cmd/main.go` (CLI entry, runtime select, TLS)                   |            6 |                   n/a |               9 |                    7 |            7 |         6 |           7 |    8 |                 7 |             7 | **7.1** | **+0.2** |
| **Project weighted average**                                     |              |                       |                 |                      |              |           |             |      |                   |               | **8.5** | **+1.1** |

¹ **Module size / cognitive load** — 10 = ≤ 200 LOC focused; 5 = ~500 LOC; 0 = 2k+ LOC mixed-concern.
² **Interface narrowness** — 10 = consumer-side sub-interfaces; 0 = fat `Client`.
³ **DI / inversion** — 10 = constructor injection; 0 = singletons / globals.
⁴ **Abstraction leakage** — 10 = clean domain language; 0 = passes runtime-specific concepts through every layer.
⁵ **Testability** — 10 = unit tests need only mockery + a narrow iface; 0 = needs a real Docker socket.
⁶ **Cohesion** — single reason to change per file/package. 10 = single concern; 0 = grab-bag.
⁷ **No globals** — absence of mutable package-level state read across packages. 10 = none; 0 = pervasive.
⁸ **DRY** — 10 = no `//nolint:dupl` and no copy-paste; 0 = duplication everywhere.
⁹ **Boundary clarity** — package name matches contract. 10 = self-evident; 0 = misleading.
¹⁰ **Doc density** — godoc on exported symbols + non-obvious WHY-comments.

### Per-dimension project totals

| Dimension            | 2026-04-25 | 2026-04-27 |    Δ |
| -------------------- | ---------: | ---------: | ---: |
| Module size          |        7.0 |        8.4 | +1.4 |
| Interface narrowness |        7.7 |        8.9 | +1.2 |
| DI / inversion       |        7.4 |        9.6 | +2.2 |
| Abstraction leakage  |        7.0 |        7.6 | +0.6 |
| Testability          |        7.7 |        8.7 | +1.0 |
| Cohesion             |        7.1 |        8.4 | +1.3 |
| No globals           |        7.4 |        9.6 | +2.2 |
| DRY                  |        7.6 |        8.4 | +0.8 |
| Boundary clarity     |        7.4 |        8.9 | +1.5 |
| Doc density          |        7.4 |        8.2 | +0.8 |

Largest movements: **DI/inversion (+2.2)** and **No globals (+2.2)** — directly attributable to killing `chaos.DockerClient` and routing the runtime through a constructor closure. **Boundary clarity (+1.5)** reflects the `pkg/chaos/docker` → `pkg/chaos/lifecycle` rename. **Module size (+1.4)** reflects the docker.go split. **Abstraction leakage (+0.6)** is the smallest gain — see Issues 1 and 2.

## Coupling Overview Table

| Integration                                                                              | [Strength](https://coupling.dev/posts/dimensions-of-coupling/integration-strength/) | [Distance](https://coupling.dev/posts/dimensions-of-coupling/distance/) | [Volatility](https://coupling.dev/posts/dimensions-of-coupling/volatility/) | [Balanced?](https://coupling.dev/posts/core-concepts/balance/) |
| ---------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------- | ----------------------------------------------------------------------- | --------------------------------------------------------------------------- | -------------------------------------------------------------- |
| `pkg/chaos/*/cmd` → `chaos.Runtime` factory closure (was: global `chaos.DockerClient`)   | Contract                                                                            | Same module, sync                                                       | Low                                                                         | **Yes — fixed**                                                |
| Chaos action → `container.{Netem,IPTables}` request-typed interface                      | Contract                                                                            | Same module, sync                                                       | Medium                                                                      | **Yes — fixed**                                                |
| Chaos action → `container.Stressor` (still 8-pos args + 4 return chans)                  | Model (Docker concepts: image, pull, injectCgroup)                                  | Same module, sync                                                       | Medium                                                                      | **No — same problem class as old NetemContainer (Issue 1)**    |
| `pkg/chaos/{netem,iptables}` internal `Params` ↔ `*Request` parallel shapes              | Functional (duplicate knowledge, two mental models per concept)                     | Same package                                                            | Medium                                                                      | **No — refactor artifact (Issue 2)**                           |
| `pkg/runtime/containerd/sidecar.go` (437 LOC, multi-concern)                             | Functional (overlay-fs + exec + sidecar lifecycle in one file)                      | Same file                                                               | Medium-High                                                                 | **No — Docker-monolith analogue (Issue 3)**                    |
| `pkg/runtime/docker/{netem,iptables}.go` internal helpers (still 9-pos args)             | Functional                                                                          | Same file                                                               | Low (request shape stable post-refactor)                                    | Tolerable — minor (Issue 4)                                    |
| `Lifecycle.RemoveContainer(ctx, *Container, bool, bool, bool, bool)`                     | Primitive obsession (4 naked positional bools)                                      | Same module, sync                                                       | Low                                                                         | Tolerable — minor (Issue 5)                                    |
| Public CLI ↔ `urfave/cli v1` for value reads                                             | Contract (via `cliflags.Flags`)                                                     | Library-level                                                           | Provider: high. Functional: low                                             | **Yes — adapter isolates v3 migration to one file**            |
| Public CLI ↔ `urfave/cli v1` for flag _declarations_ (`cli.StringFlag{}` etc.)           | Model                                                                               | Same package                                                            | Provider: high                                                              | Accepted trade-off — see "Notes on what's already good"        |
| `pkg/runtime/podman.podmanClient` → `pkg/runtime/docker.dockerClient` (struct embedding) | Intrusive (override-by-shadow)                                                      | Same module, sync                                                       | High (Podman drift)                                                         | Tolerable — distance is short; track                           |
| Runtime impls → Docker SDK / containerd v2 SDK                                           | Model                                                                               | Library-level, sync                                                     | Medium                                                                      | Yes (concentrated inside `pkg/runtime/*`)                      |

## Issues

### Issue 1: `Stressor` interface still leaks Docker concepts and uses 8 positional args + 4 return channels

**Integration**: `container.Stressor` → all three runtime implementations
**Severity**: **Significant**

#### Knowledge Leakage

The contract in `pkg/container/client.go:53`:

```go
type Stressor interface {
    StressContainer(ctx context.Context, c *Container, stressors []string,
        image string, pull bool, duration time.Duration,
        injectCgroup, dryrun bool) (string, <-chan string, <-chan error, error)
}
```

This is the same problem class the refactor fixed for `Netem` and `IPTables` — the cleanup just stopped before `Stressor`:

1. **`image` and `pull` are Docker concepts.** Same leak as the original `tcimg`/`pull` complaint. Containerd and Podman both have to invent semantics for "pull this image into our store" before the chaos layer's call lands.
2. **`injectCgroup` is a Docker / cgroup-v2 implementation detail.** No other runtime has cgroup-injection semantics; both containerd and podman inspect this flag and branch on it (containerd: `pkg/runtime/containerd/client.go:336`, podman: see `pkg/runtime/podman/stress.go`).
3. **8 positional args, 5 of them booleans / strings.** Same primitive-obsession risk as old `NetemContainer`. The `injectCgroup, dryrun bool` tail is the most footgun-prone — they have identical types.
4. **Quad-channel return.** `(string, <-chan string, <-chan error, error)` — the streaming output channel is interleaved with synchronous errors. AI agents writing tests against this routinely deadlock the channels.

This is [model coupling](https://coupling.dev/posts/dimensions-of-coupling/integration-strength/) of the same shape Issue 2 (in the 2026-04-25 baseline) flagged for `Netem`/`IPTables` — the abstraction's vocabulary mirrors Docker's mental model.

#### Cascading Changes

- Adding a runtime that doesn't have cgroup-injection semantics requires fudging or hard-erroring on the `injectCgroup` flag.
- A new stress-ng option means another positional argument across three runtimes plus mocks.
- Mocking the four return channels in tests is so awkward that the `pkg/chaos/stress/stress.go` tests already lean heavily on bookkeeping fixtures.

#### Recommended Improvement

Mirror the Task-5 fix for `Netem`/`IPTables`. Add `pkg/container/stress.go`:

```go
type StressRequest struct {
    Container    *Container
    Stressors    []string
    Duration     time.Duration
    Sidecar      SidecarSpec   // reuse existing type
    InjectCgroup bool          // implementation hint, runtime may ignore
    DryRun       bool
}

type StressResult struct {
    SidecarID string
    Output    <-chan string
    Errors    <-chan error
}

type Stressor interface {
    StressContainer(context.Context, *StressRequest) (*StressResult, error)
}
```

Two wins: (a) `injectCgroup` becomes an explicit "implementation hint" that runtimes are free to ignore, paralleling the existing `SidecarSpec` precedent; (b) `StressResult` collapses the 4-tuple into one struct and makes the streaming contract explicit.

Effort: ~½ day, mostly mechanical (the netem/iptables refactor is the template).

**Trade-off**: another mock regen. Worth it — the `Stressor` is the last interface in the codebase that still violates the value-object convention the refactor set.

---

### Issue 2: Parallel `Params` and `*Request` data shapes inside chaos action packages

**Integration**: `pkg/chaos/netem` and `pkg/chaos/iptables` (knowledge duplication, refactor artifact)
**Severity**: **Significant**

#### Knowledge Leakage

After Task 5, `pkg/chaos/netem/netem.go:38` declares:

```go
type Params struct {
    Iface    string
    Ips      []*net.IPNet
    Sports   []string
    Dports   []string
    Duration time.Duration
    Image    string
    Pull     bool
    Limit    int
}
```

And `pkg/container/requests.go:20` declares:

```go
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
```

Every per-action file (`delay.go`, `loss.go`, `corrupt.go`, etc.) reads `Params` and constructs a `NetemRequest` from it (verified at `pkg/chaos/netem/{corrupt,delay,duplicate,loss,loss_ge,loss_state,rate}.go`). The same shape exists for `pkg/chaos/iptables/iptables.go:47`.

These two structs encode the same domain concept twice with **different casing rules** (`Ips` vs `IPs`, `Sports`/`Dports` vs `SPorts`/`DPorts`), **different field sets** (`Params.Image, Pull` vs `NetemRequest.Sidecar.Image, Sidecar.Pull`), and a **field-by-field copy at every callsite**. AI agents reading any chaos action file must hold two parallel mental models for the same concept and translate between them on every edit. That's exactly the cognitive load the refactor was supposed to reduce.

This is **the most interesting finding from a post-refactor review** — the duplication did not exist before Task 5. The refactor added the cross-package `NetemRequest` _without_ removing the package-internal `Params`. Both are now load-bearing.

#### Cascading Changes

- A new netem option (e.g. `--queue-length`) must be added to `Params` _and_ `NetemRequest` _and_ the conversion at every callsite.
- A field rename in either struct requires audit of every per-action file.
- The `Image, Pull` ↔ `Sidecar.Image, Sidecar.Pull` split between the two structs is a silent foot-gun: an AI agent that copies a `Params` field into a `NetemRequest` literal at the wrong nesting level produces a compile error or, worse, silent zero-value at runtime.

#### Recommended Improvement

Drop `Params`. Have the cmd-builder parsers construct `*NetemRequest` and `*IPTablesRequest` directly:

```go
// pkg/chaos/netem/cmd/delay.go
func parseDelayParams(c cliflags.Flags, gp *chaos.GlobalParams) (DelayParams, error) {
    base, err := netem.ParseRequestBase(c)
    if err != nil { return DelayParams{}, err }
    return DelayParams{
        Base:    base,            // *container.NetemRequest skeleton
        Time:    c.Int("time"),
        Jitter:  c.Int("jitter"),
        ...
    }, nil
}
```

Then `runNetem` becomes the only function that holds the request struct. The internal `netemCommand` struct in `pkg/chaos/netem/netem.go:21` collapses too — most of its fields are duplicated from `Params` and `*NetemRequest`.

Effort: ~1 day across both packages plus tests. The conversion is mechanical because the two shapes already encode the same fields; only field names need normalizing.

**Trade-off**: the per-action files become slightly heavier (each constructs a fuller request) — but the package-level surface area drops by one struct, and the "two shapes per concept" rule of thumb holds again.

---

### Issue 3: `pkg/runtime/containerd/sidecar.go` is a 437-LOC multi-concern file

**Integration**: internal cohesion of `pkg/runtime/containerd`
**Severity**: **Minor-to-significant**

#### Knowledge Leakage

`pkg/runtime/containerd/sidecar.go` (437 LOC) holds:

1. Sidecar container provisioning (image pull, OCI spec build, snapshot key)
2. Overlay-fs mount preparation for joining target's network namespace
3. tc command exec inside the sidecar
4. iptables command exec inside the sidecar
5. cg-inject driver logic for stress-ng
6. Sidecar reap with `WithoutCancel` cleanup

This is the containerd-runtime analogue of the old 1,289-LOC `docker.go` monolith — same problem, ~⅓ the size. The refactor split `docker.go` along these exact responsibility lines (`netem.go`, `iptables.go`, `stress.go`, `sidecar.go`); containerd never got the same treatment.

The risk is identical to the original Issue 3: editing the cg-inject logic forces the agent to load 437 LOC of unrelated overlay-fs code.

#### Cascading Changes

- Cgroup-driver detection changes (low-frequency but high-stakes) live next to overlay-fs mount logic. They share a test file.
- The `commands.go` file (117 LOC) holds tc/iptables command-string builders that are also called from sidecar.go — the boundary between "build the command" and "exec the command in a sidecar" is muddled.

#### Recommended Improvement

Apply the same per-concern split that succeeded for `docker.go`:

```
pkg/runtime/containerd/
  client.go        — NewClient, struct, namespace context
  inspect.go       — toContainer, ListContainers
  task.go          — start/stop/kill/pause task plumbing (already exists, keep)
  commands.go      — tc/iptables command builders (already exists, keep)
  sidecar.go       — sidecar lifecycle only (provisioning + reap)
  netem.go         — NetemContainer, StopNetemContainer, runTCCommands
  iptables.go      — IPTablesContainer, StopIPTablesContainer, runIPTablesCommands
  stress.go        — StressContainer, stressDirectExec, stressSidecar (already partial)
  cgroup.go        — cgroup driver detection (currently inside sidecar.go)
```

Pure cut-and-paste; no public surface changes. Each resulting file lands at 100–250 LOC.

Effort: ~½ day. Lower priority than Issues 1 and 2 because the runtime is less frequently touched than the chaos layer, but it's the kind of cleanup that pays for itself the next time someone does need to edit containerd.

---

### Issue 4 (minor): `pkg/runtime/docker/netem.go` and `iptables.go` internal helpers still take 9 positional args

`pkg/runtime/docker/netem.go:49` etc. — the public `NetemContainer(ctx, *NetemRequest)` immediately unpacks the request and calls `startNetemContainer(ctx, c, netInterface, netemCmd, tcimg, pull, dryrun)`. The interface boundary is clean; the implementation file is not.

This is **same-file functional coupling with low volatility** (the request shape stabilized in Task 5). It's not unbalanced — the distance is zero. But it's a free win: passing `*ctr.NetemRequest` two levels deeper in the same file would erase another 60 LOC of repetition and reduce the position-mistake surface.

**Severity: minor follow-up.** Bundle with Issue 2 if/when that work happens.

---

### Issue 5 (minor): `Lifecycle.RemoveContainer(ctx, *Container, bool, bool, bool, bool)`

Four naked booleans (`force, links, volumes, dryrun`) at every callsite (`pkg/container/client.go:25`). Same primitive-obsession risk as the old `NetemContainer` 11-arg shape, scaled down. **Volatility is low** — these flags haven't grown in years — so the balance rule says tolerable. But it's a foot-gun for AI agents, who will eventually swap two booleans without the compiler noticing.

If touched, package as `RemoveOpts` struct. **Severity: minor.**

---

## Notes on what's already good (preserved + new wins)

Carryover from the 2026-04-25 baseline (still load-bearing):

- **`pkg/container/client.go` decomposes `Client` into six narrow sub-interfaces.** Every chaos action declares a consumer-side interface that composes only what it needs.
- **`pkg/runtime/podman` embeds `ctr.Client` and overrides only divergent methods.** The `apiBackend` test seam keeps unit tests off real sockets.
- **`pkg/chaos/command.go`'s `Command` interface** is one method.
- **`context.WithoutCancel` cleanup discipline** for sidecar removal — preserved and propagated to the new `runSidecar` helper.

New wins from this refactor:

- **`chaos.Runtime` factory closure (`pkg/chaos/command.go:23`).** Every CLI builder takes `runtime chaos.Runtime` as an explicit constructor parameter. The dependency is visible at every call boundary — exactly what AI agents need.
- **Generic `NewAction[P]` builder (`pkg/chaos/cmd/builder.go`).** 17 cmd files now share one shape: flag list + typed `ParamParser[P]` + `CommandFactory[P]`. Each cmd file is 50–80 LOC and reads in one screen.
- **`cliflags.Flags` adapter (`pkg/chaos/cliflags/`).** `urfave/cli` v3 migration is a one-file swap on the value-read side. The adapter is 60 LOC and fully tested.
- **`*NetemRequest` / `*IPTablesRequest` value objects (`pkg/container/requests.go`).** `SidecarSpec` makes the "implementation hint" semantic explicit.
- **`runSidecar` consolidation (`pkg/runtime/docker/sidecar.go`).** The previously-duplicated tc and iptables sidecar code is one helper. The exit-code check on `runSidecarExec` (line 142) closed a silent-failure bug found during code review.
- **File-per-concern docker runtime.** 11 production files, max 219 LOC. A focused change rarely loads more than 250 LOC of context.

Accepted trade-off, not flagged as an issue:

- **Flag _declarations_ still use `cli.StringFlag{...}` etc.** through `Spec.Flags`. The `cliflags.Flags` adapter intentionally covers only value-read; declaring flags through an extra abstraction would cost more than the value-side adapter saves. Migration to v3 will still touch every cmd file, but the change is mechanical schema renaming, not semantic rewiring.

## Recommended order of execution

If repaired in this order:

1. **Issue 1** (`StressRequest` value object). ~½ day. Pattern is already established by Task 5 — pure mechanical replication for the last interface that still violates the convention.
2. **Issue 2** (collapse `Params` into `*NetemRequest`/`*IPTablesRequest`). ~1 day. The single most AI-agent-impactful follow-up — eliminates the "two shapes per concept" cognitive tax across both action packages.
3. **Issue 3** (split `containerd/sidecar.go`). ~½ day. Same pattern as the docker.go split that worked. Lower urgency, do when containerd code needs editing anyway.
4. **Issues 4 & 5**: bundle with the next adjacent edit, don't schedule separately.

Estimated total: **~2 days** to push to **~9.0 / 10**. The big jumps (DI/inversion, no globals, boundary clarity, module size) are already banked.

---

_This analysis was performed using the [Balanced Coupling](https://coupling.dev) model by [Vlad Khononov](https://vladikk.com)._

_Compares against `docs/modularity-review/2026-04-25/modularity-review.md`. Refactor that closed the 2026-04-25 findings: `docs/plans/completed/20260427-modularity-refactor.md`._
