package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/cliflags"
	chaoscmd "github.com/alexei-led/pumba/pkg/chaos/cmd"
	"github.com/alexei-led/pumba/pkg/chaos/stress"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/urfave/cli"
)

// StressParams holds the per-command parameters for the stress CLI subcommand.
type StressParams struct {
	Image        string
	Pull         bool
	Stressors    string
	Duration     time.Duration
	Limit        int
	InjectCgroup bool
}

// NewStressCLICommand initialize CLI stress command.
func NewStressCLICommand(ctx context.Context, runtime chaos.Runtime) *cli.Command {
	return chaoscmd.NewAction(ctx, runtime, chaoscmd.Spec[StressParams]{
		Name: "stress",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "duration, d",
				Usage: "stress duration: must be shorter than recurrent interval; use with optional unit suffix: 'ms/s/m/h'",
			},
			cli.StringFlag{
				Name: "stress-image",
				// :latest floats forward; --inject-cgroup needs /cg-inject which
				// first shipped in 0.20.01. Pin to ":0.20.01" or newer if local
				// cache predates that.
				Usage: "Docker image with stress-ng tool (must include /cg-inject for --inject-cgroup; first available in 0.20.01)",
				Value: "ghcr.io/alexei-led/stress-ng:latest",
			},
			cli.BoolTFlag{
				Name:  "pull-image",
				Usage: "pull stress-image from Docker registry",
			},
			cli.StringFlag{
				Name:  "stressors",
				Usage: `stress-ng stressors; use = sign to pass values, e.g. --stressors="--cpu 4 --timeout 60s"; see https://kernel.ubuntu.com/~cking/stress-ng/`,
				Value: "--cpu 4 --timeout 60s",
			},
			cli.BoolFlag{
				Name:  "inject-cgroup",
				Usage: "Inject stress-ng into target container's cgroup (same cgroup, shared resource accounting). Requires stress image with cg-inject binary.",
			},
		},
		Usage:       "stress test specified containers",
		ArgsUsage:   fmt.Sprintf("containers (name, list of names, or RE2 regex if prefixed with %q)", chaos.Re2Prefix),
		Description: "stress test target container(s)",
		Parse:       parseStressParams,
		Build:       buildStressCommand,
	})
}

func parseStressParams(c cliflags.Flags, _ *chaos.GlobalParams) (StressParams, error) {
	duration := c.Duration("duration")
	if duration == 0 {
		return StressParams{}, errors.New("unset or invalid duration value")
	}
	return StressParams{
		Image:        c.String("stress-image"),
		Pull:         c.BoolT("pull-image"),
		Stressors:    c.String("stressors"),
		Duration:     duration,
		Limit:        c.Int("limit"),
		InjectCgroup: c.Bool("inject-cgroup"),
	}, nil
}

func buildStressCommand(client container.Client, gp *chaos.GlobalParams, p StressParams) (chaos.Command, error) {
	return stress.NewStressCommand(client, gp, p.Image, p.Pull, p.Stressors, p.Duration, p.Limit, p.InjectCgroup), nil
}
