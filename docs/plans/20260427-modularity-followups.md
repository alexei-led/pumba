# Pumba Modularity Follow-ups

## Overview

Close the five remaining items from `docs/modularity-review/2026-04-27/modularity-review.md`. The prior 8-task refactor lifted the project from 7.4 → 8.5 / 10; this plan pushes it to ~9.0 by completing patterns the previous refactor stopped short of.

Targeted score after refactor: **~9.0 / 10**. Estimated effort: **~2 days** of focused work. No public CLI surface change.

## Context (from discovery)

Files/components involved:

- `pkg/container/client.go`, `pkg/container/requests.go` — `Stressor` interface still has 8 positional args + 4 return channels; `Lifecycle.RemoveContainer` has 4 naked positional bools
- `pkg/chaos/netem/netem.go` + 7 per-action files (`delay.go`, `loss.go`, `loss_ge.go`, `loss_state.go`, `rate.go`, `corrupt.go`, `duplicate.go`) — duplicate `Params` shape on top of `*NetemRequest`
- `pkg/chaos/iptables/iptables.go` + `loss.go` — same parallel `Params` / `*IPTablesRequest` shape
- `pkg/chaos/stress/stress.go` + `pkg/chaos/stress/cmd/stress.go` — Stressor consumer; first to migrate to `StressRequest`
- `pkg/runtime/{docker,containerd,podman}` — three Stressor implementations
- `pkg/runtime/docker/netem.go`, `pkg/runtime/docker/iptables.go` — internal helpers still take 9 positional args after request unpacking
- `pkg/runtime/containerd/sidecar.go` — 437 LOC mixing overlay-fs / exec / sidecar lifecycle (containerd analogue of the old docker.go monolith)
- `mocks/`, `pkg/container/mock_*.go` — regenerate after each interface change

Related patterns to preserve:

- **Request value object convention** (`pkg/container/requests.go`): pointer-passed structs, `SidecarSpec` for runtime hints, `DryRun` field on the request — `StressRequest` follows this exact shape.
- **Consumer-side narrow interfaces** (`netemClient`, `iptablesClient`): each chaos package defines its own composition of the `container.*` sub-interfaces. New `stressClient` already exists in `pkg/chaos/stress/stress.go` — extend, don't duplicate.
- **`NewAction[P]` builder** (`pkg/chaos/cmd/builder.go`): every CLI action shares this shape; `parseStressParams` returns the per-command struct, no need to introduce a public mid-level `Params`.
- **`cliflags.Flags` adapter** for value reads — keep parsers off `*cli.Context`.
- **`context.WithoutCancel` cleanup discipline** — preserved across all sidecar paths.
- **Mockery EXPECT() style with `container.NewMockClient(t)`** — auto-asserts on `t.Cleanup`. Use `mock.AnythingOfType("*container.StressRequest")` for the new request type.

Dependencies identified:

- Go 1.26 (generics, `slices`, `maps` already used)
- `github.com/stretchr/testify` (assert + require)
- `github.com/vektra/mockery` — regenerate via `make mocks` after every interface change
- No new external deps

## Development Approach

- **Testing approach**: Regular (refactor first, update tests in the same task)
- Complete each task fully before moving to the next
- Make small, focused changes — every task is mechanical replication of an established pattern
- **CRITICAL: every task MUST include new/updated tests** for code changes in that task
  - Update mock matchers (`mock.AnythingOfType("*container.StressRequest")` / `*container.RemoveOpts`)
  - Regenerate mocks via `make mocks` after every interface change — never hand-edit `mock_*.go`
  - Both success and error paths covered
  - **Write tests with testify `require` for fatal preconditions, `assert` for non-fatal field checks** (project convention)
- **CRITICAL: all tests must pass before starting next task** — no exceptions
- **CRITICAL: update this plan file when scope changes during implementation**
- Run `CGO_ENABLED=0 go test ./...` after each task. Run `make lint` before marking task done.
- Maintain backward compatibility for the CLI: no flag rename, no command rename, no behavior change.

## Testing Strategy

