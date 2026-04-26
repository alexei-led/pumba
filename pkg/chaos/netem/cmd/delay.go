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

// DelayParams holds the per-command parameters for the netem delay subcommand.
type DelayParams struct {
	Netem        *netem.Params
	Time         int
	Jitter       int
	Correlation  float64
	Distribution string
}

// NewDelayCLICommand initialize CLI delay command.
func NewDelayCLICommand(ctx context.Context, runtime chaos.Runtime) *cli.Command {
	return chaoscmd.NewAction(ctx, runtime, chaoscmd.Spec[DelayParams]{
		Name: "delay",
		Flags: []cli.Flag{
			cli.IntFlag{
				Name:  "time, t",
				Usage: "delay time; in milliseconds",
				Value: 100, //nolint:mnd
			},
			cli.IntFlag{
				Name:  "jitter, j",
				Usage: "random delay variation (jitter); in milliseconds; example: 100ms ± 10ms",
				Value: 10, //nolint:mnd
			},
			cli.Float64Flag{
				Name:  "correlation, c",
				Usage: "delay correlation; in percentage",
				Value: 20, //nolint:mnd
			},
			cli.StringFlag{
				Name:  "distribution, d",
				Usage: "delay distribution, can be one of {<empty> | uniform | normal | pareto |  paretonormal}",
				Value: "",
			},
		},
		Usage:       "delay egress traffic",
		ArgsUsage:   fmt.Sprintf("containers (name, list of names, or RE2 regex if prefixed with %q", chaos.Re2Prefix),
		Description: "delay egress traffic for specified containers; networks show variability so it is possible to add random variation; delay variation isn't purely random, so to emulate that there is a correlation",
		Parse:       parseDelayParams,
		Build:       buildDelayCommand,
	})
}

func parseDelayParams(c cliflags.Flags, gp *chaos.GlobalParams) (DelayParams, error) {
	netemParams, err := parseNetemParams(c.Parent(), gp.Interval)
	if err != nil {
		return DelayParams{}, fmt.Errorf("error parsing netem parameters: %w", err)
	}
	return DelayParams{
		Netem:        netemParams,
		Time:         c.Int("time"),
		Jitter:       c.Int("jitter"),
		Correlation:  c.Float64("correlation"),
		Distribution: c.String("distribution"),
	}, nil
}

func buildDelayCommand(client container.Client, gp *chaos.GlobalParams, p DelayParams) (chaos.Command, error) {
	return netem.NewDelayCommand(client, gp, p.Netem, p.Time, p.Jitter, p.Correlation, p.Distribution)
}
