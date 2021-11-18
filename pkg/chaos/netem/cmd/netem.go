package cmd

import (
	"net"
	"regexp"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos/netem"
	"github.com/alexei-led/pumba/pkg/util"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

func parseNetemParams(c *cli.Context, interval time.Duration) (*netem.Params, error) {
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
	// validate ips
	ipsList := c.StringSlice("target")
	ips := make([]*net.IPNet, 0, len(ipsList))
	for _, str := range ipsList {
		ip, e := util.ParseCIDR(str)
		if e != nil {
			return nil, errors.Wrap(e, "failed to parse ip")
		}
		ips = append(ips, ip)
	}
	// validate source ports
	sports, err := util.GetPorts(c.String("egress-port"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get source ports")
	}
	// validate destination ports
	dports, err := util.GetPorts(c.String("ingress-port"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get destination ports")
	}
	return &netem.Params{
		Iface:    iface,
		Ips:      ips,
		Sports:   sports,
		Dports:   dports,
		Duration: duration,
		Image:    c.String("tc-image"),
		Pull:     c.Bool("pull-image"),
		Limit:    c.Int("limit"),
	}, nil
}