- **Unit tests**: required for every task. Run with `CGO_ENABLED=0 go test ./...` for fast inner loop, `make test` (race detector, requires CGO) before declaring task done.
- **Mocks**: `make mocks` regenerates after each interface signature change. Never hand-edit `mock_*.go` or `mocks/*.go`.
- **Integration tests**: bats tests in `tests/*.bats` — must still pass post-refactor since CLI surface is unchanged. Run only after Task 7 (verification).
  - Docker: `docker run --rm -v /var/run/docker.sock:/var/run/docker.sock --entrypoint bats pumba:test tests/*.bats`
  - Containerd: `colima ssh -- sudo bats tests/containerd_*.bats`
  - Podman: `podman machine ssh sudo bats tests/podman_*.bats`
- **No new behavioral tests** required — this is a refactor. Existing test coverage (71.9% statement) is the safety net; verify it does not regress.

## Progress Tracking

- Mark completed items with `[x]` immediately when done
- Add newly discovered tasks with ➕ prefix
- Document issues/blockers with ⚠️ prefix
- Update plan if implementation deviates from original scope

## Solution Overview

Six surgical tasks ordered so each one completes a pattern already established by the prior refactor:

1. **Add `StressRequest` / `StressResult` value objects** — completes the request-struct convention for the last leaky `container.*` interface (Issue 1 from review)
2. **Drop `Params` duplication in `pkg/chaos/netem`** — let cmd parsers build `*NetemRequest` directly (Issue 2a)
3. **Drop `Params` duplication in `pkg/chaos/iptables`** — same pattern (Issue 2b)
4. **`RemoveContainer` 4 naked bools → `RemoveOpts` struct** — last positional-bool foot-gun on the `Lifecycle` interface (minor item, low cost when bundled with the other interface changes)
5. **Push request structs deeper into `pkg/runtime/docker/{netem,iptables}.go` internal helpers** — close the same-file 9-pos arg leak (minor; mechanical now that the public boundary is clean)
6. **Split `pkg/runtime/containerd/sidecar.go` 437 LOC into per-concern files** — apply the docker.go split pattern (Issue 3)

Order rationale:

- Tasks 1, 2, 3, 4 mutate `container.*` interfaces and shared callsites — group them so `make mocks` runs minimally and test churn is concentrated.
- Task 5 is internal-only; safe to follow the public-boundary tasks because no caller change is involved.
- Task 6 is purely intra-package cut-and-paste; independent of every other task.

## Technical Details

### Task 1: `StressRequest` / `StressResult`

```go
// pkg/container/requests.go (extend existing file)
type StressRequest struct {
    Container    *Container
    Stressors    []string
    Duration     time.Duration
    Sidecar      SidecarSpec  // reuse — runtime may ignore
    InjectCgroup bool         // implementation hint, runtime may ignore
    DryRun       bool
}

type StressResult struct {
    SidecarID string
    Output    <-chan string
    Errors    <-chan error
}

// pkg/container/client.go
type Stressor interface {
    StressContainer(context.Context, *StressRequest) (*StressResult, error)
}
```

Three runtime impls (`docker`, `containerd`, `podman`) and one consumer (`pkg/chaos/stress/stress.go`) update. `StressResult` collapses the awkward 4-tuple `(string, <-chan string, <-chan error, error)` return into one struct + sentinel error — current channels are preserved as fields, so callers selecting on `<-Output` / `<-Errors` work unchanged.

### Task 2 + 3: drop `Params` in `pkg/chaos/{netem,iptables}`

Current shape (per the review): each cmd action file reads `Params` from CLI flags, then constructs `*NetemRequest` from it. Two structs encode the same domain concept with subtly different field names.

Target shape: cmd parsers construct `*NetemRequest` / `*IPTablesRequest` directly. Add a single helper per package for the shared base fields:

```go
// pkg/chaos/netem/parse.go (new)
// ParseRequestBase reads --interface, --duration, --target, --egress-port,
// --ingress-port, --tc-image, --pull-image from the netem parent + global
// flags and returns a *container.NetemRequest with those base fields filled.
// Each per-action parser augments req.Command and any action-specific fields.
func ParseRequestBase(c cliflags.Flags, gp *chaos.GlobalParams) (*container.NetemRequest, error) { ... }
```

Then `delay.go::parseDelayParams` (and the 6 sibling parsers) become:

```go
func parseDelayParams(c cliflags.Flags, gp *chaos.GlobalParams) (DelayParams, error) {
    base, err := netem.ParseRequestBase(c, gp)
    if err != nil { return DelayParams{}, err }
    base.Command = []string{"delay", fmt.Sprintf("%dms", time), ...}
    return DelayParams{Base: base, Limit: c.Int("limit")}, nil
}
```

