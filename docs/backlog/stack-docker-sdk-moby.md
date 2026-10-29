# Docker SDK to moby/moby client modules

Tags: stack · Effort: M

## Why
Currently `github.com/docker/docker v28.5.2+incompatible`; upstream is moving to the `moby/moby` client and api modules.

## Approach
Migrate imports and moved types in `pkg/runtime/docker` (and podman, which embeds it). Verify against current upstream release notes first.

## Open questions
- Podman compat-API behavior with the new client (see exec gotcha in AGENTS.md).
