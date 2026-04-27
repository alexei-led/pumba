package netem

import (
	"errors"
	"fmt"
	"net"
	"regexp"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/cliflags"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/alexei-led/pumba/pkg/util"
)

// reInterface validates --interface against shell-injection. Compiled once.
var reInterface = regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9.:_-]*`)

// ParseRequestBase reads the netem-level flags (--duration, --interface,
// --target, --egress-port, --ingress-port, --tc-image, --pull-image, --limit)
// from c and returns a *container.NetemRequest with the shared base fields
// filled, plus the --limit value (consumed by per-action ListNContainers calls
// rather than by the runtime). Container and Command are left zero — each
// per-action Run sets them per iteration.
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
	if iface != reInterface.FindString(iface) {
		return nil, 0, fmt.Errorf("bad network interface name: must match '%s'", reInterface.String())
	}
	ipsList := c.StringSlice("target")
	ips := make([]*net.IPNet, 0, len(ipsList))
	for _, s := range ipsList {
		ip, err := util.ParseCIDR(s)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to parse ip: %w", err)
		}
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
		Interface: iface,
		IPs:       ips,
		SPorts:    sports,
		DPorts:    dports,
		Duration:  duration,
		Sidecar:   container.SidecarSpec{Image: c.String("tc-image"), Pull: c.Bool("pull-image")},
		DryRun:    gp.DryRun,
	}, c.Int("limit"), nil
}
