package cmd

import (
	"net"
	"regexp"
	"slices"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos/iptables"
	"github.com/alexei-led/pumba/pkg/util"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

func validateIPList(list []string) (ips []*net.IPNet, err error) {
	ips = make([]*net.IPNet, 0, len(list))
	for _, s := range list {
		ip, e := util.ParseCIDR(s)
		if e != nil {
			return ips, errors.Wrap(e, "failed to parse ip")
		}
		ips = append(ips, ip)
	}
	return
}

func parseIPTablesParams(c *cli.Context, interval time.Duration) (*iptables.Params, error) {
	// get duration
	duration := c.Duration("duration")
	if duration == 0 {
		return nil, errors.New("unset or invalid duration value")
	}
	if interval != 0 && duration >= interval {
		return nil, errors.New("duration must be shorter than interval")
	}

	// protect from Command Injection, using Regexp
	iface := c.String("interface")
	reInterface := regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9.:_-]*`)
	validIface := reInterface.FindString(iface)
	if iface != validIface {
		return nil, errors.Errorf("bad network interface name: must match '%s'", reInterface.String())
	}

	// check for valid protocol
	protocol := c.String("protocol")
	if !slices.Contains([]string{iptables.ProtocolAny, iptables.ProtocolTCP, iptables.ProtocolUDP, iptables.ProtocolICMP}, protocol) {
		return nil, errors.Errorf("bad protocol name: must be one of any, tcp, udp or icmp")
	}

	// validate src ips
	srcIPs, err := validateIPList(c.StringSlice("source"))
	if err != nil {
		return nil, err
	}
	// validate dst ips
	dstIPs, err := validateIPList(c.StringSlice("destination"))
	if err != nil {
		return nil, err
	}

	// validate source ports
	sports, err := util.GetPorts(c.String("src-port"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get source ports")
	}
	// validate destination ports
	dports, err := util.GetPorts(c.String("dst-port"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get destination ports")
	}
	if protocol != iptables.ProtocolUDP && protocol != iptables.ProtocolTCP {
		if len(sports) > 0 {
			return nil, errors.Errorf("using source port is only supported for %s and %s protocol", iptables.ProtocolTCP, iptables.ProtocolUDP)
		}
		if len(dports) > 0 {
			return nil, errors.Errorf("using destination port is only supported for %s and %s protocol", iptables.ProtocolTCP, iptables.ProtocolUDP)
		}
	}

	return &iptables.Params{
		Iface:    iface,
		Protocol: protocol,
		SrcIPs:   srcIPs,
		DstIPs:   dstIPs,
		Sports:   sports,
		Dports:   dports,
		Duration: duration,
		Image:    c.String("iptables-image"),
		Pull:     c.Bool("pull-image"),
		Limit:    c.Int("limit"),
	}, nil
}
