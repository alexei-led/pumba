# Pumba Development Guide

## Build & Test Commands

- **Build:** `make build` (builds for current TARGETOS/TARGETARCH)
- **Full pipeline:** `make all` (format → lint → test → build)
- **Unit tests:** `make test` (requires `CGO_ENABLED=1` for race detector)
- **Test with coverage:** `make test-coverage`
- **Race detector:** `make test-race` (linux/amd64 only)
- **Lint:** `make lint` (runs golangci-lint with `.golangci.yaml`)
- **Format:** `make fmt`
- **Cross-compile:** `make release` (darwin/linux/windows × amd64/arm64)
- **Integration tests:** `make integration-tests` (requires Docker, uses bats)
- **All integration tests:** `make integration-tests-all` (includes stress tests)
- **Advanced Go integration tests:** `make integration-tests-advanced` (Go-based tests in `tests/integration/`)
- **Generate mocks:** `make mocks` (uses mockery)
- **Unit tests in sandbox:** `CGO_ENABLED=0 go test ./...` (skips race detector, works without CGO toolchain)

## Integration Testing

Integration tests use [bats](https://github.com/bats-core/bats-core):

- Tests are in `tests/*.bats` with helpers in `tests/test_helper.bash`
- **Run all tests locally (recommended):** `colima ssh -- sudo bats tests/*.bats tests/containerd_*.bats`
  - Colima VM has native Docker + containerd sockets; `sudo` is required for containerd sidecar tests
- **Docker tests only (via Docker image):** `docker run --rm -v /var/run/docker.sock:/var/run/docker.sock --entrypoint bats pumba:test tests/*.bats`
- **Containerd tests only:** `colima ssh -- sudo bats tests/containerd_*.bats`
- **Podman tests only (macOS, inside podman machine VM):** `podman machine ssh sudo bats tests/podman_*.bats`
- **Podman tests only (Linux, rootful):** `sudo bats tests/podman_*.bats`
- CI builds a Docker image (`pumba:test` target `integration-tests`) and runs bats inside it
- Rebuild test image after code changes: `docker build --target integration-tests -t pumba:test -f docker/Dockerfile .`
- Copy updated binary to Colima: `colima ssh -- sudo cp /Users/alexei/workspace/pumba/.bin/linux/pumba /usr/local/bin/pumba`
- **Bats teardown:** Use `sudo pkill -f "pumba.*<container-name>"` to stop background pumba processes; `kill %1` for job control fallback

## Technical Stack

- **Go version:** 1.26 (see go.mod)
- **CLI framework:** `github.com/urfave/cli` (v1)
- **Docker SDK:** `github.com/docker/docker` v28.5.2
- **Containerd SDK:** `github.com/containerd/containerd/v2` (containerd runtime support)
- **Error handling:** `github.com/pkg/errors` (deprecated — migrate to `fmt.Errorf` with `%w`)
- **Logging:** `github.com/sirupsen/logrus`
- **Testing:** `github.com/stretchr/testify` (assert, mock, require)
- **Mocking:** `github.com/vektra/mockery`
- **Linting:** golangci-lint with `.golangci.yaml`
- **CI:** GitHub Actions (build.yaml, release.yaml, codeql-analysis.yml, nettools-images.yaml)

## Project Structure

```
cmd/main.go            — CLI entry point, all command/flag definitions
pkg/
  chaos/
    command.go         — ChaosCommand interface, scheduling/interval runner
    docker/            — Docker chaos actions (kill, stop, pause, rm, exec, restart)
    docker/cmd/        — CLI command builders for docker chaos actions
    netem/             — Network emulation (delay, loss, corrupt, duplicate, rate, loss_ge, loss_state)
    netem/cmd/         — CLI command builders for netem
    iptables/          — iptables-based packet filtering
    iptables/cmd/      — CLI command builders for iptables
    stress/            — stress-ng based resource stress
    stress/cmd/        — CLI command builder for stress
  container/           — Container model, interfaces (Client, Lister, Lifecycle, Executor, Netem, etc.), filtering
  runtime/
    docker/            — Docker runtime implementation of container.Client
    containerd/        — Containerd runtime implementation of container.Client
    podman/            — Podman runtime implementation (embeds Docker client, overrides stress cgroup resolution + rootless guards)
  util/                — Shared utilities (IP/port parsing)
mocks/                 — Generated mock files (mockery)
tests/                 — Bats integration tests
docker/                — Dockerfiles (main, alpine-nettools, debian-nettools)
deploy/                — K8s/OpenShift deployment manifests
examples/              — Demo scripts
```

## Architecture

- **Container interfaces** (`pkg/container/client.go`): Focused sub-interfaces (Lister, Lifecycle, Executor, Netem, IPTables, Stressor) composed into a unified Client interface
- **Docker runtime** (`pkg/runtime/docker/`): Docker SDK implementation of container.Client
- **Containerd runtime** (`pkg/runtime/containerd/`): Containerd implementation of container.Client (socket: `/run/containerd/containerd.sock`, namespace: `k8s.io`)
- **Podman runtime** (`pkg/runtime/podman/`): Podman implementation of container.Client; reuses the Docker SDK against Podman's Docker-compat socket and overrides only what diverges (stress cgroup resolution + rootless guards). Socket auto-detected from `$CONTAINER_HOST`, `$PODMAN_SOCK`, `podman machine inspect`, `/run/podman/podman.sock`, and `$XDG_RUNTIME_DIR/podman/podman.sock`; override via `--podman-socket`. Cgroup parent/leaf derived host-side from `/proc/<pid>/cgroup` of the target container (see `pkg/runtime/podman/cgroup.go`) — pumba must run on the same kernel as the targets.
- **Chaos commands**: Each action implements `ChaosCommand` interface with `Run(ctx, random)` method
- **Network emulation**: Executes `tc` commands inside a sidecar container via Docker exec
- **Stress testing**: Two modes — (1) default child-cgroup mode places stress-ng sidecar in target's cgroup via Docker's `--cgroup-parent`; (2) inject-cgroup mode (`--inject-cgroup`) uses the `cg-inject` binary (shipped in [`ghcr.io/alexei-led/stress-ng`](https://github.com/alexei-led/stress-ng)) to write sidecar PID into target's `cgroup.procs` for shared resource accounting
- **Target selection**: Container names (exact), comma-separated lists, or `re2:` prefixed regex patterns
- **Label filtering**: `--label key=value` flags for container selection
- **Interval mode**: `--interval` flag for recurring chaos on a schedule

## Code Conventions

- **Error wrapping:** Currently uses `github.com/pkg/errors` — migrate to `fmt.Errorf("...: %w", err)`
- **Interfaces:** Define interfaces for testability (Client in `pkg/container/client.go`)
- **Mocking:** testify mocks, generated by mockery. Mock files in `mocks/` and `pkg/container/mock_*.go`
- **Mock constructor:** Always use `container.NewMockClient(t)` — never `new(container.MockClient)`; auto-asserts expectations on test cleanup
- **Mock typed nils:** Use `([]*net.IPNet)(nil)` and `[]string(nil)` for nil slice args in EXPECT() calls — plain `nil` causes type mismatch
- **Mock context:** Use `mock.Anything` only for `context.Context` args; use exact values for all business params
- **Mock random container:** Use `mock.AnythingOfType("*container.Container")` + `.Once()` when only one of N containers is targeted
- **Logging:** logrus with structured fields. Log level set via `--log-level` flag
- **Constants:** Magic numbers use `mnd` linter — use named constants
- **Cleanup defer survives cancellation:** Use `context.WithoutCancel(ctx)` with a timeout so defers (e.g. sidecar cleanup) run even when the caller cancels
- **gocyclo violations (limit: 15):** Extract loop bodies or complex branches into named helper methods
- **funlen violations (limit: 105):** Extract initialization blocks (flags, config setup) into named helper functions
- **Default branch:** `master`
- **NEVER add AI co-author to git commits**

## Unit Test Coverage Strategy

- **Skip from unit tests** (covered by integration tests or untestable without real runtime):
  `cmd/main.go`, `mocks/`, `NewClient`/`Close`, `sidecar.go`, `*/cmd/` flag builders
- **Run() method variants:** Always add NoContainers + DryRun + WithRandom test cases
- **Run unit tests in sandbox:** `CGO_ENABLED=0 go test ./...` (no CGO needed outside CI)

## Gotchas

- **PID 1 signal handling:** `sleep`/`tail -f /dev/null` as container PID 1 ignores SIGTERM — use `top` in bats tests that kill with SIGTERM
- **iptables flag ordering:** `--source`, `--destination`, `--src-port`, `--dst-port` are on the `iptables` parent command, NOT on the `loss` subcommand
- **exec command parsing:** `--command "touch /tmp/foo"` is wrong (treated as binary name with spaces); use `--command "touch" --args "/tmp/foo"`
- **Containerd sidecar requires root:** netem/iptables tests on containerd need `sudo pumba` — overlayfs mounts for sidecar creation require root in Colima VM
- **Containerd namespaces:** Docker-managed containers live in `moby` namespace; pure containerd in `default`; Kubernetes in `k8s.io`
- **Podman requires rootful for netem/iptables/stress:** rootless Podman is detected at `NewClient` time from `Info.SecurityOptions` and every netem/iptables/stress call fails fast with a message pointing at `podman machine set --rootful` (macOS) or the rootful systemd unit (Linux). Rootless support is out of scope — would need slirp4netns/pasta netns handling and user-namespace cgroup math.
- **Podman cgroup leaf naming:** Podman uses `libpod-<id>.scope` (or `libpod-<id>.scope/container` with init sub-cgroup) under systemd, vs Docker's `docker-<id>.scope`. Pointing `--runtime docker` at Podman's compat socket silently places stress-ng sidecars in the wrong cgroup; `--runtime podman` derives the correct path.
- **Podman cgroup resolution reads host-side `/proc/<pid>/cgroup`:** containers launched under Podman's default `--cgroupns=private` see only `0::/` or `0::/container` from inside the container, so we read from pumba's own view of `/proc` (requires shared kernel with targets). On macOS this means running pumba **inside the `podman machine` VM** — the same pattern used for containerd testing in Colima. See `pkg/runtime/podman/cgroup.go` and the `cgroupReader` var for the test-injectable hook.
- **`ContainerExecStart` empty options breaks on Podman:** Docker's `ContainerExecStart(ctx, id, ExecStartOptions{})` with no attach/detach flags is accepted by Docker (implicit sync via HTTP hijack) but rejected by Podman's compat API with _"must provide at least one stream to attach to"_. The `runExecAttached` helper in `pkg/runtime/docker/docker.go` goes through `ContainerExecAttach` + drain + inspect — works on both. When writing new exec-driven code, never call `ContainerExecStart` with empty options; use the helper.
- **tc/iptables sidecar cleanup must survive ctx cancel:** `tcContainerCommands`/`ipTablesContainerCommands` use `removeSidecar` (wraps `ContainerRemove` with `context.WithoutCancel(ctx)` + 15 s timeout). Without this, SIGTERM to pumba during the narrow window after tc exec and before sidecar reap leaks the sidecar AND leaves the netem qdisc on the target's netns.
- **Sidecar `StopSignal: "SIGKILL"`:** `tail -f /dev/null` as PID 1 ignores SIGTERM. Podman's `DELETE ?force=1` sends SIGTERM and waits the full `StopTimeout` (10 s) before escalating — that's 10 s per sidecar reap on every netem/iptables call. Config sets `StopSignal: "SIGKILL"` so force-remove is immediate.
- **Podman inject-cgroup needs SYS_ADMIN + `label=disable` + nested leaf:** cg-inject writes the sidecar's PID into the target's `cgroup.procs`. Three gotchas stack on cgroup v2 + systemd: (1) the target's scope may have a nested `container/` leaf (Podman's libpod init sub-cgroup) — cgroup v2's "no internal processes" rule means we must target `libpod-<id>.scope/container/cgroup.procs`, not the outer scope. `pkg/runtime/podman/stress.go::resolveCgroup` reads `/proc/<pid>/cgroup` plus `os.Stat` to pick between them. (2) Writing across sibling scopes requires `CAP_SYS_ADMIN` in the initial user namespace. (3) SELinux `container_t` blocks cgroup writes even with SYS_ADMIN; `SecurityOpt: ["label=disable"]` on the sidecar bypasses this. All three are required on Fedora CoreOS / RHEL hosts.
- **Podman 4.9.x transient `/container` sub-cgroup races inject-cgroup:** Stock Podman 4.9.x on Ubuntu 24.04 creates `<scope>/container` during libpod init, migrates PID 1 to `<scope>` shortly after, and `rmdir`s `/container` mid-flight. The resolver's `os.Stat` check can pass and then `/container` is gone by the time cg-inject opens/writes `cgroup.procs`, yielding ENOENT on write (documented Podman behavior — see [podman#20910](https://github.com/containers/podman/issues/20910)). Podman 5.x (podman machine, Fedora CoreOS) keeps `/container` stable. The inject-cgroup bats test lives in `tests/skip_ci/podman_stress_inject_cgroup.bats` and is excluded from the GH CI glob `tests/podman_*.bats` for this reason. Proper fix: retry-on-ENOENT inside cg-inject (stress-ng sibling repo), then move the test back.
- **Ephemeral tc sidecar breaks poll-based bats assertions:** the tc sidecar lives only for the duration of tc exec (sub-second) before `removeSidecar` reaps it. Bats tests that `podman ps`-polled for the sidecar (to verify the skip label) race the lifecycle. Rewrote `tests/podman_sidecar.bats` tests 68/69 to assert invariants instead: (a) create a fake container with the skip label manually and confirm pumba's re2: regex doesn't target it; (b) verify netem rules are removed from the TARGET netns after SIGTERM (via `nsenter`), not whether the sidecar itself was tracked.
