package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	chaoscmd "github.com/alexei-led/pumba/pkg/chaos/cmd"
	"github.com/alexei-led/pumba/pkg/chaos/lifecycle"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/urfave/cli"
)

// StopParams holds the per-command parameters for the stop CLI subcommand.
type StopParams struct {
	WaitTime int
	Limit    int
	Restart  bool
	Duration time.Duration
}

// NewStopCLICommand initialize CLI stop command.
func NewStopCLICommand(ctx context.Context, runtime chaos.Runtime) *cli.Command {
	return chaoscmd.NewAction(ctx, runtime, chaoscmd.Spec[StopParams]{
		Name: "stop",
		Flags: []cli.Flag{
			cli.IntFlag{
				Name:  "time, t",
				Usage: "seconds to wait for stop before killing container (default 5)",
				Value: lifecycle.DeafultWaitTime,
			},
			cli.IntFlag{
				Name:  "limit, l",
				Usage: "limit number of container to stop (0: stop all matching)",
				Value: 0,
			},
			cli.BoolFlag{
				Name:  "restart, r",
				Usage: "restart stopped container after specified duration",
			},
			cli.StringFlag{
				Name:  "duration, d",
				Usage: "stop duration (works only with `restart` flag): must be shorter than recurrent interval; use with optional unit suffix: 'ms/s/m/h'",
				Value: "10s",
			},
		},
		Usage:       "stop containers",
		ArgsUsage:   fmt.Sprintf("containers (name, list of names, or RE2 regex if prefixed with %q", chaos.Re2Prefix),
		Description: "stop the main process inside target containers, sending  SIGTERM, and then SIGKILL after a grace period",
		RequireArgs: true,
		Parse:       parseStopParams,
		Build:       buildStopCommand,
	})
}

func parseStopParams(c *cli.Context, _ *chaos.GlobalParams) (StopParams, error) {
	duration := c.Duration("duration")
	if duration == 0 {
		return StopParams{}, errors.New("unset or invalid duration value")
	}
	return StopParams{
		WaitTime: c.Int("time"),
		Limit:    c.Int("limit"),
		Restart:  c.Bool("restart"),
		Duration: duration,
	}, nil
}

func buildStopCommand(client container.Client, gp *chaos.GlobalParams, p StopParams) (chaos.Command, error) {
	return lifecycle.NewStopCommand(client, gp, p.Restart, p.Duration, p.WaitTime, p.Limit), nil
}
