# Contributing to Pumba

Thank you for your interest in contributing to Pumba! This guide covers how to set up your development environment, build the project, run tests, and submit contributions.

## Prerequisites

- **Go 1.26+** — see [go.dev/dl](https://go.dev/dl/) for installation
- **Docker** — required for integration tests, building Docker images, and running Pumba itself
- **golangci-lint** — installed automatically via `make setup-tools`
- **mockery** — for generating test mocks (`make setup-mockery`)

## Getting Started

```sh
git clone https://github.com/alexei-led/pumba.git
cd pumba
```

## Building

### Build locally

```sh
# Build binary for your current OS/architecture
make build

# The binary is placed in .bin/pumba
.bin/pumba --version
```

### Build with Docker

No local Go installation required:

```sh
DOCKER_BUILDKIT=1 docker build -t pumba -f docker/Dockerfile .
```

### Cross-compile for all platforms

```sh
# Builds for darwin/linux/windows × amd64/arm64
make release
```

## Running Tests

### Unit tests

```sh
# Run all unit tests (requires CGO_ENABLED=1 for race detector)
make test

# Run with race detector (linux/amd64 only)
make test-race

# Run with coverage report
make test-coverage
```

### Integration tests

Integration tests use [bats](https://github.com/bats-core/bats-core) and require Docker:

```sh
# Run core integration tests
make integration-tests

# Run all integration tests including stress tests
make integration-tests-all
```

### Linting

```sh
# Run golangci-lint (installs if needed)
make lint

# Format code
make fmt
```

### Full pipeline

```sh
# Format → Lint → Test → Build
make all
```

## Project Structure

```
cmd/main.go            — CLI entry point and command definitions
pkg/
  chaos/
    command.go         — ChaosCommand interface, scheduling/interval runner
    docker/            — Docker chaos actions (kill, stop, pause, rm, exec, restart)
    docker/cmd/        — CLI command builders for docker chaos actions
    netem/             — Network emulation (delay, loss, corrupt, duplicate, rate)
    netem/cmd/         — CLI command builders for netem
    iptables/          — iptables-based packet filtering
    iptables/cmd/      — CLI command builders for iptables
    stress/            — stress-ng resource stress testing
    stress/cmd/        — CLI command builder for stress
  container/           — Docker client abstraction, container types, filtering
  util/                — Shared utilities (IP/port parsing)
mocks/                 — Generated mock files (mockery)
tests/                 — Bats integration tests
docker/                — Dockerfiles (main, alpine-nettools, debian-nettools)
deploy/                — Kubernetes and OpenShift deployment manifests
docs/                  — Detailed documentation
examples/              — Demo scripts
```

## Generating Mocks

Test mocks are generated using [mockery](https://github.com/vektra/mockery):

```sh
make mocks
```

Mock files live in `mocks/` and `pkg/container/`. Do not edit generated mock files by hand.

## How to Contribute

### Reporting Issues

- Search [existing issues](https://github.com/alexei-led/pumba/issues) first
- Include Pumba version, Docker version, OS/architecture, and steps to reproduce

### Submitting Pull Requests

1. Fork the repository and create a feature branch from `master`
2. Make your changes with clear, focused commits
3. Ensure all checks pass: `make fmt && make lint && make test`
4. Add or update tests for any new functionality
5. Open a pull request against `master` with a clear description of the change

### Code Style

- Follow standard Go conventions (`gofmt`, `go vet`)
- The project uses golangci-lint with the config in `.golangci.yaml`
- Define interfaces for testability (see `pkg/container/client.go`)
- Use `fmt.Errorf("...: %w", err)` for error wrapping
- Use structured logging with logrus

## License

By contributing, you agree that your contributions will be licensed under the [Apache License v2.0](LICENSE).
