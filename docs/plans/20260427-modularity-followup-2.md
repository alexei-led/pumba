# Modularity Follow-up 2 — Coupling Reduction & Abstraction Layers

## Overview

Closes the six remaining findings from `docs/modularity-review/2026-04-27/modularity-review.md` (post-refactor baseline 9.0/10 → target ~9.5/10). Goal: reduce code coupling and concentrate abstraction layers so both humans and LLM agents can change Pumba with smaller context windows, fewer files per task, and predictable blast radius.

The six items, in priority order:

1. Extract the duplicated `Run()` body across 15 chaos action files into `chaos.RunOnContainers`.
2. Move the `reInterface` regex to `pkg/util` (eliminates duplicate `regexp.MustCompile`).
3. Document the Podman → Docker embedding invariant.
4. Add `pkg/runtime/podman/doc.go` naming Docker-compat as the intentional working vocabulary.
5. Extend `pkg/chaos/cliflags` to cover global flag reads in `cmd/main.go`.
6. Split `cmd/main.go` (455 LOC) into focused per-concern files.

Items 1 and 6 are the structural changes; 2 is mechanical; 3, 4, 5 are documentation / interface extension. Total ~2 days.

## Context (from discovery)

- **Files involved**:
  - 15 chaos action files: `pkg/chaos/netem/{corrupt,delay,duplicate,loss,loss_ge,loss_state,rate}.go`, `pkg/chaos/iptables/loss.go`, `pkg/chaos/lifecycle/{exec,kill,pause,remove,restart,stop}.go`, `pkg/chaos/stress/stress.go`
  - `pkg/chaos/{netem,iptables}/parse.go` (duplicate `reInterface` regex)
  - `pkg/runtime/podman/client.go` (embedding invariant)
  - `pkg/chaos/cliflags/{flags,v1}.go` (extend for global flags)
  - `cmd/main.go` (455 LOC, multi-concern)

- **Existing patterns** (from CLAUDE.md and prior refactor):
  - Constructor injection over globals: `chaos.Runtime func() container.Client` factory closure
  - Consumer-side narrow interface aliases: `netemClient = container.Lister + container.Netem`, `killClient`, `stressClient`, etc.
  - Request value objects: `NetemRequest`, `IPTablesRequest`, `StressRequest`, `RemoveOpts`, `SidecarSpec`
  - Generic `NewAction[P]` builder with `cliflags.Flags` adapter for value reads
  - `context.WithoutCancel` cleanup discipline for sidecar reap
  - Mocks: `container.NewMockClient(t)` (mockery v2.53.5), narrow interfaces have package-local mock files
  - Run unit tests in sandbox: `CGO_ENABLED=0 go test ./...`
  - Error wrapping: `fmt.Errorf("...: %w", err)` (migrating away from `pkg/errors`)
  - File-per-concern split: docker (max 219 LOC), containerd (max 222 LOC)

- **Dependencies identified**:
  - `golang.org/x/sync/errgroup` already used by `pkg/chaos/stress/stress.go` — Item 1 standardises every parallel chaos action on errgroup
  - `urfave/cli` v1 used everywhere; v3 migration on roadmap (Item 5 prepares for it)
  - testify (assert/require/mock) is the test toolkit; no new test deps

## Development Approach

- **Testing approach**: Regular (code + tests per task). Existing tests already cover NoContainers/DryRun/WithRandom for every chaos action and assert behavior — they catch regressions immediately.
- complete each task fully before moving to the next
- make small, focused changes
- **CRITICAL: every task MUST include new/updated tests** for code changes in that task
- **CRITICAL: all tests must pass before starting next task** — `CGO_ENABLED=0 go test ./...`
- **CRITICAL: run `make lint` after each task** to catch golangci-lint regressions early
- update this plan file when scope changes during implementation
- maintain backward compatibility: all existing CLI flags, public package surfaces, and integration test behavior unchanged

## Testing Strategy

