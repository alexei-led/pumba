package cmd

import (
	"context"
	"fmt"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/cliflags"
	chaoscmd "github.com/alexei-led/pumba/pkg/chaos/cmd"
	"github.com/alexei-led/pumba/pkg/chaos/netem"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/urfave/cli"
)

// LossStateParams holds the per-command parameters for the netem loss-state subcommand.
type LossStateParams struct {
	Netem *netem.Params
	P13   float64
	P31   float64
	P32   float64
	P23   float64
	P14   float64
}

// NewLossStateCLICommand initialize CLI loss-state command.
func NewLossStateCLICommand(ctx context.Context, runtime chaos.Runtime) *cli.Command {
	return chaoscmd.NewAction(ctx, runtime, chaoscmd.Spec[LossStateParams]{
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
		Parse: parseLossStateParams,
		Build: buildLossStateCommand,
	})
}

func parseLossStateParams(c cliflags.Flags, gp *chaos.GlobalParams) (LossStateParams, error) {
	netemParams, err := parseNetemParams(c.Parent(), gp.Interval)
	if err != nil {
		return LossStateParams{}, fmt.Errorf("error parsing netem parameters: %w", err)
	}
	return LossStateParams{
		Netem: netemParams,
		P13:   c.Float64("p13"),
		P31:   c.Float64("p31"),
		P32:   c.Float64("p32"),
		P23:   c.Float64("p23"),
		P14:   c.Float64("p14"),
	}, nil
}

func buildLossStateCommand(client container.Client, gp *chaos.GlobalParams, p LossStateParams) (chaos.Command, error) {
	return netem.NewLossStateCommand(client, gp, p.Netem, p.P13, p.P31, p.P32, p.P23, p.P14)
}
