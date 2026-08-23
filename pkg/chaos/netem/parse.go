package netem

import (
	"errors"
	"fmt"
	"net"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/cliflags"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/alexei-led/pumba/pkg/util"
)

// ParseRequestBase reads the netem-level flags (--duration, --interface,
// --target, --egress-port, --ingress-port, --tc-image, --pull-image, --limit)
// from c and returns a *container.NetemRequest with the shared base fields
// filled, plus the --limit value (consumed by per-action ListNContainers calls
// rather than by the runtime). Container and Command are left zero — each
// per-action Run sets them per iteration.
//
// Each --target value is parsed as an IP or CIDR literal first; values that
// aren't (e.g. a container name or ID) are kept as-is in TargetNames rather
// than rejected here, since resolving them requires a runtime client that
// isn't available during CLI flag parsing. Each command Run resolves
// TargetNames into IPv4 filters before enumerating affected containers.
//
// c must be the netem parent context. Per-action parsers pass c.Parent().
func ParseRequestBase(c cliflags.Flags, gp *chaos.GlobalParams) (*container.NetemRequest, int, error) {
	duration := c.Duration("duration")
	if duration == 0 {
		return nil, 0, errors.New("unset or invalid duration value")
	}
	if gp.Interval != 0 && duration >= gp.Interval {
		return nil, 0, errors.New("duration must be shorter than interval")
	}
	iface := c.String("interface")
	if err := util.ValidateInterfaceName(iface); err != nil {
		return nil, 0, err
	}
	targetList := c.StringSlice("target")
	ips := make([]*net.IPNet, 0, len(targetList))
	seenIPs := make(map[string]struct{}, len(targetList))
	var targetNames []string
	for _, s := range targetList {
		ip, err := util.ParseCIDR(s)
		if err != nil {
			// Not a literal IP/CIDR — treat as a container name or ID to
			// resolve later, once a runtime client is available.
			targetNames = append(targetNames, s)
			continue
		}
		if ip.IP.To4() == nil {
			return nil, 0, fmt.Errorf("IPv6 --target %q is not supported", s)
		}
		if _, ok := seenIPs[ip.String()]; ok {
			continue
		}
		seenIPs[ip.String()] = struct{}{}
		ips = append(ips, ip)
	}
	sports, err := util.GetPorts(c.String("egress-port"))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get source ports: %w", err)
	}
	dports, err := util.GetPorts(c.String("ingress-port"))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get destination ports: %w", err)
	}
	return &container.NetemRequest{
		Interface:   iface,
		IPs:         ips,
		TargetNames: targetNames,
		SPorts:      sports,
		DPorts:      dports,
		Duration:    duration,
		Sidecar:     container.SidecarSpec{Image: c.String("tc-image"), Pull: c.Bool("pull-image")},
		DryRun:      gp.DryRun,
	}, c.Int("limit"), nil
}
