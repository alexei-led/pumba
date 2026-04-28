// Package podman implements ctr.Client for the Podman runtime.
//
// # Docker SDK as the working vocabulary
//
// Podman exposes a Docker-compatible API socket (libpod's "compat" endpoints).
// pumba programs against that socket using the upstream Docker SDK rather than
// libpod's native bindings. As a result, this package depends on
// github.com/docker/docker/api/types and its sub-packages
// (container, image, network, system) by design — those types are Podman's
// intentional working vocabulary, not a leaky abstraction.
//
// The choice is deliberate: a thin Podman-shaped anti-corruption layer on top
// of the Docker SDK would add ~50 LOC of indirection per type for negligible
// decoupling. The Podman project commits to Docker-compat as a first-class
// surface, so embedding the Docker delegate (see client.go) and overriding
// only the divergent paths is both smaller and easier to audit.
//
// # Divergent paths
//
// Three concerns require Podman-specific code; everything else inherits from
// the embedded Docker delegate unchanged:
//
//   - Rootless guards (rootless.go) — rootless Podman cannot grant NET_ADMIN
//     to a sidecar in the target's netns or write across sibling cgroup
//     scopes, so netem/iptables/stress fail fast at the API boundary with a
//     diagnostic pointing at `podman machine set --rootful` or the rootful
//     systemd unit.
//   - cgroup leaf naming (cgroup.go, stress.go) — Podman uses
//     `libpod-<id>.scope` (sometimes with a nested `container/` leaf for the
//     libpod init sub-cgroup) under systemd, vs Docker's `docker-<id>.scope`.
//     The resolver reads /proc/<pid>/cgroup host-side to pick the correct
//     leaf, which means pumba must run on the same kernel as the targets.
//   - Sidecar config (stress.go) — `StopSignal: "SIGKILL"` so force-remove is
//     immediate (Podman's compat DELETE waits the full StopTimeout before
//     escalating SIGTERM); `SecurityOpt: ["label=disable"]` for inject-cgroup
//     mode so SELinux container_t doesn't block cgroup writes.
//
// # Embedding invariant
//
// The override set is documented next to podmanClient in client.go. When
// adding a method to ctr.Client, audit Podman behavior — either confirm the
// Docker delegate's implementation works unchanged on the Docker-compat
// socket, or override defensively here. Silently inheriting a Docker method
// that Podman implements differently produces the worst kind of bug: works
// against Docker in CI, fails against Podman in the field, with no signal
// from the type system.
//
// # When this rationale stops holding
//
// If Podman ever drops Docker-compat (or the compat surface diverges far
// enough that the override set grows unmanageable), the trigger to migrate
// to libpod's native bindings is: more methods overridden than inherited, or
// override logic that no longer fits in a single file per concern. At that
// point a proper anti-corruption layer pays for itself.
package podman