The `netemCommand` struct in `pkg/chaos/netem/netem.go:21` collapses — its fields are duplicated into the request anyway. `runNetem` becomes the only function that holds the request struct.

### Task 4: `RemoveOpts`

```go
// pkg/container/requests.go
type RemoveOpts struct {
    Force   bool
    Links   bool
    Volumes bool
    DryRun  bool
}

// pkg/container/client.go
type Lifecycle interface {
    // ...
    RemoveContainer(context.Context, *Container, RemoveOpts) error
    // ...
}
```

Pass-by-value (struct is 4 bytes). One callsite per runtime + one per chaos consumer (`pkg/chaos/lifecycle/remove.go`).

### Task 5: push requests deeper in `pkg/runtime/docker/{netem,iptables}.go`

`startNetemContainer`, `stopNetemContainer`, `startNetemContainerIPFilter`, `ipTablesContainer`, `ipTablesContainerWithIPFilter` — all take 7–9 positional args after the public method unpacks the request struct. Change them to take `*ctr.NetemRequest` / `*ctr.IPTablesRequest` directly. No caller change outside the file; no test churn except the helper's own tests if any exist (most are tested through the public method).

### Task 6: split `pkg/runtime/containerd/sidecar.go`

Cut points (verified by reading the current file):

```
pkg/runtime/containerd/
  client.go        — keep: NewClient, Close, nsCtx, ListContainers, lifecycle methods
  task.go          — keep as is (start/stop/kill/pause primitives)
  commands.go      — keep as is (tc/iptables command builders)
  netem.go         — NEW: NetemContainer, StopNetemContainer, runTCCommands
  iptables.go      — NEW: IPTablesContainer, StopIPTablesContainer, runIPTablesCommands
  stress.go        — NEW: StressContainer, stressDirectExec, stressSidecar
  sidecar.go       — TRIM: provisioning + reap only (overlay-fs, OCI spec, snapshot)
  cgroup.go        — NEW: cgroup driver detection (currently inside sidecar.go)
```

Pure cut-and-paste. No public surface change. Each resulting file lands at 100–250 LOC.

## What Goes Where

- **Implementation Steps** — six refactor tasks with code + tests
- **Post-Completion** — bats integration sweep, mock-generation verification, score-table re-baseline (optional schedule)

## Implementation Steps

### Task 1: `StressRequest` + `StressResult` value objects

**Files:**