- **unit tests**: required for every task, run with `CGO_ENABLED=0 go test ./...`
- **mock regen**: `make mocks` after any interface change (Item 5 only — no other interface changes in this plan)
- **integration tests** (bats): not modified; the public CLI surface and runtime behavior are preserved. Optional smoke check before the final task: `colima ssh -- sudo bats tests/*.bats tests/containerd_*.bats`
- **lint**: `make lint` must pass after every task — `gocyclo` 15, `funlen` 105 limits, `mnd`, `dupl`

## Progress Tracking

- mark completed items with `[x]` immediately when done
- add newly discovered tasks with ➕ prefix
- document issues/blockers with ⚠️ prefix
- update plan if implementation deviates from original scope

## Solution Overview

**Item 1** introduces a single helper `chaos.RunOnContainers(ctx, lister, gp, limit, random, parallel, fn)` that absorbs the list-then-random-then-fanout-then-collect-errors algorithm. Every per-action `Run()` collapses from ~30–50 lines to ~10 lines. errgroup becomes the universal parallel fanout (replaces hand-rolled `sync.WaitGroup` + `errs []error` slices in netem/iptables). The helper takes `container.Lister` (not the action's narrow client interface) — keeping the helper domain-agnostic and avoiding cross-package generic-interface gymnastics.

**Item 2** moves `var reInterface = regexp.MustCompile(...)` from both parse.go files into `pkg/util.ValidateInterfaceName(name string) error`. Both parsers call it; the regex compiles once.

**Item 3** adds an invariant godoc block to `pkg/runtime/podman/client.go` listing the override set and the rule "when adding a method to `ctr.Client`, audit Podman behavior or override defensively."

**Item 4** adds `pkg/runtime/podman/doc.go` (package-level godoc) explaining that Docker SDK types are Podman's intentional working vocabulary because Podman exposes a Docker-compat socket. No code aliases — the `minimalistic` principle wins; aliases would add 50+ LOC of indirection for negligible decoupling.

**Item 5** adds an embedded `Global` interface to `cliflags.Flags` exposing the global flag values `cmd/main.go` reads (`runtime`, `log-level`, `json`, `slackhook`, `slackchannel`, `host`, `tlsverify`, `tlscacert`, `tlscert`, `tlskey`). `cliflags.NewV1` is extended to satisfy it. `cmd/main.go::before` and `createRuntimeClient` consume the adapter instead of `*cli.Context`. The v3 migration becomes a one-file swap (add `cliflags.NewV3`).

**Item 6** splits `cmd/main.go` (455 LOC) along its three natural concerns:

- `cmd/main.go` — `main()`, `init()`, signal context, app construction (~120 LOC)
- `cmd/runtime.go` — `createRuntimeClient`, `tlsConfig`, factory vars (~140 LOC)
- `cmd/logging.go` — log-level switch, slackrus hook setup (~50 LOC)
- `cmd/flags.go` — `globalFlags(rootCertPath)`, `initializeCLICommands` (~150 LOC)

Each file lands at ≤ 200 LOC. No public surface changes; same `package main`, same call graph.

## Technical Details

### chaos.RunOnContainers — exact signature

```go
// pkg/chaos/runner.go
package chaos

import (
    "context"
    "fmt"

    "github.com/alexei-led/pumba/pkg/container"
    log "github.com/sirupsen/logrus"
    "golang.org/x/sync/errgroup"
)

// ContainerAction applies a chaos action to a single target container.
// Returning an error from any closure aborts a parallel run via errgroup
// and aborts a serial run on first error.
type ContainerAction func(ctx context.Context, c *container.Container) error

// RunOnContainers lists containers matching gp.{Names,Pattern,Labels} (capped
// by limit), optionally narrows to a single random pick when random is true,
// then invokes fn for each container. parallel selects between errgroup
// fanout (true) and a sequential for-loop (false). Returns nil when no
// containers match — same warning the per-action loops used to log.
func RunOnContainers(
    ctx context.Context,
    lister container.Lister,
    gp *GlobalParams,
    limit int,
    random, parallel bool,
    fn ContainerAction,
) error {
    containers, err := container.ListNContainers(ctx, lister, gp.Names, gp.Pattern, gp.Labels, limit)
    if err != nil {
        return fmt.Errorf("listing containers: %w", err)
    }
    if len(containers) == 0 {
        log.Warning("no containers found")
        return nil
    }
    if random {
        if c := container.RandomContainer(containers); c != nil {
            containers = []*container.Container{c}
        }
    }
    if !parallel {
        for _, c := range containers {
            if err := fn(ctx, c); err != nil {
                return err
            }
        }
        return nil
    }
    var eg errgroup.Group
    for _, c := range containers {
        eg.Go(func() error { return fn(ctx, c) })
    }
    return eg.Wait()
}
```

Key design points:

- `container.Lister` (not the per-action `netemClient`/`killClient` interface) — the helper is intentionally domain-agnostic. Every consumer interface already embeds `container.Lister`, so the existing call sites pass `n.client` etc. without change.
- Go 1.22+ loop-variable scoping makes the `eg.Go(func() error { ... })` capture safe without `c := c` shadowing — confirmed Go 1.26 in `go.mod`.
- No timeout / cancellation enrichment — current per-action code uses `context.WithCancel(ctx)` or `context.WithTimeout(ctx, n.req.Duration)` inside the closure, where it belongs (the timeout is action-specific, not container-iteration-specific).

### Call-site shape (netem delay example)

Before (`pkg/chaos/netem/delay.go:72-143`, ~70 LOC):

```go
func (n *delayCommand) Run(ctx context.Context, random bool) error {
    log.Debug(...)
    log.WithFields(...).Debug("listing matching containers")
    containers, err := container.ListNContainers(...)
    if err != nil { ... }
    if len(containers) == 0 { ... }
    if random { ... }
    netemCmd := []string{...}
    var wg sync.WaitGroup
    errs := make([]error, len(containers))
    for i, c := range containers {
        wg.Add(1)
        go func(i int, c *container.Container) { ... }(i, c)
    }
    wg.Wait()
    for _, err := range errs { ... }
    return nil
}
```

After (~20 LOC):

```go
func (n *delayCommand) Run(ctx context.Context, random bool) error {
    netemCmd := n.buildNetemCmd()
    return chaos.RunOnContainers(ctx, n.client, n.gp, n.limit, random, true,
        func(ctx context.Context, c *container.Container) error {
            netemCtx, cancel := context.WithCancel(ctx)
            defer cancel()
            req := *n.req
            req.Container = c
            req.Command = netemCmd
            return runNetem(netemCtx, n.client, &req)
        })
}

func (n *delayCommand) buildNetemCmd() []string {
    cmd := []string{"delay", strconv.Itoa(n.time) + "ms"}
    // ... existing builder logic
    return cmd
}
```

### cliflags global-flag extension

```go
// pkg/chaos/cliflags/flags.go
type Flags interface {
    // existing methods (Bool, String, StringSlice, Int, Float64, Duration, Args, Parent, Global)
    Global() Flags
}

// New: Global is the same Flags interface — already returned by the existing
// Global() method. cmd/main.go callers receive a *cliflags.V1 wrapping the
// app context and read globals directly (c.Global().String("runtime")).
```

The existing `Global()` method already returns a `Flags` (via `c.App.Run(...)` global context). Item 5 doesn't change the `Flags` interface itself — it introduces a `cliflags.V1FromApp(*cli.Context)` constructor that wraps `cmd/main.go`'s app-level cli.Context, and converts `before(c *cli.Context)` and `createRuntimeClient(c *cli.Context)` to take `cliflags.Flags`.

## What Goes Where

- **Implementation Steps** (`[ ]`): all six refactor tasks plus tests are inside this codebase
- **Post-Completion** (no checkboxes): integration test smoke check via colima/bats; no external systems

## Implementation Steps

### Task 1: Extract `chaos.RunOnContainers` helper

**Files:**

- Create: `pkg/chaos/runner.go`
- Create: `pkg/chaos/runner_test.go`

- [x] create `pkg/chaos/runner.go` with `ContainerAction` type and `RunOnContainers` function (exact shape per Technical Details)
- [x] write table-driven tests in `pkg/chaos/runner_test.go` covering: empty list / list error / single container serial / multiple containers serial / multiple containers parallel / random pick path / parallel error short-circuits via errgroup / serial error stops on first
- [x] use `container.NewMockClient(t)` for the `container.Lister` argument; assert with `testify/require` for setup, `assert` for outcomes
- [x] add a godoc example showing a typical call site so future actions copy from one canonical place
- [x] run `CGO_ENABLED=0 go test ./pkg/chaos/...` and `make lint` — must pass before Task 2

### Task 2: Migrate netem actions to `chaos.RunOnContainers`

**Files:**

- Modify: `pkg/chaos/netem/{corrupt,delay,duplicate,loss,loss_ge,loss_state,rate}.go`
- Modify: `pkg/chaos/netem/{corrupt,delay,duplicate,loss,loss_ge,loss_state,rate}_test.go` (only if existing assertions reference removed `sync.WaitGroup`/`errs[]` internals — likely none do)

- [x] extract the `netemCmd := []string{...}` builder logic from each action's Run() into a private method (e.g. `(n *delayCommand) buildNetemCmd()`); the closure remains data-only
- [x] replace each action's Run() body with a `chaos.RunOnContainers(...)` call wrapping the existing per-container closure (timeout/withTimeout stays inside the closure where the existing code put it)
- [x] drop the `sync` and `sync.WaitGroup` imports per file once the manual fanout is gone
- [x] verify tests still pass for NoContainers / DryRun / WithRandom paths in each action's existing `*_test.go`
- [x] update any test that asserted `WaitGroup` invariants (none expected; verify with grep)
- [x] run `CGO_ENABLED=0 go test ./pkg/chaos/netem/...` and `make lint` — must pass before Task 3

### Task 3: Migrate iptables, lifecycle, stress actions to `chaos.RunOnContainers`

**Files:**

- Modify: `pkg/chaos/iptables/loss.go`
- Modify: `pkg/chaos/lifecycle/{exec,kill,pause,remove,restart,stop}.go`
- Modify: `pkg/chaos/stress/stress.go`

- [ ] iptables/loss.go: extract `cmdPrefix`/`cmdSuffix` builder into a private method; replace Run body with `RunOnContainers(... parallel=true, fn)` matching netem migration shape
- [ ] lifecycle actions: each Run body becomes `RunOnContainers(... parallel=false, ...)` (lifecycle ops are sequential today — preserve)
- [ ] stress: replace the existing `errgroup` block with `RunOnContainers(... parallel=true, ...)` — keeps the same fanout semantics with one fewer pattern
- [ ] verify NoContainers / DryRun / WithRandom tests pass in each action's existing `*_test.go`
- [ ] confirm grep finds no remaining `container.ListNContainers(ctx, ` callers in `pkg/chaos/{netem,iptables,lifecycle,stress}` — all should route through the helper
- [ ] run `CGO_ENABLED=0 go test ./pkg/chaos/...` and `make lint` — must pass before Task 4

### Task 4: Move `reInterface` regex to `pkg/util`

**Files:**

- Modify: `pkg/util/util.go`
- Modify: `pkg/util/util_test.go`
- Modify: `pkg/chaos/netem/parse.go`
- Modify: `pkg/chaos/iptables/parse.go`

- [ ] add `var reInterface = regexp.MustCompile(...)` and `func ValidateInterfaceName(name string) error` to `pkg/util/util.go`
- [ ] replace `iface != reInterface.FindString(iface)` checks in both parse.go files with `if err := util.ValidateInterfaceName(iface); err != nil { ... }`
- [ ] remove the `var reInterface = ...` constant and the unused `regexp` import from both parse files
- [ ] add table-driven tests in `pkg/util/util_test.go` covering: valid names (eth0, en0, lo, wlan-1, vlan.10), invalid names (1eth, eth\*, eth$, empty, "rm -rf /")
- [ ] run `CGO_ENABLED=0 go test ./pkg/...` and `make lint` — must pass before Task 5

### Task 5: Document Podman embedding invariant

**Files:**

- Modify: `pkg/runtime/podman/client.go`

- [ ] add a godoc block above `type podmanClient struct` listing the override set (`NetemContainer`, `StopNetemContainer`, `IPTablesContainer`, `StopIPTablesContainer`, `Close`, `StressContainer`) and explaining that every other `ctr.Client` method is intentionally inherited from the docker delegate
- [ ] state the invariant: "When adding a method to `ctr.Client`, audit Podman behavior — either confirm Docker's implementation works on the Docker-compat socket, or override defensively in this package."
- [ ] no code change; doc-only — no test changes needed
- [ ] run `make lint` — must pass before Task 6

### Task 6: Add `pkg/runtime/podman/doc.go`

**Files:**

- Create: `pkg/runtime/podman/doc.go`

- [ ] create `doc.go` with a package-level godoc comment explaining: (a) Podman exposes a Docker-compatible API socket and pumba programs against that socket via the Docker SDK, (b) the package depends on `github.com/docker/docker/api/types/{container,image,network,system}` by design, (c) only the three divergent paths (rootless guards, cgroup leaf naming, sidecar config) carry Podman-specific code
- [ ] cross-link to `pkg/runtime/podman/client.go`'s embedding-invariant doc block from Task 5
- [ ] no code change; doc-only — no test changes needed
- [ ] run `make lint` — must pass before Task 7

### Task 7: Extend `cliflags.Flags` for global-flag reads

**Files:**

- Modify: `pkg/chaos/cliflags/flags.go`
- Modify: `pkg/chaos/cliflags/v1.go`
- Create: `pkg/chaos/cliflags/v1_test.go` (or extend existing if present)

- [ ] add `cliflags.NewV1FromApp(c *cli.Context) Flags` that wraps the app-level (root) cli.Context — semantically equivalent to `NewV1(c).Global()` but exposed as a standalone constructor for callers that don't have a subcommand context (i.e. `cmd/main.go::before`)
- [ ] verify the existing `Flags` interface's read methods (`Bool`, `String`, `StringSlice`, `Int`, `Duration`) cover every global-flag read in `cmd/main.go::before` and `createRuntimeClient` (they do — the v1 adapter's reads delegate to the underlying `*cli.Context`'s `String`/`Bool` which work identically on the app-level context)
- [ ] add tests in `pkg/chaos/cliflags/v1_test.go` for `NewV1FromApp`: read each global flag type via the Flags interface and assert the value
- [ ] run `CGO_ENABLED=0 go test ./pkg/chaos/cliflags/...` and `make lint` — must pass before Task 8

### Task 8: Convert `cmd/main.go::before` and `createRuntimeClient` to consume `cliflags.Flags`

**Files:**

- Modify: `cmd/main.go` (still monolithic at this point; Task 9 splits it)

- [ ] change `before(c *cli.Context) error` body to construct `f := cliflags.NewV1FromApp(c)` once at the top, then read all global values via `f.String("log-level")`, `f.Bool("json")`, etc. (the function signature must keep `*cli.Context` because urfave/cli v1 dictates it — the conversion is internal-only)
- [ ] change `createRuntimeClient(c *cli.Context)` similarly: read `runtime`, `host`, `tlsverify`, `tlscacert`, `tlscert`, `tlskey` via the adapter
- [ ] keep `tlsConfig(c *cli.Context)` reading directly from `*cli.Context` for now — it uses `os.Getenv` paths and TLS-specific helpers; converting it adds noise without payoff. Document why in a one-line comment.
- [ ] run all main_test.go tests (existing tests for `createRuntimeClient` already swap factories via package vars — keep those test seams unchanged)
- [ ] run `CGO_ENABLED=0 go test ./cmd/...` and `make lint` — must pass before Task 9

### Task 9: Split `cmd/main.go` into focused per-concern files

**Files:**

- Modify: `cmd/main.go` (shrinks to ~120 LOC: main + init + signal context + app construction)
- Create: `cmd/runtime.go` (~140 LOC: `createRuntimeClient`, `tlsConfig`, runtime factory vars `newDockerClient`/`newContainerdClient`/`newPodmanClient`)
- Create: `cmd/logging.go` (~50 LOC: `before`'s log-level switch, slackrus hook wiring)
- Create: `cmd/flags.go` (~150 LOC: `globalFlags(rootCertPath)`, `initializeCLICommands`)
- Modify: `cmd/main_test.go` if package-internal references break

- [ ] move `createRuntimeClient`, `tlsConfig`, `newDockerClient`/`newContainerdClient`/`newPodmanClient` factory vars to `cmd/runtime.go`
- [ ] extract the log-level switch and slackrus hook setup from `before` into `setupLogging(f cliflags.Flags)` in `cmd/logging.go`; `before` calls it
- [ ] move `globalFlags` and `initializeCLICommands` into `cmd/flags.go`
- [ ] keep `cmd/main.go` minimal: `main()`, `init()`, `handleSignals`, `topContext`, `runtimeClient` package var, `app := cli.NewApp(); ...; app.Run(os.Args)`
- [ ] verify every file is under 200 LOC (`wc -l cmd/*.go`)
- [ ] verify the build: `go build ./cmd/...`
- [ ] run any existing main_test.go tests — none expected to need changes since `package main` stays unified
- [ ] run `CGO_ENABLED=0 go test ./...` and `make lint` — must pass before Task 10

### Task 10: Verify acceptance criteria

- [ ] verify all six items from Overview are implemented; tick each in this section
- [ ] verify file size budgets:
  - `chaos.RunOnContainers` helper ≤ 80 LOC
  - every chaos action `Run()` ≤ 30 LOC (down from 70–120)
  - every `cmd/*.go` file ≤ 200 LOC
- [ ] verify duplicate-pattern audit:
  - `grep -rn "sync.WaitGroup" pkg/chaos/` → 0 results
  - `grep -rn "regexp.MustCompile" pkg/chaos/` → 0 results (regex moved to util)
  - `grep -rn "ListNContainers(ctx, " pkg/chaos/{netem,iptables,lifecycle,stress}` → 0 results (all routed through helper)
- [ ] run full test suite: `CGO_ENABLED=0 go test ./...`
- [ ] run race detector if on linux/amd64: `make test-race`
- [ ] run `make lint` and `make build`
- [ ] integration smoke (manual): `colima ssh -- sudo bats tests/netem.bats tests/iptables.bats tests/kill.bats tests/stress.bats` — verifies Run-helper behavior end-to-end on at least one action per package
- [ ] verify test coverage hasn't regressed: `make test-coverage` and compare against pre-refactor baseline

### Task 11: [Final] Update documentation

- [ ] update `CLAUDE.md` Architecture section: add `chaos.RunOnContainers` as the canonical fanout helper; add `util.ValidateInterfaceName` as the canonical interface-name validator
- [ ] update `CLAUDE.md` Code Conventions: new chaos actions MUST use `chaos.RunOnContainers` (no hand-rolled list-then-fanout); document the convention next to the existing "Request value objects" rule
- [ ] update `CLAUDE.md` Project Structure: list the new `cmd/{runtime,logging,flags}.go` files
- [ ] move this plan to `docs/plans/completed/`

## Post-Completion

_Items requiring manual intervention — no checkboxes, informational only_

**Manual verification**:

- bats integration smoke (one action per package) listed in Task 10 — requires a Colima VM
- visually scan `cmd/*.go` files in an editor to confirm the split reads naturally for a new contributor

**External system updates** (none expected):

- no public CLI surface change → no docs site update needed
- no exported package surface change in `pkg/chaos` (helper is additive) → no semver bump implications for any external consumer

**Future-deferred work** (out of scope for this plan):

- urfave/cli v3 migration: when it lands, add `cliflags.NewV3` and swap `cmd/main.go::main`'s app construction. The `cliflags.Flags` interface itself is already v3-ready.
- Podman libpod-native SDK migration (if Podman ever drops Docker-compat): would require an anti-corruption layer in `pkg/runtime/podman`. Not actionable today; `pkg/runtime/podman/doc.go` from Task 6 documents the trigger condition.
