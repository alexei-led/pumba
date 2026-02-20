# Plan: Fix stress cgroup paths for Kubernetes

## Context

PR #291 (`fix/stress-cgroups-v2` branch) hardcodes cgroup parent as `/docker/<containerID>` (cgroupfs) or `system.slice` (systemd). This works on standalone Docker but fails on Kubernetes where containers live under paths like `/kubepods/burstable/pod<uid>/<containerID>`.

The fix: use `ContainerInspect` to get the target container's actual cgroup parent path from the Docker API, instead of constructing it from conventions.

### Branch: `fix/stress-cgroups-v2` (continue from existing work)

## Validation Commands

```bash
go build ./...
go vet ./...
golangci-lint run ./...
CGO_ENABLED=1 go test -count=1 -race ./pkg/container/ ./cmd/cg-inject/
```

## Tasks

### Task 1: Use ContainerInspect to resolve cgroup parent path

In `pkg/container/docker_client.go`:

- [x] Add a `targetCgroupParent(ctx context.Context, targetID string) (string, error)` method on `dockerClient` that calls `client.containerAPI.ContainerInspect(ctx, targetID)` and returns `inspect.HostConfig.CgroupParent` — this is the cgroup parent Docker assigned to the target container
- [x] However, `HostConfig.CgroupParent` is the _parent_ of the container's cgroup, not the container's own cgroup. The actual container cgroup is `<CgroupParent>/<containerID>` for cgroupfs or `<CgroupParent>/docker-<containerID>.scope` for systemd. So:
  - For cgroupfs driver: if `CgroupParent` is empty, the container is at `/docker/<id>`, otherwise at `<CgroupParent>/<id>` — so the stress sidecar's `--cgroup-parent` should be `<CgroupParent>/<id>` (or `/docker/<id>` if empty)
  - For systemd driver: the container scope is `docker-<id>.scope` under whatever slice, so `--cgroup-parent` should target the same slice
- [x] Actually, the simplest correct approach: inspect the target container, read its `HostConfig.CgroupParent`. If non-empty, the target sits under that parent. The stress sidecar should use the **same parent + targetID** pattern. If empty, fall back to current behavior (`/docker/<id>` for cgroupfs, `system.slice` for systemd)
- [x] Update `stressContainerCommand()` to call `targetCgroupParent()` and pass the resolved path to `stressContainerConfig()`
- [x] Update `stressContainerConfig()` signature: replace `driver string` with `cgroupParent string` for the default mode — the caller resolves the full path, the config builder just uses it
- [x] For inject-cgroup mode: pass both `driver` and `targetID` as before (cg-inject resolves its own path inside the container)
- [x] For inject-cgroup mode on K8s: the cg-inject binary also needs the real cgroup path. Add a `--cgroup-path` flag to cg-inject that accepts the full path to the target's cgroup.procs file, bypassing driver-based path construction. When pumba knows the actual path (from inspect), it should pass it directly.

### Task 2: Extend cg-inject with --cgroup-path flag

In `cmd/cg-inject/main.go`:

- [x] Add `--cgroup-path` flag (optional) — when set, use this exact path(s) instead of constructing from `--target-id` + `--cgroup-driver`
- [x] If `--cgroup-path` is set, skip `--target-id` requirement (target-id is only needed for path construction)
- [x] Validate that `--cgroup-path` and `--target-id` are mutually exclusive (or at least `--cgroup-path` takes precedence)
- [x] For cgroups v1: `--cgroup-path` can be specified multiple times (one per controller), or accept a base path and append controller names
- [x] Simpler approach: `--cgroup-path` is the base cgroup path (e.g., `/kubepods/burstable/pod<uid>/<id>`). cg-inject appends `/cgroup.procs` for v2, or iterates controllers for v1 under `/sys/fs/cgroup/<ctrl>/<path>/cgroup.procs`
- [x] Update `parseArgs` and `run` functions
- [x] Update tests in `cmd/cg-inject/main_test.go`

### Task 3: Wire K8s-aware cgroup resolution in stressContainerCommand

In `pkg/container/docker_client.go`:

- [x] In `stressContainerCommand()`, after getting the driver, call `ContainerInspect` on the target to get its actual cgroup info
- [x] For default (child-cgroup) mode: construct `cgroupParent` from inspect data. If `inspect.HostConfig.CgroupParent` is non-empty, use it + targetID structure. If empty, use current fallback
- [x] For inject-cgroup mode: if we can determine the full cgroup path from inspect, pass `--cgroup-path` to cg-inject instead of `--target-id` + `--cgroup-driver`. Fall back to current behavior if inspect doesn't reveal the path
- [x] Update `stressContainerConfig()` to accept the resolved cgroup parent string for default mode, and optional cgroup path for inject mode

### Task 4: Unit tests for K8s cgroup paths

In `pkg/container/stress_test.go`:

- [x] Add test "stress container with K8s cgroup parent (cgroupfs)": mock ContainerInspect returning `CgroupParent: "/kubepods/burstable/pod-abc123"`, verify the stress sidecar gets `CgroupParent: "/kubepods/burstable/pod-abc123/<targetID>"`
- [x] Add test "stress container with K8s cgroup parent (systemd)": mock with systemd-style K8s parent
- [x] Add test "stress container with empty cgroup parent (standalone Docker)": verify fallback to `/docker/<id>`
- [x] Add test "inject-cgroup with K8s cgroup path": verify `--cgroup-path` is passed to cg-inject entrypoint
- [x] Update existing tests that mock ContainerInspect if signature changes

In `cmd/cg-inject/main_test.go`:

- [x] Add tests for `--cgroup-path` flag parsing
- [x] Add test for `--cgroup-path` with v1 (multiple controllers)
- [x] Add test for `--cgroup-path` with v2 (single path)
- [x] Add test for mutual exclusivity / precedence with `--target-id`

### Task 5: Update documentation

- [x] In `docs/stress-testing.md`: add note about Kubernetes cgroup path resolution — pumba auto-detects the target's cgroup hierarchy
- [x] In `docs/deployment.md`: update K8s stress example if needed
- [x] Update `deploy/pumba_kube_stress.yml`: remove docker.sock mount if no longer needed for stress (note: pumba still needs it for container discovery, so keep it but remove the comment about SYS_ADMIN)
