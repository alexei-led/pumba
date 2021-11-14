package cmd

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/netem"
	"github.com/urfave/cli"
)

type delayContext struct {
	context context.Context
}

// NewDelayCLICommand initialize CLI delay command and bind it to the delayContext
func NewDelayCLICommand(ctx context.Context) *cli.Command {
	cmdContext := &delayContext{context: ctx}
	return &cli.Command{
		Name: "delay",
		Flags: []cli.Flag{
			cli.IntFlag{
				Name:  "time, t",
				Usage: "delay time; in milliseconds",
				Value: 100,
			},
			cli.IntFlag{
				Name:  "jitter, j",
				Usage: "random delay variation (jitter); in milliseconds; example: 100ms ± 10ms",
				Value: 10,
			},
			cli.Float64Flag{
				Name:  "correlation, c",
				Usage: "delay correlation; in percentage",
				Value: 20,
			},
			cli.StringFlag{
				Name:  "distribution, d",
				Usage: "delay distribution, can be one of {<empty> | uniform | normal | pareto |  paretonormal}",
				Value: "",
			},
		},
		Usage:       "delay egress traffic",
		ArgsUsage:   fmt.Sprintf("containers (name, list of names, or RE2 regex if prefixed with %q", chaos.Re2Prefix),
		Description: "delay egress traffic for specified containers; networks show variability so it is possible to add random variation; delay variation isn't purely random, so to emulate that there is a correlation",
		Action:      cmdContext.delay,
	}
}

// NETEM DELAY Command - network emulation delay
func (cmd *delayContext) delay(c *cli.Context) error {
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
	// get delay time
	time := c.Int("time")
	// get delay jitter
	jitter := c.Int("jitter")
	// get delay time
	correlation := c.Float64("correlation")
	// get delay distribution
	distribution := c.String("distribution")

	// init netem delay command
	delayCommand, err := netem.NewDelayCommand(chaos.DockerClient, globalParams, netemParams, time, jitter, correlation, distribution)
	if err != nil {
		return err
	}
	// run netem delay command
	err = chaos.RunChaosCommand(cmd.context, delayCommand, globalParams)
	if err != nil {
		return errors.Wrap(err, "error running netem delay command")
	}
	return nil
}
