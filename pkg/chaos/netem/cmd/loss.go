//nolint:dupl // Generic NewAction[P] enforces a uniform per-command shape; the residual similarity is intentional, not copy-paste.
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

// LossParams holds the per-command parameters for the netem loss subcommand.
type LossParams struct {
	Base        *container.NetemRequest
	Limit       int
	Percent     float64
	Correlation float64
}

// NewLossCLICommand initialize CLI loss command.
func NewLossCLICommand(ctx context.Context, runtime chaos.Runtime) *cli.Command {
	return chaoscmd.NewAction(ctx, runtime, chaoscmd.Spec[LossParams]{
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
		Parse:       parseLossParams,
		Build:       buildLossCommand,
	})
}

func parseLossParams(c cliflags.Flags, gp *chaos.GlobalParams) (LossParams, error) {
	base, limit, err := netem.ParseRequestBase(c.Parent(), gp)
	if err != nil {
		return LossParams{}, fmt.Errorf("error parsing netem parameters: %w", err)
	}
	return LossParams{
		Base:        base,
		Limit:       limit,
		Percent:     c.Float64("percent"),
		Correlation: c.Float64("correlation"),
	}, nil
}

func buildLossCommand(client container.Client, gp *chaos.GlobalParams, p LossParams) (chaos.Command, error) {
	return netem.NewLossCommand(client, gp, p.Base, p.Limit, p.Percent, p.Correlation)
}
