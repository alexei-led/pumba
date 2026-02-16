# Plan: Fix stress command for cgroups v2

## Validation Commands

- `go build ./...`
- `go vet ./...`
- `golangci-lint run ./...`
- `CGO_ENABLED=1 go test -count=1 ./pkg/container/ ./pkg/chaos/stress/`

## Problem

`pumba stress` fails on cgroups v2 with: `Cgroup is not mounted / cgroup controller and pathparsing failed`.
Root cause: uses `dockhack cg_exec` which needs `lscgroup`/`cgexec` (cgroups v1 only).
Tracked in: #223

## Solution

Replace `dockhack`+`cgexec` with Docker's native `--cgroup-parent` flag. The stress-ng container becomes a child cgroup of the target, inheriting all resource limits. Works on both cgroups v1 and v2.

### Task 1: Add systemAPI to dockerClient and implement cgroupParent helper

- [x] Add `systemAPI dockerapi.SystemAPIClient` field to `dockerClient` struct in `pkg/container/docker_client.go`
- [x] Update `NewClient()` to wire `systemAPI: apiClient`
- [x] Add `cgroupParent(ctx context.Context, targetID string) (string, error)` method that calls `client.systemAPI.Info(ctx)` to get `CgroupDriver`, then returns `/docker/<targetID>` for cgroupfs or `system.slice:docker:<targetID>` for systemd
- [x] Import `dockerapi "github.com/docker/docker/client"` is already there, just need to use `dockerapi.SystemAPIClient`

### Task 2: Rewrite stressContainerCommand to use cgroup-parent

- [x] In `stressContainerCommand()` in `pkg/container/docker_client.go`, call `client.cgroupParent(ctx, targetID)` at the start to get the cgroup parent path
- [x] Change `Config.Entrypoint` from `[]string{"dockhack", "cg_exec"}` to `[]string{"stress-ng"}`
- [x] Change `Config.Cmd` from `dockhackArgs` (which prepends targetID) to just `stressors` directly
- [x] Add `CgroupParent: cgroupParent` to `HostConfig`
- [x] Remove the `dockerSocket` mount (bind mount of `/var/run/docker.sock`)
- [x] Remove the `fsCgroup` mount (bind mount of `/sys/fs/cgroup`)
- [x] Remove `CapAdd: []string{"SYS_ADMIN"}` from HostConfig
- [x] Remove `SecurityOpt: []string{"apparmor:unconfined"}` from HostConfig
- [x] Remove the `Mounts` field from HostConfig entirely (no mounts needed)
- [x] Keep `AutoRemove: true` and `Labels` in HostConfig

### Task 3: Update default stress-ng image and CLI description

- [x] In `pkg/chaos/stress/cmd/stress.go`, change the default `Value` for `stress-image` flag from `"alexeiled/stress-ng:latest-ubuntu"` to `"ghcr.io/alexei-led/stress-ng:latest"`
- [x] Update the `Usage` text for the `stress-image` flag from `"Docker image with stress-ng tool, cgroup-bin and docker packages, and dockhack script"` to `"Docker image with stress-ng tool"`

### Task 4: Update tests for new stress container configuration

- [x] In `pkg/container/stress_test.go`, update all `dockerClient` construction to include `systemAPI: api` field
- [x] Add mock expectation `api.On("Info", mock.Anything).Return(system.Info{CgroupDriver: "cgroupfs"}, nil)` to non-dryrun test cases (import `"github.com/docker/docker/api/types/system"`)
- [x] Update the "stress container creation failure" test: the mock for `ContainerCreate` should still work since signature is unchanged
- [x] For the "stress container image pull failure" test: add Info mock since cgroupParent is called before image pull
- [x] Add a new test case "stress container with systemd cgroup driver" that sets `CgroupDriver: "systemd"` and verifies the container is created with correct cgroup parent
- [x] Verify the container create mock expectations do NOT include docker socket mount, SYS_ADMIN cap, or apparmor security opt

### Task 5: Update documentation

- [x] In `docs/stress-testing.md`: remove the entire "stress-ng Image Requirements" section about dockhack, bash, docker CLI, cgexec, cgroup-tools, and the custom Dockerfile example
- [x] Replace with a simpler section: the stress image only needs a `stress-ng` binary (the default `ghcr.io/alexei-led/stress-ng:latest` is a minimal scratch image)
- [x] Update the default image name throughout the file from `alexeiled/stress-ng:latest-ubuntu` to `ghcr.io/alexei-led/stress-ng:latest`
- [x] In `docs/deployment.md`: in the "Stress Testing on Kubernetes" section, remove the `SYS_ADMIN` capability from the security context example
- [x] In `docs/deployment.md`: update the stress-ng image comment if present
