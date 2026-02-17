# Plan: Enhanced stress command with dual cgroup modes (v1 + v2)

## Context

PR #291 (`fix/stress-cgroups-v2` branch) already implements the **child cgroup** approach using `--cgroup-parent`. This plan adds a **same-cgroup injection** mode using `cg-inject`, a minimal Go binary that writes the stress-ng PID into the target container's cgroup. Both modes work on cgroups v1 and v2 without requiring privileged mode or any Linux capabilities.

### Branch: `fix/stress-cgroups-v2` (continue from existing PR #291)

## Validation Commands

```bash
go build ./...
go vet ./...
golangci-lint run ./...
CGO_ENABLED=1 go test -count=1 -race ./pkg/container/ ./pkg/chaos/stress/
```

## Research Summary

Two working approaches for stress on cgroups v2:

| Mode                | How                                                                 | Cgroup placement              | OOM behavior                      | Security                                                                      |
| ------------------- | ------------------------------------------------------------------- | ----------------------------- | --------------------------------- | ----------------------------------------------------------------------------- |
| **child** (default) | `--cgroup-parent=/docker/<id>`                                      | New child cgroup under target | OOM kills stress-ng only (safe)   | No caps, no mounts                                                            |
| **same-cgroup**     | `--cgroupns=host` + cg-inject writes PID to target's `cgroup.procs` | Same cgroup as target         | Shared OOM risk (realistic chaos) | No caps needed, needs `--cgroupns=host` + `/sys/fs/cgroup` mount (read-write) |

Key findings:

- `--cgroupns=host` is required for same-cgroup — without it the container can't see host cgroup hierarchy
- NO extra capabilities needed — verified with `--cap-drop=ALL`
- `cg-inject` is a tiny static Go binary (~1.6MB) that: finds target cgroup path → writes own PID to `cgroup.procs` → exec's stress-ng
- Works on both cgroupfs and systemd cgroup drivers

## Architecture

### New CLI flag

Add `--inject-cgroup` boolean flag to `pumba stress` command. Default: `false` (child cgroup mode).

When `--inject-cgroup=true`:

- Use a stress image that contains both `cg-inject` and `stress-ng` (default: `ghcr.io/alexei-led/stress-ng:latest` — needs to be rebuilt with cg-inject, or use a separate image)
- Set `--cgroupns=host` on the stress container
- Mount `/sys/fs/cgroup:/sys/fs/cgroup:rw`
- Entrypoint: `/cg-inject` with args: `--target-id <containerID> --cgroup-driver <cgroupfs|systemd> -- stress-ng <stressors...>`
- No `--cgroup-parent` needed in this mode

### cg-inject binary

A new `cmd/cg-inject/` directory in the pumba repo:

```go
// cmd/cg-inject/main.go
// Minimal binary that:
// 1. Parses --target-id, --cgroup-driver flags and -- stress-ng args
// 2. Constructs cgroup path: /sys/fs/cgroup/docker/<id>/cgroup.procs (cgroupfs)
//    or /sys/fs/cgroup/system.slice/docker-<id>.scope/cgroup.procs (systemd)
// 3. Writes own PID to cgroup.procs
// 4. Exec's stress-ng with remaining args
```

Build: `CGO_ENABLED=0 go build -ldflags='-s -w' -o cg-inject ./cmd/cg-inject/`

### Stress image

For `--inject-cgroup` mode, need an image with both `/cg-inject` and `/stress-ng`. Options:

- Add `cg-inject` to the existing `stress-ng` image build (preferred — keep one image)
- Or use a separate `stress-inject` image

Decision: Add a `Dockerfile.stress` in the repo that builds cg-inject and copies stress-ng, producing a scratch-based multi-arch image. The default stress image for inject mode should be `ghcr.io/alexei-led/pumba-stress:latest`.

For now in this PR: **include cg-inject source in the repo** but use the existing prototype approach — the stress image configuration is the user's responsibility via `--stress-image` flag. Document how to build a combined image.

## Tasks

### Task 1: Create cg-inject binary

Create `cmd/cg-inject/main.go`:

- [x] Parse flags: `--target-id` (required), `--cgroup-driver` (default: auto-detect from `/sys/fs/cgroup/cgroup.controllers` existence → v2)
- [x] For cgroupfs driver: cgroup path = `/sys/fs/cgroup/docker/<target-id>/cgroup.procs`
- [x] For systemd driver: cgroup path = `/sys/fs/cgroup/system.slice/docker-<target-id>.scope/cgroup.procs`
- [x] Also support cgroups v1: `/sys/fs/cgroup/cpu/docker/<target-id>/cgroup.procs` (check if `/sys/fs/cgroup/cgroup.controllers` exists to detect v2 vs v1)
- [x] Write `os.Getpid()` to `cgroup.procs` file
- [x] `syscall.Exec` the remaining args after `--` (first arg = stress-ng path)
- [x] Minimal error handling with clear error messages
- [x] No external dependencies (stdlib only)

