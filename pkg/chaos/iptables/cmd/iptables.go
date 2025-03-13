package cmd

import (
	"net"
	"regexp"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos/iptables"
	"github.com/alexei-led/pumba/pkg/util"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

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
	if protocol != "any" && protocol != "tcp" && protocol != "udp" && protocol != "icmp" {
		return nil, errors.Errorf("bad protocol name: must be one of any, tcp, udp or icmp")
	}
	// validate src ips
	srcIPsList := c.StringSlice("source")
	srcIPs := make([]*net.IPNet, 0, len(srcIPsList))
	for _, str := range srcIPsList {
		ip, e := util.ParseCIDR(str)
		if e != nil {
			return nil, errors.Wrap(e, "failed to parse ip")
		}
		srcIPs = append(srcIPs, ip)
	}
	// validate dst ips
	dstIPsList := c.StringSlice("destination")
	dstIPs := make([]*net.IPNet, 0, len(dstIPsList))
	for _, str := range dstIPsList {
		ip, e := util.ParseCIDR(str)
		if e != nil {
			return nil, errors.Wrap(e, "failed to parse ip")
		}
		dstIPs = append(dstIPs, ip)
	}
	// validate source ports
	sports, err := util.GetPorts(c.String("src-port"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get source ports")
	}
	if len(sports) > 0 {
		if protocol != "udp" && protocol != "tcp" {
			return nil, errors.Errorf("using source port is only supported for tcp and udp protocol")
		}
	}
	// validate destination ports
	dports, err := util.GetPorts(c.String("dst-port"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get destination ports")
	}
	if len(dports) > 0 {
		if protocol != "udp" && protocol != "tcp" {
			return nil, errors.Errorf("using destination port is only supported for tcp and udp protocol")
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
