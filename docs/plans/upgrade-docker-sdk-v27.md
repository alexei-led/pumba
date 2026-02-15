# Plan: Upgrade Docker SDK to v27.5.1

Upgrade github.com/docker/docker from v23.0.3 to v27.5.1. This fixes the Docker API version mismatch (client 1.42 vs minimum 1.44) that causes integration test failures on modern Docker engines.

## Context

- CLAUDE.md in project root has full project context — READ IT FIRST
- Default branch: master
- Current: github.com/docker/docker v23.0.3+incompatible (API version 1.42)
- Target: github.com/docker/docker v27.5.1+incompatible (API version 1.46)
- The Docker SDK v25+ restructured types into sub-packages
- Mock files in mocks/ and pkg/container/mock_*.go need regeneration after interface changes
- Do NOT add AI co-author to commits
- GPG signing unavailable — use: git -c commit.gpgsign=false commit

## Key Type Migration Reference

Types moved between v23 and v27:
- types.ContainerListOptions → container.ListOptions
- types.ContainerStartOptions → container.StartOptions
- types.ContainerRemoveOptions → container.RemoveOptions
- types.ContainerAttachOptions → container.AttachOptions
- types.ExecConfig → container.ExecOptions
- types.ExecStartCheck → container.ExecStartOptions
- types.ContainerExecInspect → container.ExecInspect (also aliased in types)
- types.ImagePullOptions → image.PullOptions
- types.ContainerCreateCreatedBody → container.CreateResponse
- types.IDResponse — still in types package
- types.HijackedResponse — still in types package
- types.ContainerJSON — still in types package
- types.Container — still in types package

Import aliases already used in codebase:
- ctypes = github.com/docker/docker/api/types/container
- Add: imagetypes = github.com/docker/docker/api/types/image

## Validation Commands
- `TARGETOS=darwin TARGETARCH=amd64 make build`
- `CGO_ENABLED=1 TARGETOS=darwin TARGETARCH=amd64 go test -timeout 30s ./...`
- `golangci-lint run -v -c .golangci.yaml ./...`

### Task 1: Upgrade Docker SDK dependency
- [ ] Run: go get github.com/docker/docker@v27.5.1
- [ ] Run: go mod tidy
- [ ] Check go.mod shows v27.5.1

### Task 2: Update docker_client.go types
- [ ] Replace types.ContainerListOptions with ctypes.ListOptions
- [ ] Replace types.ContainerStartOptions with ctypes.StartOptions
- [ ] Replace types.ContainerRemoveOptions with ctypes.RemoveOptions
- [ ] Replace types.ContainerAttachOptions with ctypes.AttachOptions
- [ ] Replace types.ExecConfig with ctypes.ExecOptions
- [ ] Replace types.ExecStartCheck with ctypes.ExecStartOptions
- [ ] Replace types.ImagePullOptions with imagetypes.PullOptions (add import)
- [ ] Update import block: add imagetypes, remove types if no longer needed (keep if IDResponse/HijackedResponse/ContainerJSON still used)
- [ ] Run goimports to fix imports
- [ ] Verify: go build ./cmd/main.go passes

### Task 3: Update test files
- [ ] Update pkg/container/client_test.go with same type migrations
- [ ] Update pkg/container/stress_test.go if needed
- [ ] Update pkg/container/mockengine_responses.go if needed
- [ ] Replace types.ContainerCreateCreatedBody with ctypes.CreateResponse in test files
- [ ] Add imagetypes import where needed
- [ ] Run goimports on all changed test files
- [ ] Verify: go test ./pkg/container/... passes (may fail on mock compilation — that is OK, Task 4 fixes mocks)

### Task 4: Regenerate mock files
- [ ] Install mockery: go install github.com/vektra/mockery/v2@latest
- [ ] Find Docker SDK source dir: go mod download -json github.com/docker/docker@v27.5.1+incompatible
- [ ] Regenerate mocks/ContainerAPIClient.go from Docker SDK ContainerAPIClient interface
- [ ] Regenerate mocks/ImageAPIClient.go from Docker SDK ImageAPIClient interface
- [ ] Regenerate mocks/APIClient.go from Docker SDK APIClient interface
- [ ] Regenerate pkg/container/mock_Client.go: mockery --dir pkg/container --inpackage --name Client
- [ ] Regenerate pkg/container/mock_FilterFunc.go: mockery --dir pkg/container --inpackage --name FilterFunc
- [ ] Run goimports on all regenerated mock files
- [ ] Verify: go build ./... passes
- [ ] Verify: go test ./... passes
- [ ] Also update mocks/ChaosCommand.go if needed: mockery --dir pkg/chaos/docker --all

### Task 5: Fix any remaining compilation or test failures
- [ ] Run full test suite and fix any test failures caused by API changes
- [ ] Check if any Docker API method signatures changed (e.g. new parameters, changed return types)
- [ ] Run golangci-lint and fix any new issues
- [ ] Verify everything is clean: build, test, lint all pass
