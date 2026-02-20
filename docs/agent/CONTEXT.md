# Pumba Agent Context

## Project Overview
Pumba is a chaos testing tool for Docker containers. It intentionally degrades network performance, stops/kills containers, and pauses/unpauses them to test system resilience.

## Architecture (Refactor: Runtime Abstraction)
We are currently refactoring Pumba to support multiple container runtimes (Docker, Containerd, Podman).

### Core Layers
1.  **`pkg/chaos/`**: Chaos logic (Network, Stress, Docker lifecycle). Depends on `pkg/container` interfaces.
2.  **`pkg/container/`**: **Interfaces only**. Defines the contract for runtimes.
    *   `Client`: Composite interface (Lister, Lifecycle, Executor, Netem, IPTables, Stressor).
    *   `Container`: Generic container model (ID, Name, Image, etc.).
    *   `FilterFunc`: Container filtering logic.
3.  **`pkg/runtime/`**: Runtime implementations.
    *   **`docker/`**: The Docker SDK implementation (legacy code moved here).
    *   **`containerd/`**: (Future) Containerd implementation.
4.  **`cmd/`**: CLI entrypoints.
    *   `main.go`: Main Pumba CLI.
    *   `cg-inject/`: Helper binary for Cgroups v2 stress injection.

## Key Invariants
- **Interfaces first**: Chaos commands MUST NOT depend on concrete runtime implementations (e.g., `dockerClient`). They must use `container.Client` or specific interfaces like `container.Lister`.
- **Cgroups v2**: Stress testing now supports Cgroups v2 via `cg-inject`. Logic is in `pkg/chaos/stress` and `pkg/runtime/docker`.
- **No manual deps**: Use `go mod tidy`.

## Directory Map
- `docs/agent/plans/`: Active and completed engineering plans.
- `docs/`: User-facing documentation.
- `tests/`: Integration tests (`bats`).
- `docker/`: Dockerfiles for build/release.
