package cmd

import (
	"context"
	"fmt"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/netem"
	"github.com/urfave/cli"
)

const (
	duplicateCmd = "duplicate"
)

type duplicateContext struct {
	context context.Context
}

// NewDuplicateCLICommand initialize CLI duplicate command and bind it to the duplicateContext
func NewDuplicateCLICommand(ctx context.Context) *cli.Command {
	cmdContext := &duplicateContext{context: ctx}
	return &cli.Command{
		Name: duplicateCmd,
		Flags: []cli.Flag{
			cli.Float64Flag{
				Name:  "percent, p",
				Usage: "packet duplicate percentage",
				Value: 0.0,
			},
			cli.Float64Flag{
				Name:  "correlation, c",
				Usage: "duplicate correlation; in percentage",
				Value: 0.0,
			},
		},
		Usage:       "adds packet duplication",
		ArgsUsage:   fmt.Sprintf("containers (name, list of names, or RE2 regex if prefixed with %q", chaos.Re2Prefix),
		Description: "adds packet duplication, based on independent (Bernoulli) probability model\n \tsee:  http://www.voiptroubleshooter.com/indepth/burstloss.html",
		Action:      cmdContext.duplicate,
	}
}

// NETEM Duplicate Command - network emulation duplicate
//
//nolint:dupl
func (cmd *duplicateContext) duplicate(c *cli.Context) error {
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
	// get duplicate percentage
	percent := c.Float64("percent")
	// get delay variation
	correlation := c.Float64("correlation")

	// init netem duplicate command
	duplicateCommand, err := netem.NewDuplicateCommand(chaos.ContainerClient, globalParams, netemParams, percent, correlation)
	if err != nil {
		return fmt.Errorf("unable to create netem duplicate command: %w", err)
	}
	// run netem command
	err = chaos.RunChaosCommand(cmd.context, duplicateCommand, globalParams)
	if err != nil {
		return fmt.Errorf("error running netem duplicate command: %w", err)
	}
	return nil
}
