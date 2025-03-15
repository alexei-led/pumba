package cmd

import (
	"context"
	"fmt"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/netem"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

type lossStateContext struct {
	context context.Context
}

// NewLossStateCLICommand initialize CLI loss command and bind it to the lossContext
func NewLossStateCLICommand(ctx context.Context) *cli.Command {
	cmdContext := &lossStateContext{context: ctx}
	return &cli.Command{
		Name: "loss-state",
		Flags: []cli.Flag{
			cli.Float64Flag{
				Name:  "p13",
				Usage: "probability to go from state (1) to state (3)",
				Value: 0.0,
			},
			cli.Float64Flag{
				Name:  "p31",
				Usage: "probability to go from state (3) to state (1)",
				Value: 100.0, //nolint:mnd
			},
			cli.Float64Flag{
				Name:  "p32",
				Usage: "probability to go from state (3) to state (2)",
				Value: 0.0,
			},
			cli.Float64Flag{
				Name:  "p23",
				Usage: "probability to go from state (2) to state (3)",
				Value: 100.0, //nolint:mnd
			},
			cli.Float64Flag{
				Name:  "p14",
				Usage: "probability to go from state (1) to state (4)",
				Value: 0.0,
			},
		},
		Usage:     "adds packet losses, based on 4-state Markov probability model",
		ArgsUsage: fmt.Sprintf("containers (name, list of names, or RE2 regex if prefixed with %q", chaos.Re2Prefix),
		Description: `adds a packet losses, based on 4-state Markov probability model:

		state (1) – packet received successfully
		state (2) – packet received within a burst
		state (3) – packet lost within a burst
		tstate (4) – isolated packet lost within a gap

	 	see detailed description: https://www.voiptroubleshooter.com/indepth/burstloss.html`,
		Action: cmdContext.lossState,
	}
}

// NETEM LOSS STATE Command - network emulation loss 4-state Markov
func (cmd *lossStateContext) lossState(c *cli.Context) error {
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
	// get loss p13 state probability
	p13 := c.Float64("p13")
	// get loss p31 state probability
	p31 := c.Float64("p31")
	// get loss p32 state probability
	p32 := c.Float64("p32")
	// get loss p23 state probability
	p23 := c.Float64("p23")
	// get loss p23 state probability
	p14 := c.Float64("p14")

	// init netem loss state command
	lossStateCommand, err := netem.NewLossStateCommand(chaos.DockerClient, globalParams, netemParams, p13, p31, p32, p23, p14)
	if err != nil {
		return errors.Wrap(err, "error creating netem loss state command")
	}
	// run netem command
	err = chaos.RunChaosCommand(cmd.context, lossStateCommand, globalParams)
	if err != nil {
		return errors.Wrap(err, "error running netem loss state command")
	}
	return nil
}
