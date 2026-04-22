package podman

import (
	"fmt"
	"strings"

	"github.com/docker/docker/api/types/system"
)

// rootlessMarker is the substring Podman publishes in /info SecurityOptions
// when the API is serving a rootless connection.
const rootlessMarker = "name=rootless"

// detectRootless reports whether the Podman API is serving a rootless socket
// by scanning /info's SecurityOptions for the `name=rootless` marker.
func detectRootless(info *system.Info) bool {
	if info == nil {
		return false
	}
	for _, opt := range info.SecurityOptions {
		if strings.Contains(opt, rootlessMarker) {
			return true
		}
	}
	return false
}

// rootlessError returns the user-facing error emitted when a chaos command
// that requires kernel privileges (netem, iptables, stress) is invoked
// against a rootless Podman socket. Message includes both macOS and Linux
// remediation hints.
func rootlessError(cmd, socketURI string) error {
	return fmt.Errorf(
		"podman runtime: %s requires rootful podman (detected rootless socket at %s). "+
			"On macOS: podman machine stop && podman machine set --rootful && podman machine start. "+
			"On Linux: run as root with /run/podman/podman.sock or configure rootful service",
		cmd, socketURI,
	)
}

// Keep these reachable from the package at build time so the unused linter
// can trace the dependency graph before client.go (a later task) wires them
// into NewClient. Removed once NewClient calls detectRootless/rootlessError
// directly.
var (
	_ = detectRootless
	_ = rootlessError
	_ = rootlessMarker
)
