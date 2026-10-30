package main

import (
	"fmt"

	"github.com/alexei-led/pumba/pkg/chaos"
	ipTablesCmd "github.com/alexei-led/pumba/pkg/chaos/iptables/cmd"
	"github.com/alexei-led/pumba/pkg/chaos/lifecycle/cmd"
	netemCmd "github.com/alexei-led/pumba/pkg/chaos/netem/cmd"
	stressCmd "github.com/alexei-led/pumba/pkg/chaos/stress/cmd"
	"github.com/urfave/cli"
)

//nolint:funlen
func initializeCLICommands(runtime chaos.Runtime, events chaos.EventSink) []cli.Command {
	return []cli.Command{
		*cmd.NewKillCLICommand(topContext, runtime, events),
		*cmd.NewExecCLICommand(topContext, runtime, events),
		*cmd.NewRestartCLICommand(topContext, runtime, events),
		*cmd.NewStopCLICommand(topContext, runtime, events),
		*cmd.NewPauseCLICommand(topContext, runtime, events),
		*cmd.NewRemoveCLICommand(topContext, runtime, events),
		*stressCmd.NewStressCLICommand(topContext, runtime, events),
		{
			Name: "netem",
			Flags: []cli.Flag{
				cli.DurationFlag{
					Name:  "duration, d",
					Usage: "network emulation duration; should be smaller than recurrent interval; use with optional unit suffix: 'ms/s/m/h'",
				},
				cli.StringFlag{
					Name:  "interface, i",
					Usage: "network interface to apply delay on",
					Value: defaultInterface,
				},
				cli.StringSliceFlag{
					Name:  "target, t",
					Usage: "target filter; repeatable IPv4/CIDR or running container name/ID; IPv6 is not supported",
				},
				cli.StringFlag{
					Name:  "egress-port, egressPort",
					Usage: "target port filter for egress, or sport; supports multiple ports (comma-separated)",
				},
				cli.StringFlag{
					Name:  "ingress-port, ingressPort",
					Usage: "target port filter for ingress, or dport; supports multiple ports (comma-separated)",
				},
				cli.StringFlag{
					Name:  "tc-image",
					Usage: "nettools image with sh, grep, which, tail, sleep, tc, and iptables",
					Value: "ghcr.io/alexei-led/pumba-alpine-nettools:latest",
				},
				cli.BoolTFlag{
					Name:  "pull-image",
					Usage: "force pull tc-image",
				},
				cli.IntFlag{
					Name:  "limit",
					Usage: "limit number of matching containers (0: target all)",
					Value: 0,
				},
			},
			Usage:       "emulate the properties of wide area networks",
			ArgsUsage:   fmt.Sprintf("containers (names and RE2 regexes prefixed with %q; may be mixed)", re2Prefix),
			Description: "delay, loss, duplicate and re-order packets, limit bandwidth, or combine effects in one netem qdisc",
			Subcommands: []cli.Command{
				*netemCmd.NewCombineCLICommand(topContext, runtime, events),
				*netemCmd.NewDelayCLICommand(topContext, runtime, events),
				*netemCmd.NewLossCLICommand(topContext, runtime, events),
				*netemCmd.NewLossStateCLICommand(topContext, runtime, events),
				*netemCmd.NewLossGECLICommand(topContext, runtime, events),
				*netemCmd.NewRateCLICommand(topContext, runtime, events),
				*netemCmd.NewDuplicateCLICommand(topContext, runtime, events),
				*netemCmd.NewCorruptCLICommand(topContext, runtime, events),
			},
		},
		{
			Name: "iptables",
			Flags: []cli.Flag{
				cli.DurationFlag{
					Name:  "duration, d",
					Usage: "network emulation duration; should be smaller than recurrent interval; use with optional unit suffix: 'ms/s/m/h'",
				},
				cli.StringFlag{
					Name:  "interface, i",
					Usage: "network interface to apply input rules on",
					Value: defaultInterface,
				},
				cli.StringFlag{
					Name:  "protocol, p",
					Usage: "protocol to apply input rules on (any, udp, tcp or icmp)",
					Value: "any",
				},
				cli.StringSliceFlag{
					Name:  "source, src, s",
					Usage: "source IP filter; supports multiple IPs; supports CIDR notation",
				},
				cli.StringSliceFlag{
					Name:  "destination, dest",
					Usage: "destination IP filter; supports multiple IPs; supports CIDR notation",
				},
				cli.StringFlag{
					Name:  "src-port, sport",
					Usage: "source port filter; supports multiple ports (comma-separated)",
				},
				cli.StringFlag{
					Name:  "dst-port, dport",
					Usage: "destination port filter; supports multiple ports (comma-separated)",
				},
				cli.StringFlag{
					Name:  "iptables-image",
					Usage: "Docker image with iptables and tc (iproute2 package)",
					Value: "ghcr.io/alexei-led/pumba-alpine-nettools:latest",
				},
				cli.BoolTFlag{
					Name:  "pull-image",
					Usage: "force pull iptables-image",
				},
				cli.IntFlag{
					Name:  "limit",
					Usage: "limit number of matching containers (0: target all)",
					Value: 0,
				},
			},
			Usage:       "apply IPv4 packet filter on incoming IP packets",
			ArgsUsage:   fmt.Sprintf("containers (names and RE2 regexes prefixed with %q; may be mixed)", re2Prefix),
			Description: "emulate loss of incoming packets, all ports and address arguments will result in separate rules",
			Subcommands: []cli.Command{
				*ipTablesCmd.NewLossCLICommand(topContext, runtime, events),
			},
		},
	}
}
