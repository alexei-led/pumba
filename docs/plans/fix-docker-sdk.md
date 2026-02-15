# Fix Docker SDK Upgrade Conflicts

## Context

- We are merging PR #276 (Docker SDK v28.5.2) into master (Go 1.26 + stdlib errors).
- Conflicts in `pkg/container/docker_client.go`.
- HEAD uses `fmt.Errorf` and `%w`.
- Incoming uses `errors.Wrap` and `ctypes.*`, `imagetypes.*`.

## Task

1.  [x] Resolve conflicts in `pkg/container/docker_client.go`.
    - [x] Keep HEAD's `fmt.Errorf` style.
    - [x] Adopt Incoming's type changes (`ctypes.RemoveOptions`, etc.).
    - [x] Update imports:
      - [x] `github.com/docker/docker/api/types/container` as `ctypes`
      - [x] `github.com/docker/docker/api/types/image` as `imagetypes`
      - [x] `github.com/docker/docker/api/types/network` as `networktypes`
      - [x] Remove `github.com/pkg/errors`.
2.  [x] After resolving, verify imports and compilation.