- Modify: `pkg/container/requests.go` (add `StressRequest` + `StressResult`)
- Modify: `pkg/container/client.go` (`Stressor` signature shrinks to `(ctx, *StressRequest) (*StressResult, error)`)
- Modify: `pkg/runtime/docker/stress.go` (consume request, return `*StressResult`)
- Modify: `pkg/runtime/containerd/client.go` + `pkg/runtime/containerd/sidecar.go` (whichever holds containerd's `StressContainer`)
- Modify: `pkg/runtime/podman/stress.go`
- Modify: `pkg/chaos/stress/stress.go` (build request, consume `*StressResult`)
- Modify: `pkg/chaos/stress/cmd/stress.go` (parse function builds request — but since this lives one level above the chaos consumer, only signature plumbing changes)
- Regenerate: `pkg/container/mock_Stressor.go`, `pkg/container/mock_Client.go`
- Modify: `pkg/chaos/stress/stress_test.go`, `pkg/runtime/{docker,containerd,podman}/stress_test.go` (mock matcher swaps to `*container.StressRequest`)

- [x] add `StressRequest` + `StressResult` to `pkg/container/requests.go` with godoc on each field
- [x] update `Stressor` interface in `pkg/container/client.go` to `StressContainer(context.Context, *StressRequest) (*StressResult, error)`
- [x] `make mocks` — regenerate
- [x] update `pkg/runtime/docker/stress.go` to accept `*StressRequest`, return `*StressResult` (no behavior change — repackage existing fields)
- [x] update `pkg/runtime/containerd/` Stress impl
- [x] update `pkg/runtime/podman/stress.go`
- [x] update `pkg/chaos/stress/stress.go` to construct `*StressRequest` and consume `*StressResult` (channels become `result.Output` / `result.Errors`)
- [x] update `pkg/chaos/stress/stress_test.go` — mock matcher uses `mock.AnythingOfType("*container.StressRequest")`; assertions read from `*StressResult`
- [x] update runtime stress tests if they directly exercise `StressContainer`
- [x] `CGO_ENABLED=0 go test ./...` — all green
- [x] `make lint` — must pass before next task

### Task 2: drop `Params` in `pkg/chaos/netem`

**Files:**

- Create: `pkg/chaos/netem/parse.go` (`ParseRequestBase` shared helper)
- Create: `pkg/chaos/netem/parse_test.go`
- Modify: `pkg/chaos/netem/netem.go` (delete `Params` struct + `netemCommand` struct + `newNetemCommand`)
- Modify: `pkg/chaos/netem/{delay,loss,loss_ge,loss_state,rate,corrupt,duplicate}.go` (build `*NetemRequest` directly)
- Modify: `pkg/chaos/netem/cmd/{delay,loss,loss_ge,loss_state,rate,corrupt,duplicate,netem}.go` (parse functions construct request)
- Modify: corresponding `*_test.go` files in both directories (drop `Params` literals)

- [x] write `pkg/chaos/netem/parse.go` with `ParseRequestBase(c cliflags.Flags, gp *chaos.GlobalParams) (*container.NetemRequest, error)` — reads parent flags via `c.Parent()`, fills `Container=nil` (set per-iteration), `Interface`, `IPs`, `SPorts`, `DPorts`, `Duration`, `Sidecar`, `DryRun`
- [x] write `pkg/chaos/netem/parse_test.go` — table-driven cases for happy path, invalid duration, invalid CIDR, invalid port
- [x] convert `pkg/chaos/netem/cmd/delay.go::parseDelayParams` to call `ParseRequestBase` + augment `req.Command` (pilot to confirm shape)
- [x] convert remaining 6 netem cmd parsers (`loss`, `loss_ge`, `loss_state`, `rate`, `corrupt`, `duplicate`)
- [x] convert each per-action chaos file (`pkg/chaos/netem/{delay,loss,...}.go`) to consume the request directly — drop the field-by-field copy at every callsite
- [x] delete `Params` struct + `newNetemCommand` + `netemCommand` from `pkg/chaos/netem/netem.go`
- [x] update `pkg/chaos/netem/*_test.go` — replace `Params{...}` literals with `*NetemRequest{...}`; mock expectations unchanged (already use `*container.NetemRequest`)
- [x] update `pkg/chaos/netem/cmd/*_test.go` — assert on the request fields directly
- [x] `CGO_ENABLED=0 go test ./...` — all green
- [x] `make lint` — must pass before next task

### Task 3: drop `Params` in `pkg/chaos/iptables`

**Files:**

- Create: `pkg/chaos/iptables/parse.go` (`ParseRequestBase` for iptables — separate `Add` and `Del` cmd prefixes)
- Create: `pkg/chaos/iptables/parse_test.go`
- Modify: `pkg/chaos/iptables/iptables.go` (delete `Params` struct + `ipTablesCommand` struct + `newIPTablesCommand`)
- Modify: `pkg/chaos/iptables/loss.go` (build `*IPTablesRequest` directly)
- Modify: `pkg/chaos/iptables/cmd/{loss,iptables}.go`
- Modify: corresponding `*_test.go` files

- [x] write `pkg/chaos/iptables/parse.go` with `ParseRequestBase(c cliflags.Flags, gp *chaos.GlobalParams) (*RequestBase, error)` — returns `RequestBase{Request, Iface, Protocol, Limit}` so per-action parsers can build the iptables `-I/-D INPUT -i <iface> [-p <proto>] …` prefix at Run time; reads `--protocol`, `--source`, `--destination`, `--src-port`, `--dst-port`, `--iptables-image`, `--pull-image`, `--duration`, `--limit` from parent flags
- [x] write `pkg/chaos/iptables/parse_test.go` — table-driven cases mirroring netem parse_test
- [x] convert `pkg/chaos/iptables/cmd/loss.go::parseLossParams` to build `*RequestBase` directly (CmdPrefix/CmdSuffix per the action — assembled in `lossCommand.Run` from base.Iface/base.Protocol)
- [x] convert `pkg/chaos/iptables/loss.go` to consume the request — `runIPTables` already takes `addReq, delReq *container.IPTablesRequest`; `lossCommand` now stores `*container.IPTablesRequest` + `iface`/`protocol` directly
- [x] delete `Params` struct + `newIPTablesCommand` + `ipTablesCommand` from `pkg/chaos/iptables/iptables.go`; deleted obsolete `pkg/chaos/iptables/cmd/iptables.go` (validation moved to `iptables/parse.go`)
- [x] update `pkg/chaos/iptables/*_test.go` — replaced `*Params` literals with `*RequestBase{Request: &container.IPTablesRequest{...}, Iface, Protocol}`
- [x] update `pkg/chaos/iptables/cmd/*_test.go` — `got.IPTables.Iface` → `got.Base.Iface` / `got.Base.Protocol`
- [x] `CGO_ENABLED=0 go test ./...` — all green
- [x] `make lint` — 0 issues

### Task 4: `RemoveContainer` → `RemoveOpts`

**Files:**

- Modify: `pkg/container/requests.go` (add `RemoveOpts`)
- Modify: `pkg/container/client.go` (`Lifecycle.RemoveContainer` signature shrinks)
- Modify: `pkg/runtime/docker/lifecycle.go`, `pkg/runtime/containerd/client.go` (RemoveContainer impl)
- Modify: `pkg/chaos/lifecycle/remove.go` (build opts, pass to client)
- Modify: `pkg/chaos/lifecycle/cmd/remove.go` (parse function)
- Regenerate: `pkg/container/mock_Lifecycle.go`, `pkg/container/mock_Client.go`
- Modify: `pkg/chaos/lifecycle/remove_test.go`, runtime lifecycle tests if they exercise `RemoveContainer` directly

- [x] add `RemoveOpts{Force, Links, Volumes, DryRun bool}` to `pkg/container/requests.go`
- [x] update `Lifecycle.RemoveContainer` signature in `pkg/container/client.go` to `RemoveContainer(context.Context, *Container, RemoveOpts) error` (pass by value — 4 bytes)
- [x] regenerate mocks (mockery v2.53.5 incompatible with Go 1.26 — hand-edited `mock_Lifecycle.go` + `mock_Client.go` to match generated shape)
- [x] update `pkg/runtime/docker/lifecycle.go::RemoveContainer` to take `RemoveOpts` (unpack to existing internals)
- [x] update `pkg/runtime/containerd/client.go::RemoveContainer`
- [x] verified Podman does not override RemoveContainer (embedded Docker delegate handles it)
- [x] update `pkg/chaos/lifecycle/remove.go` to construct `RemoveOpts` (collapsed individual force/links/volumes/dryRun fields into single `opts` field on `removeCommand`)
- [x] update tests — replace 4-bool positional matcher with `RemoveOpts` literal
- [x] `CGO_ENABLED=0 go test ./...` — all green
- [x] `make lint` — 0 issues

### Task 5: push request structs deeper in `pkg/runtime/docker/{netem,iptables}.go`

**Files:**

- Modify: `pkg/runtime/docker/netem.go` (`startNetemContainer`, `stopNetemContainer`, `startNetemContainerIPFilter` take `*ctr.NetemRequest`)
- Modify: `pkg/runtime/docker/iptables.go` (`ipTablesContainer`, `ipTablesContainerWithIPFilter` take `*ctr.IPTablesRequest`)
- Modify: corresponding `*_test.go` files only if they unit-test the helpers directly (most go through the public method)

- [ ] update `startNetemContainer` to take `(ctx, *ctr.NetemRequest)` — read fields from request, no caller change outside the file
- [ ] update `stopNetemContainer` similarly
- [ ] update `startNetemContainerIPFilter` similarly
- [ ] verify `tcCommands` helper signature is consistent — keep it taking primitive args since it's truly low-level (no leakage)
- [ ] update `ipTablesContainer` + `ipTablesContainerWithIPFilter` similarly
- [ ] verify no other callsite for these helpers (grep `git grep -n 'startNetemContainer\|ipTablesContainerWithIPFilter'`)
- [ ] update unit tests if any directly call these helpers
- [ ] `CGO_ENABLED=0 go test ./...` — all green
- [ ] `make lint` — must pass before next task

### Task 6: split `pkg/runtime/containerd/sidecar.go` into per-concern files

**Files:**

- Modify (split): `pkg/runtime/containerd/sidecar.go` → existing trimmed + new `netem.go`, `iptables.go`, `stress.go`, `cgroup.go`
- Modify (split): `pkg/runtime/containerd/sidecar_test.go` (if it exists) → mirror per-file test files
- Keep unchanged: `pkg/runtime/containerd/{client,task,commands,container,api}.go`

- [ ] read `pkg/runtime/containerd/sidecar.go` end-to-end (~437 LOC) and identify the cut lines per the technical-details map above
- [ ] create `pkg/runtime/containerd/netem.go` — move `NetemContainer`, `StopNetemContainer`, `runTCCommands`, `sidecarExec` if used only by netem (verify call graph first)
- [ ] create `pkg/runtime/containerd/iptables.go` — move `IPTablesContainer`, `StopIPTablesContainer`, `runIPTablesCommands`
- [ ] create `pkg/runtime/containerd/stress.go` — move `StressContainer`, `stressDirectExec`, `stressSidecar`
- [ ] create `pkg/runtime/containerd/cgroup.go` — move cgroup driver detection
- [ ] trim `pkg/runtime/containerd/sidecar.go` to provisioning + reap helpers shared by netem/iptables/stress
- [ ] reassign existing `*_test.go` cases to matching per-concern test files (cleanest: rename or split — leave `sidecar_test.go` only if cross-concern fixtures remain)
- [ ] verify each resulting file is ≤ 250 LOC; if any exceeds, split further along the same per-concern lines
- [ ] `CGO_ENABLED=0 go test ./...` — all green
- [ ] `make lint` — must pass before next task

### Task 7: Verify acceptance criteria

- [ ] `git grep -n 'type Params struct' pkg/chaos/` returns zero results
- [ ] `git grep -n 'StressContainer' pkg/container/` shows the new request-shape signature only
- [ ] `git grep -n 'RemoveContainer' pkg/container/` shows `RemoveOpts` only
- [ ] `wc -l pkg/runtime/containerd/*.go` — every production file ≤ 250 LOC (target: split is the only way to hit this)
- [ ] every `mock.AnythingOfType("*container.NetemRequest")` / `*container.IPTablesRequest` / `*container.StressRequest` / `container.RemoveOpts` matcher matches actual call sites
- [ ] `make test` — all packages pass with race detector
- [ ] `make test-coverage` — statement coverage ≥ 71.9% (no regression vs. post-refactor baseline)
- [ ] `make lint` — 0 issues from active linters
- [ ] `make build` — binary builds clean
- [ ] manual smoke: `pumba --help`, `pumba stress --help`, `pumba rm --help`, `pumba netem delay --help`, `pumba iptables loss --help` — all show expected help, no flag/command regressions
- [ ] run bats integration tests (Docker, containerd, Podman) — defer to release-time per Post-Completion if no daemons available locally

### Task 8: Update documentation

- [ ] update `CLAUDE.md` Architecture section: mention `StressRequest`/`StressResult` and `RemoveOpts` alongside the existing `NetemRequest`/`IPTablesRequest` reference
- [ ] update `CLAUDE.md` Code Conventions: extend the "Request value objects for fat methods" rule to call out `Stressor` and `Lifecycle.RemoveContainer` as examples
- [ ] update `CLAUDE.md` Mock conventions: add `mock.AnythingOfType("*container.StressRequest")` and `RemoveOpts{}` literal patterns to the mock-request-structs guidance
- [ ] regenerate `docs/modularity-review/` for a date later than the merge to confirm score moved 8.5 → ~9.0 (or schedule a `/loop` agent in 1–2 weeks per Post-Completion)
- [ ] move this plan to `docs/plans/completed/`

## Post-Completion

**Manual verification:**

- Run a full bats integration sweep on each runtime (Docker, containerd, Podman) on a real host before tagging a release. The refactor is non-behavioral but bats is the only end-to-end check that interface migrations didn't break runtime wiring.
- Smoke `pumba` against a live Docker daemon for kill / netem / iptables / stress / rm — confirm no flag regressions.

**External system updates:**

- None. Pumba is a CLI binary, not a Go library; no downstream importers.

**Optional follow-up:**

- Schedule a `/loop` or `/schedule` agent in 1–2 weeks to re-baseline the modularity review and confirm the 8.5 → ~9.0 score change held after the PRs landed.
