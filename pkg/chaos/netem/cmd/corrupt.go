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

// CorruptParams holds the per-command parameters for the netem corrupt subcommand.
type CorruptParams struct {
	Netem       *netem.Params
	Percent     float64
	Correlation float64
}

// NewCorruptCLICommand initialize CLI corrupt command.
func NewCorruptCLICommand(ctx context.Context, runtime chaos.Runtime) *cli.Command {
	return chaoscmd.NewAction(ctx, runtime, chaoscmd.Spec[CorruptParams]{
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
		Parse:       parseCorruptParams,
		Build:       buildCorruptCommand,
	})
}

func parseCorruptParams(c cliflags.Flags, gp *chaos.GlobalParams) (CorruptParams, error) {
	netemParams, err := parseNetemParams(c.Parent(), gp.Interval)
	if err != nil {
		return CorruptParams{}, fmt.Errorf("error parsing netem parameters: %w", err)
	}
	return CorruptParams{
		Netem:       netemParams,
		Percent:     c.Float64("percent"),
		Correlation: c.Float64("correlation"),
	}, nil
}

func buildCorruptCommand(client container.Client, gp *chaos.GlobalParams, p CorruptParams) (chaos.Command, error) {
	return netem.NewCorruptCommand(client, gp, p.Netem, p.Percent, p.Correlation)
}
