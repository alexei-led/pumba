package netem

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/alexei-led/pumba/pkg/container"
)

// minIDPrefixLen is the shortest container-ID prefix resolveTargetNames will
// try to match. Docker's own short IDs are 12 characters; anything much
// shorter has a real chance of prefix-colliding with an unrelated container
// in a large fleet, so target names shorter than this are only tried
// against exact container names, never against ID prefixes.
const minIDPrefixLen = 6

// resolveTargetNames resolves req.TargetNames — --target values that were
// not valid IP/CIDR literals at parse time (see ParseRequestBase) — into IP
// filters, appending the result to req.IPs and clearing TargetNames. It is a
// no-op — no runtime call at all — when TargetNames is empty, so plain
// IP/CIDR --target usage, the overwhelming common case, costs nothing extra.
//
// Each name/ID is matched against currently running containers, in order:
//  1. exact container name (Docker/Podman report names with a leading "/",
//     which is trimmed before comparing);
//  2. exact container ID;
//  3. a unique container-ID prefix of at least minIDPrefixLen characters.
//
// A target that matches more than one container at the same priority level
// is a hard error rather than an arbitrary pick: silently applying a network
// filter to the wrong container's IP is worse than refusing to run. A target
// that matches no container, or a container with no IP address on any
// attached network (e.g. host networking, or a runtime that doesn't report
// one), is also a hard error — never a silent no-op.
//
// A resolved container contributes every IPv4 address found across all of
// its attached networks: a container connected to more than one network is
// reachable on each of those addresses, and tc needs a filter per address to
// actually scope traffic to all of them rather than just one arbitrarily
// chosen network.
func resolveTargetNames(ctx context.Context, lister container.Lister, req *container.NetemRequest) error {
	if len(req.TargetNames) == 0 {
		return nil
	}
	candidates, err := lister.ListContainers(ctx, func(*container.Container) bool { return true }, container.ListOpts{All: false})
	if err != nil {
		return fmt.Errorf("failed to list containers while resolving --target: %w", err)
	}
	for _, name := range req.TargetNames {
		c, err := matchTargetContainer(name, candidates)
		if err != nil {
			return err
		}
		ips := c.IPs()
		if len(ips) == 0 {
			return fmt.Errorf("--target %q resolved to container %q (%s) but it has no IP address on any attached network",
				name, c.Name(), c.ID())
		}
		for _, ip := range ips {
			req.IPs = append(req.IPs, hostCIDR(ip))
		}
	}
	req.TargetNames = nil
	return nil
}

// hostCIDR wraps a single resolved address as a host route (/32 for IPv4,
// /128 for IPv6), matching how util.ParseCIDR treats a bare IP given
// directly on --target.
func hostCIDR(ip net.IP) *net.IPNet {
	if v4 := ip.To4(); v4 != nil {
		return &net.IPNet{IP: v4, Mask: net.CIDRMask(32, 32)} //nolint:mnd
	}
	return &net.IPNet{IP: ip, Mask: net.CIDRMask(128, 128)} //nolint:mnd
}

// matchTargetContainer resolves a single --target value against the
// candidate container list. See resolveTargetNames for the matching order
// and ambiguity rules.
func matchTargetContainer(target string, candidates []*container.Container) (*container.Container, error) {
	trimmedTarget := strings.TrimPrefix(target, "/")
	if trimmedTarget == "" {
		return nil, fmt.Errorf("--target %q is not a valid IP/CIDR or container name/ID", target)
	}

	var byName, byID, byIDPrefix []*container.Container
	for _, c := range candidates {
		if strings.TrimPrefix(c.Name(), "/") == trimmedTarget {
			byName = append(byName, c)
		}
		switch {
		case c.ID() == target:
			byID = append(byID, c)
		case len(target) >= minIDPrefixLen && strings.HasPrefix(c.ID(), target):
			byIDPrefix = append(byIDPrefix, c)
		}
	}

	switch {
	case len(byName) == 1:
		return byName[0], nil
	case len(byName) > 1:
		return nil, fmt.Errorf("--target %q is ambiguous: matches %d container names", target, len(byName))
	case len(byID) == 1:
		return byID[0], nil
	case len(byIDPrefix) == 1:
		return byIDPrefix[0], nil
	case len(byIDPrefix) > 1:
		return nil, fmt.Errorf("--target %q is ambiguous: matches %d container IDs", target, len(byIDPrefix))
	default:
		return nil, fmt.Errorf("--target %q is not a valid IP/CIDR and does not match any running container name or ID", target)
	}
}
