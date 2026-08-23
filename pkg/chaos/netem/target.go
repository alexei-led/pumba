package netem

import (
	"context"
	"fmt"
	"net"
	"slices"
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
// Each name/ID is matched against currently running containers by exact
// container name or ID first, then by a unique container-ID prefix of at least
// minIDPrefixLen characters. Docker/Podman's leading name slash is ignored.
//
// A target that matches more than one container at the same priority level
// is a hard error rather than an arbitrary pick: silently applying a network
// filter to the wrong container's IP is worse than refusing to run. A target
// that matches no container, or a container with no IP address on any
// attached network (e.g. host networking, or a runtime that doesn't report
// one), is also a hard error — never a silent no-op.
//
// A resolved container contributes every IPv4 address reported by its
// runtime. IPv6 targets fail explicitly because the current tc filter planner
// only emits protocol-ip rules.
type targetResolver interface {
	container.Lister
	container.AddressResolver
}

func resolveRequestTargets(ctx context.Context, resolver targetResolver, source *container.NetemRequest) (*container.NetemRequest, error) {
	req := *source
	req.IPs = slices.Clone(source.IPs)
	req.TargetNames = slices.Clone(source.TargetNames)
	if err := resolveTargetNames(ctx, resolver, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

func resolveTargetNames(ctx context.Context, resolver targetResolver, req *container.NetemRequest) error {
	if len(req.TargetNames) == 0 {
		return nil
	}
	candidates, err := resolver.ListContainers(ctx, func(*container.Container) bool { return true }, container.ListOpts{All: false})
	if err != nil {
		return fmt.Errorf("failed to list containers while resolving --target: %w", err)
	}
	seen := make(map[string]struct{}, len(req.IPs))
	for _, ip := range req.IPs {
		seen[ip.String()] = struct{}{}
	}
	resolved := make([]*net.IPNet, 0, len(req.TargetNames))
	addressCache := make(map[string][]net.IP)
	for _, name := range req.TargetNames {
		c, err := matchTargetContainer(name, candidates)
		if err != nil {
			return err
		}
		addresses, ok := addressCache[c.ID()]
		if !ok {
			addresses = c.IPs()
			if len(addresses) == 0 {
				addresses, err = resolver.ContainerAddresses(ctx, c)
				if err != nil {
					return fmt.Errorf("failed to resolve addresses for --target %q container %q (%s): %w", name, c.Name(), c.ID(), err)
				}
			}
			addressCache[c.ID()] = addresses
		}
		if len(addresses) == 0 {
			return fmt.Errorf("--target %q resolved to container %q (%s) but it has no IP address on any attached network",
				name, c.Name(), c.ID())
		}
		for _, address := range addresses {
			ipv4 := address.To4()
			if ipv4 == nil {
				return fmt.Errorf("--target %q resolved to unsupported IPv6 address %s; IPv6 netem filters are not supported", name, address)
			}
			cidr := hostCIDR(ipv4)
			if _, ok := seen[cidr.String()]; ok {
				continue
			}
			seen[cidr.String()] = struct{}{}
			resolved = append(resolved, cidr)
		}
	}
	req.IPs = append(req.IPs, resolved...)
	req.TargetNames = nil
	return nil
}

// hostCIDR wraps a resolved IPv4 address as a host route, matching how
// util.ParseCIDR treats a bare IP passed directly to --target.
func hostCIDR(ip net.IP) *net.IPNet {
	return &net.IPNet{IP: ip.To4(), Mask: net.CIDRMask(32, 32)} //nolint:mnd
}

// matchTargetContainer resolves a single --target value against the
// candidate container list. See resolveTargetNames for the matching order
// and ambiguity rules.
func matchTargetContainer(target string, candidates []*container.Container) (*container.Container, error) {
	trimmedTarget := strings.TrimPrefix(target, "/")
	if trimmedTarget == "" {
		return nil, fmt.Errorf("--target %q is not a valid IP/CIDR or container name/ID", target)
	}

	exact := make(map[*container.Container]struct{})
	var byIDPrefix []*container.Container
	for _, c := range candidates {
		if strings.TrimPrefix(c.Name(), "/") == trimmedTarget || c.ID() == target {
			exact[c] = struct{}{}
			continue
		}
		if len(target) >= minIDPrefixLen && strings.HasPrefix(c.ID(), target) {
			byIDPrefix = append(byIDPrefix, c)
		}
	}

	switch {
	case len(exact) == 1:
		var match *container.Container
		for c := range exact {
			match = c
		}
		return match, nil
	case len(exact) > 1:
		return nil, fmt.Errorf("--target %q is ambiguous: matches %d exact container names or IDs", target, len(exact))
	case len(byIDPrefix) == 1:
		return byIDPrefix[0], nil
	case len(byIDPrefix) > 1:
		return nil, fmt.Errorf("--target %q is ambiguous: matches %d container IDs", target, len(byIDPrefix))
	default:
		return nil, fmt.Errorf("--target %q is not a valid IP/CIDR and does not match any running container name or ID", target)
	}
}
