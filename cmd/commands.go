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
func initializeCLICommands(runtime chaos.Runtime) []cli.Command {
	return []cli.Command{
		*cmd.NewKillCLICommand(topContext, runtime),
		*cmd.NewExecCLICommand(topContext, runtime),
		*cmd.NewRestartCLICommand(topContext, runtime),
		*cmd.NewStopCLICommand(topContext, runtime),
		*cmd.NewPauseCLICommand(topContext, runtime),
		*cmd.NewRemoveCLICommand(topContext, runtime),
		*stressCmd.NewStressCLICommand(topContext, runtime),
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
					Usage: "target IP filter; supports multiple IPs; supports CIDR notation",
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
					Usage: "Docker image with tc (iproute2 package) and iptables",
					Value: "ghcr.io/alexei-led/pumba-alpine-nettools:latest",
				},
				cli.BoolTFlag{
					Name:  "pull-image",
					Usage: "force pull tc-image",
				},
			},
			Usage:       "emulate the properties of wide area networks",
			ArgsUsage:   fmt.Sprintf("containers (name, list of names, or RE2 regex if prefixed with %q", re2Prefix),
			Description: "delay, loss, duplicate and re-order (run 'netem') packets, and limit the bandwidth, to emulate different network problems",
			Subcommands: []cli.Command{
				*netemCmd.NewDelayCLICommand(topContext, runtime),
				*netemCmd.NewLossCLICommand(topContext, runtime),
				*netemCmd.NewLossStateCLICommand(topContext, runtime),
				*netemCmd.NewLossGECLICommand(topContext, runtime),
				*netemCmd.NewRateCLICommand(topContext, runtime),
				*netemCmd.NewDuplicateCLICommand(topContext, runtime),
				*netemCmd.NewCorruptCLICommand(topContext, runtime),
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
			},
			Usage:       "apply IPv4 packet filter on incoming IP packets",
			ArgsUsage:   fmt.Sprintf("containers (name, list of names, or RE2 regex if prefixed with %q", re2Prefix),
			Description: "emulate loss of incoming packets, all ports and address arguments will result in separate rules",
			Subcommands: []cli.Command{
				*ipTablesCmd.NewLossCLICommand(topContext, runtime),
			},
		},
	}
}
