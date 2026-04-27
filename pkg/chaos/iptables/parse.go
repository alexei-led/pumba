package iptables

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"slices"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/cliflags"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/alexei-led/pumba/pkg/util"
)

// reInterface validates --interface against shell-injection. Compiled once.
var reInterface = regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9.:_-]*`)

// RequestBase bundles the parsed iptables-level state shared by every
// per-action subcommand. Request carries the runtime fields (IPs, ports,
// duration, sidecar hint, dry-run); Iface and Protocol are kept separate so
// per-action parsers can assemble the iptables command prefix
// (`-I/-D INPUT -i <iface> [-p <proto>] …`); Limit is the --limit value
// consumed by the per-action ListNContainers call rather than by the runtime.
type RequestBase struct {
	Request  *container.IPTablesRequest
	Iface    string
	Protocol string
	Limit    int
}

// ParseRequestBase reads the iptables-level flags (--duration, --interface,
// --protocol, --source, --destination, --src-port, --dst-port,
// --iptables-image, --pull-image, --limit) from c and returns a RequestBase
// with the shared fields filled. Container, CmdPrefix and CmdSuffix on
// Request are left zero — each per-action Run sets them per iteration.
//
// c must be the iptables parent context. Per-action parsers pass c.Parent().
func ParseRequestBase(c cliflags.Flags, gp *chaos.GlobalParams) (*RequestBase, error) {
	duration := c.Duration("duration")
	if duration == 0 {
		return nil, errors.New("unset or invalid duration value")
	}
	if gp.Interval != 0 && duration >= gp.Interval {
		return nil, errors.New("duration must be shorter than interval")
	}
	iface := c.String("interface")
	if iface != reInterface.FindString(iface) {
		return nil, fmt.Errorf("bad network interface name: must match '%s'", reInterface.String())
	}
	protocol := c.String("protocol")
	if !slices.Contains([]string{ProtocolAny, ProtocolTCP, ProtocolUDP, ProtocolICMP}, protocol) {
		return nil, errors.New("bad protocol name: must be one of any, tcp, udp or icmp")
	}
	srcIPs, err := validateCIDRList(c.StringSlice("source"))
	if err != nil {
		return nil, err
	}
	dstIPs, err := validateCIDRList(c.StringSlice("destination"))
	if err != nil {
		return nil, err
	}
	sports, err := util.GetPorts(c.String("src-port"))
	if err != nil {
		return nil, fmt.Errorf("failed to get source ports: %w", err)
	}
	dports, err := util.GetPorts(c.String("dst-port"))
	if err != nil {
		return nil, fmt.Errorf("failed to get destination ports: %w", err)
	}
	if protocol != ProtocolUDP && protocol != ProtocolTCP {
		if len(sports) > 0 {
			return nil, fmt.Errorf("using source port is only supported for %s and %s protocol", ProtocolTCP, ProtocolUDP)
		}
		if len(dports) > 0 {
			return nil, fmt.Errorf("using destination port is only supported for %s and %s protocol", ProtocolTCP, ProtocolUDP)
		}
	}
	return &RequestBase{
		Request: &container.IPTablesRequest{
			SrcIPs:   srcIPs,
			DstIPs:   dstIPs,
			SPorts:   sports,
			DPorts:   dports,
			Duration: duration,
			Sidecar:  container.SidecarSpec{Image: c.String("iptables-image"), Pull: c.Bool("pull-image")},
			DryRun:   gp.DryRun,
		},
		Iface:    iface,
		Protocol: protocol,
		Limit:    c.Int("limit"),
	}, nil
}

func validateCIDRList(list []string) ([]*net.IPNet, error) {
	ips := make([]*net.IPNet, 0, len(list))
	for _, s := range list {
		ip, err := util.ParseCIDR(s)
		if err != nil {
			return nil, fmt.Errorf("failed to parse ip: %w", err)
		}
		ips = append(ips, ip)
	}
	return ips, nil
}
