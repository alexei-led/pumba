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

// DuplicateParams holds the per-command parameters for the netem duplicate subcommand.
type DuplicateParams struct {
	Netem       *netem.Params
	Percent     float64
	Correlation float64
}

// NewDuplicateCLICommand initialize CLI duplicate command.
func NewDuplicateCLICommand(ctx context.Context, runtime chaos.Runtime) *cli.Command {
	return chaoscmd.NewAction(ctx, runtime, chaoscmd.Spec[DuplicateParams]{
		Name: "duplicate",
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
		Parse:       parseDuplicateParams,
		Build:       buildDuplicateCommand,
	})
}

func parseDuplicateParams(c cliflags.Flags, gp *chaos.GlobalParams) (DuplicateParams, error) {
	netemParams, err := parseNetemParams(c.Parent(), gp.Interval)
	if err != nil {
		return DuplicateParams{}, fmt.Errorf("error parsing netem parameters: %w", err)
	}
	return DuplicateParams{
		Netem:       netemParams,
		Percent:     c.Float64("percent"),
		Correlation: c.Float64("correlation"),
	}, nil
}

func buildDuplicateCommand(client container.Client, gp *chaos.GlobalParams, p DuplicateParams) (chaos.Command, error) {
	return netem.NewDuplicateCommand(client, gp, p.Netem, p.Percent, p.Correlation)
}
