package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/cliflags"
	chaoscmd "github.com/alexei-led/pumba/pkg/chaos/cmd"
	"github.com/alexei-led/pumba/pkg/chaos/lifecycle"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/urfave/cli"
)

// PauseParams holds the per-command parameters for the pause CLI subcommand.
type PauseParams struct {
	Duration time.Duration
	Limit    int
}

// NewPauseCLICommand initialize CLI pause command.
func NewPauseCLICommand(ctx context.Context, runtime chaos.Runtime) *cli.Command {
	return chaoscmd.NewAction(ctx, runtime, chaoscmd.Spec[PauseParams]{
		Name: "pause",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "duration, d",
				Usage: "pause duration: must be shorter than recurrent interval; use with optional unit suffix: 'ms/s/m/h'",
			},
			cli.IntFlag{
				Name:  "limit, l",
				Usage: "limit number of container to pause (0: pause all matching)",
				Value: 0,
			},
		},
		Usage:       "pause all processes",
		ArgsUsage:   fmt.Sprintf("containers (name, list of names, or RE2 regex if prefixed with %q", chaos.Re2Prefix),
		Description: "pause all running processes within target containers",
		Parse:       parsePauseParams,
		Build:       buildPauseCommand,
	})
}

func parsePauseParams(c cliflags.Flags, _ *chaos.GlobalParams) (PauseParams, error) {
	duration := c.Duration("duration")
	if duration == 0 {
		return PauseParams{}, errors.New("unset or invalid duration value")
	}
	return PauseParams{
		Duration: duration,
		Limit:    c.Int("limit"),
	}, nil
}

func buildPauseCommand(client container.Client, gp *chaos.GlobalParams, p PauseParams) (chaos.Command, error) {
	return lifecycle.NewPauseCommand(client, gp, p.Duration, p.Limit), nil
}
