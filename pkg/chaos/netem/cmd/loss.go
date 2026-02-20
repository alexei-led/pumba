package cmd

import (
	"context"
	"fmt"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/netem"
	"github.com/urfave/cli"
)

type lossContext struct {
	context context.Context
}

// NewLossCLICommand initialize CLI loss command and bind it to the lossContext
func NewLossCLICommand(ctx context.Context) *cli.Command {
	cmdContext := &lossContext{context: ctx}
	return &cli.Command{
		Name: "loss",
		Flags: []cli.Flag{
			cli.Float64Flag{
				Name:  "percent, p",
				Usage: "packet loss percentage",
				Value: 0.0,
			},
			cli.Float64Flag{
				Name:  "correlation, c",
				Usage: "loss correlation; in percentage",
				Value: 0.0,
			},
		},
		Usage:       "adds packet losses",
		ArgsUsage:   fmt.Sprintf("containers (name, list of names, or RE2 regex if prefixed with %q", chaos.Re2Prefix),
		Description: "adds packet losses, based on independent (Bernoulli) probability model\n \tsee:  http://www.voiptroubleshooter.com/indepth/burstloss.html",
		Action:      cmdContext.loss,
	}
}

// NETEM LOSS Command - network emulation loss
//
//nolint:dupl
func (cmd *lossContext) loss(c *cli.Context) error {
	// parse common chaos flags
	globalParams, err := chaos.ParseGlobalParams(c)
	if err != nil {
		return fmt.Errorf("error parsing global parameters: %w", err)
	}
	// parse netem flags
	netemParams, err := parseNetemParams(c.Parent(), globalParams.Interval)
	if err != nil {
		return fmt.Errorf("error parsing netem parameters: %w", err)
	}
	// get loss percentage
	percent := c.Float64("percent")
	// get delay variation
	correlation := c.Float64("correlation")
	// init netem loss command
	lossCommand, err := netem.NewLossCommand(chaos.ContainerClient, globalParams, netemParams, percent, correlation)
	if err != nil {
		return fmt.Errorf("error creating netem loss command: %w", err)
	}
	// run netem command
	err = chaos.RunChaosCommand(cmd.context, lossCommand, globalParams)
	if err != nil {
		return fmt.Errorf("error running netem loss command: %w", err)
	}
	return nil
}
