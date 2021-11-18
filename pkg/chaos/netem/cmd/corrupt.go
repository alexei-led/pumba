package cmd

import (
	"context"
	"fmt"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/netem"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

type corruptContext struct {
	context context.Context
}

// NewCorruptCLICommand initialize CLI corrupt command and bind it to the corruptContext
func NewCorruptCLICommand(ctx context.Context) *cli.Command {
	cmdContext := &corruptContext{context: ctx}
	return &cli.Command{
		Name: "corrupt",
		Flags: []cli.Flag{
			cli.Float64Flag{
				Name:  "percent, p",
				Usage: "packet corrupt percentage",
				Value: 0.0,
			},
			cli.Float64Flag{
				Name:  "correlation, c",
				Usage: "corrupt correlation; in percentage",
				Value: 0.0,
			},
		},
		Usage:       "adds packet corruption",
		ArgsUsage:   fmt.Sprintf("containers (name, list of names, or RE2 regex if prefixed with %q", chaos.Re2Prefix),
		Description: "adds packet corruption, based on independent (Bernoulli) probability model\n \tsee:  http://www.voiptroubleshooter.com/indepth/burstloss.html",
		Action:      cmdContext.corrupt,
	}
}

// NETEM Corrupt Command - network emulation corrupt
func (cmd *corruptContext) corrupt(c *cli.Context) error {
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
	// get corrupt percentage
	percent := c.Float64("percent")
	// get delay variation
	correlation := c.Float64("correlation")
	// init netem corrupt command
	corruptCommand, err := netem.NewCorruptCommand(chaos.DockerClient, globalParams, netemParams, percent, correlation)
	if err != nil {
		return errors.Wrap(err, "error creating netem corrupt command")
	}
	// run netem command
	err = chaos.RunChaosCommand(cmd.context, corruptCommand, globalParams)
	if err != nil {
		return errors.Wrap(err, "error running netem corrupt command")
	}
	return nil
}
