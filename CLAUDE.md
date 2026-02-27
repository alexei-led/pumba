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
