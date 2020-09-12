package cmd

import (
	"context"
	"fmt"

	"github.com/urfave/cli"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/netem"
)

type rateContext struct {
	context context.Context
}

// NewRateCLICommand initialize CLI rate command and bind it to the lossContext
func NewRateCLICommand(ctx context.Context) *cli.Command {
	cmdContext := &rateContext{context: ctx}
	return &cli.Command{
		Name: "rate",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "rate, r",
				Usage: "delay outgoing packets; in common units",
				Value: "100kbit",
			},
			cli.IntFlag{
				Name:  "packetoverhead, p",
				Usage: "per packet overhead; in bytes",
				Value: 0,
			},
			cli.IntFlag{
				Name:  "cellsize, s",
				Usage: "cell size of the simulated link layer scheme",
				Value: 0,
			},
			cli.IntFlag{
				Name:  "celloverhead, c",
				Usage: "per cell overhead; in bytes",
				Value: 0,
			},
		},
		Usage:       "rate limit egress traffic",
		ArgsUsage:   fmt.Sprintf("containers (name, list of names, or RE2 regex if prefixed with %q", chaos.Re2Prefix),
		Description: "rate limit egress traffic for specified containers",
		Action:      cmdContext.rate,
	}
}

// NETEM RATE Command - network emulation rate
func (cmd *rateContext) rate(c *cli.Context) error {
	// get random flag
	random := c.GlobalBool("random")
	// get labels
	labels := c.GlobalStringSlice("label")
	// get dry-run mode
	dryRun := c.GlobalBool("dry-run")
	// get skip error flag
	skipError := c.GlobalBool("skip-error")
	// get names or pattern
	names, pattern := chaos.GetNamesOrPattern(c)
	// get global chaos interval
	interval := c.GlobalString("interval")

	// get network interface from parent `netem` command
	iface := c.Parent().String("interface")
	// get ips list from parent `netem`` command `target` flag
	ips := c.Parent().StringSlice("target")
	// get egress port list from parent `netem` command `egressPort` flag
	sports := c.Parent().String("egressPort")
	// get ingress port list from parent `netem` command `ingressPort` flag
	dports := c.Parent().String("ingressPort")
	// get duration from parent `netem`` command
	duration := c.Parent().String("duration")
	// get traffic control image from parent `netem` command
	image := c.Parent().String("tc-image")
	// get pull tc image flag
	pull := c.Parent().BoolT("pull-image")
	// get limit for number of containers to netem command
	limit := c.Parent().Int("limit")

	// get target egress rate
	rate := c.String("rate")
	// get packet overhead
	packetOverhead := c.Int("packetoverhead")
	// get cell size
	cellSize := c.Int("cellsize")
	// get cell overhead
	cellOverhead := c.Int("celloverhead")

	// init netem rate command
	lossCommand, err := netem.NewRateCommand(chaos.DockerClient, names, pattern, labels, iface, ips, sports, dports, duration, interval, rate, packetOverhead, cellSize, cellOverhead, image, pull, limit, dryRun)
	if err != nil {
		return err
	}
	// run netem command
	return chaos.RunChaosCommand(cmd.context, lossCommand, interval, random, skipError)
}
