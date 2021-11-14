package cmd

import (
	"context"
	"fmt"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/netem"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
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
	// parse common chaos flags
	globalParams, err := chaos.ParseGlobalParams(c)
	if err != nil {
		return errors.Wrap(err, "error parsing global parameters")
	}
	// parse netem flags
	netemParams, err := parseNetemParams(c.Parent(), globalParams.Interval)
	if err != nil {
		return errors.Wrap(err, "error parsing netem parameters")
	}
	// get target egress rate
	rate := c.String("rate")
	// get packet overhead
	packetOverhead := c.Int("packetoverhead")
	// get cell size
	cellSize := c.Int("cellsize")
	// get cell overhead
	cellOverhead := c.Int("celloverhead")

	// init netem rate command
	lossCommand, err := netem.NewRateCommand(chaos.DockerClient, globalParams, netemParams, rate, packetOverhead, cellSize, cellOverhead)
	if err != nil {
		return err
	}
	// run netem command
	err = chaos.RunChaosCommand(cmd.context, lossCommand, globalParams)
	if err != nil {
		return errors.Wrap(err, "error running netem rate command")
	}
	return nil
}