Create `cmd/cg-inject/main_test.go`:

- [x] Test flag parsing
- [x] Test cgroup path construction for cgroupfs/systemd/v1
- [x] Test error cases (missing target-id, missing command after --)

### Task 2: Add --inject-cgroup flag to stress command

In `pkg/chaos/stress/cmd/stress.go`:

- [x] Add `--inject-cgroup` boolean flag (default: false), usage: `"Inject stress-ng into target container's cgroup (same cgroup, shared resource accounting). Requires stress image with cg-inject binary."`
- [x] Pass the flag value through to the stress command execution

In `pkg/chaos/stress/stress.go`:

- [x] Add `InjectCgroup bool` to the stress command struct/params
- [x] Pass it through to `StressContainerCommand`

### Task 3: Implement dual-mode stress container creation

In `pkg/container/docker_client.go`, modify `stressContainerCommand`:

When `injectCgroup=false` (default, current behavior):

- [x] Keep existing `--cgroup-parent` logic unchanged

When `injectCgroup=true`:

- [x] Entrypoint: `[]string{"/cg-inject"}`
- [x] Cmd: `[]string{"--target-id", targetID, "--cgroup-driver", driver, "--", "/stress-ng", stressors...}`
- [x] HostConfig: add `CgroupnsMode: "host"`, add bind mount `/sys/fs/cgroup:/sys/fs/cgroup:rw`
- [x] Do NOT set `CgroupParent`
- [x] Do NOT add any capabilities (verified: none needed)
- [x] Get cgroup driver from `systemAPI.Info()` (already available)

- [x] Update the `StressContainerCommand` interface/signature to accept `injectCgroup bool` parameter.

### Task 4: Unit tests for dual-mode stress

In `pkg/container/stress_test.go`:

- [x] Add test case "stress container with inject-cgroup mode (cgroupfs)":
  - Verify entrypoint is `/cg-inject`
  - Verify cmd contains `--target-id`, `--cgroup-driver cgroupfs`, `--`, `/stress-ng`, stressors
  - Verify CgroupnsMode is "host"
  - Verify `/sys/fs/cgroup` bind mount present
  - Verify NO CgroupParent set
  - Verify NO capabilities
- [x] Add test case "stress container with inject-cgroup mode (systemd)":
  - Same as above but with systemd driver
- [x] Add test case "stress container inject-cgroup with custom image"
- [x] Existing child-cgroup tests should remain unchanged

In `cmd/cg-inject/main_test.go`:

- [x] Test cgroup path construction
- [x] Test v1 vs v2 detection logic
- [x] Test flag parsing edge cases

### Task 5: Integration test for stress with inject-cgroup

In integration tests (bats):

- [x] Add test that runs `pumba stress --inject-cgroup --duration 5s --stressors="--cpu 1" <target>`
- [x] Verify stress-ng runs and target container survives
- [x] This requires a stress image with cg-inject — skip if image not available

### Task 6: Update documentation

In `docs/stress-testing.md`:

- [x] Add new section "## Same-Cgroup Injection Mode" explaining:
  - What it does (places stress-ng in exact same cgroup as target)
  - When to use it (realistic chaos: shared OOM, accurate resource contention)
  - The `--inject-cgroup` flag
  - Security: no privileged mode, no capabilities needed
  - Requirements: stress image must contain `/cg-inject` and `/stress-ng`
  - How to build a combined image (Dockerfile example)
- [x] Add comparison table: child cgroup vs same cgroup (behavior, safety, use case)
- [x] Document cgroups v1 vs v2 compatibility

In `docs/guide.md`:

- [x] Mention `--inject-cgroup` briefly in the stress section

- [x] Update `README.md` if stress features are mentioned.

### Task 7: Makefile / build updates

- [x] Add `cg-inject` build target to Makefile: `build-cg-inject: CGO_ENABLED=0 go build -ldflags='-s -w' -o bin/cg-inject ./cmd/cg-inject/`
- [x] Add `Dockerfile.stress` that builds a combined image (cg-inject + stress-ng from upstream)
- [x] Ensure `go build ./...` includes `cmd/cg-inject`

## Non-goals (for this PR)

- CI pipeline for building/pushing the combined stress image (follow-up)
- Automatic image selection based on `--inject-cgroup` flag (user passes `--stress-image`)
- Containerd support
