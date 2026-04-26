//nolint:dupl // Generic NewAction[P] enforces a uniform per-command shape; the residual similarity is intentional, not copy-paste.
package cmd

import (
	"context"
	"fmt"

	"github.com/alexei-led/pumba/pkg/chaos"
	chaoscmd "github.com/alexei-led/pumba/pkg/chaos/cmd"
	"github.com/alexei-led/pumba/pkg/chaos/netem"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/urfave/cli"
)

// LossGEParams holds the per-command parameters for the netem loss-gemodel subcommand.
type LossGEParams struct {
	Netem *netem.Params
	PG    float64
	PB    float64
	OneH  float64
	OneK  float64
}

// NewLossGECLICommand initialize CLI loss-gemodel command.
func NewLossGECLICommand(ctx context.Context, runtime chaos.Runtime) *cli.Command {
	return chaoscmd.NewAction(ctx, runtime, chaoscmd.Spec[LossGEParams]{
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
		Parse: parseLossGEParams,
		Build: buildLossGECommand,
	})
}

func parseLossGEParams(c *cli.Context, gp *chaos.GlobalParams) (LossGEParams, error) {
	netemParams, err := parseNetemParams(c.Parent(), gp.Interval)
	if err != nil {
		return LossGEParams{}, fmt.Errorf("error parsing netem parameters: %w", err)
	}
	return LossGEParams{
		Netem: netemParams,
		PG:    c.Float64("pg"),
		PB:    c.Float64("pb"),
		OneH:  c.Float64("one-h"),
		OneK:  c.Float64("one-k"),
	}, nil
}

func buildLossGECommand(client container.Client, gp *chaos.GlobalParams, p LossGEParams) (chaos.Command, error) {
	return netem.NewLossGECommand(client, gp, p.Netem, p.PG, p.PB, p.OneH, p.OneK)
}
