//nolint:dupl
package cmd

import (
	"context"
	"fmt"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/netem"
	"github.com/urfave/cli"
)

type lossGEContext struct {
	context context.Context
}

// NewLossGECLICommand initialize CLI loss gemodel command and bind it to the lossContext
func NewLossGECLICommand(ctx context.Context) *cli.Command {
	cmdContext := &lossGEContext{context: ctx}
	return &cli.Command{
		Name: "loss-gemodel",
		Flags: []cli.Flag{
			cli.Float64Flag{
				Name:  "pg, p",
				Usage: "transition probability into the bad state",
				Value: 0.0,
			},
			cli.Float64Flag{
				Name:  "pb, r",
				Usage: "transition probability into the good state",
				Value: 100.0, //nolint:mnd
			},
			cli.Float64Flag{
				Name:  "one-h",
				Usage: "loss probability in the bad state",
				Value: 100.0, //nolint:mnd
			},
			cli.Float64Flag{
				Name:  "one-k",
				Usage: "loss probability in the good state",
				Value: 0.0,
			},
		},
		Usage:     "adds packet losses, according to the Gilbert-Elliot loss model",
		ArgsUsage: fmt.Sprintf("containers (name, list of names, or RE2 regex if prefixed with %q", chaos.Re2Prefix),
		Description: `adds packet losses, according to the Gilbert-Elliot loss model
	 see detailed description: http://www.voiptroubleshooter.com/indepth/burstloss.html`,
		Action: cmdContext.lossGE,
	}
}

// NETEM LOSS GEMODEL Command - network emulation loss by Gilbert-Elliot model
func (cmd *lossGEContext) lossGE(c *cli.Context) error {
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
	// Good State transition probability
	pg := c.Float64("pg")
	// Bad State transition probability
	pb := c.Float64("pb")
	// loss probability in Bad state
	oneH := c.Float64("one-h")
	// loss probability in Good state
	oneK := c.Float64("one-k")

	// init netem loss gemodel command
	lossGECommand, err := netem.NewLossGECommand(chaos.DockerClient, globalParams, netemParams, pg, pb, oneH, oneK)
	if err != nil {
		return fmt.Errorf("error creating loss gemodel command: %w", err)
	}
	// run netem command
	err = chaos.RunChaosCommand(cmd.context, lossGECommand, globalParams)
	if err != nil {
		return fmt.Errorf("error running netem loss gemodel command: %w", err)
	}
	return nil
}
