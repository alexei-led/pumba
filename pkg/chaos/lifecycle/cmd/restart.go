package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/cliflags"
	chaoscmd "github.com/alexei-led/pumba/pkg/chaos/cmd"
	"github.com/alexei-led/pumba/pkg/chaos/lifecycle"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/urfave/cli"
)

// RestartParams holds the per-command parameters for the restart CLI subcommand.
type RestartParams struct {
	Timeout time.Duration
	Limit   int
}

// NewRestartCLICommand initialize CLI restart command.
func NewRestartCLICommand(ctx context.Context, runtime chaos.Runtime) *cli.Command {
	return chaoscmd.NewAction(ctx, runtime, chaoscmd.Spec[RestartParams]{
		Name: "restart",
		Flags: []cli.Flag{
			cli.DurationFlag{
				Name:  "timeout, t",
				Usage: "time to wait before killing the container",
				Value: 1 * time.Second,
			},
			cli.IntFlag{
				Name:  "limit, l",
				Usage: "limit number of container to restart (0: restart all matching)",
				Value: 0,
			},
		},
		Usage:       "restart specified containers",
		ArgsUsage:   fmt.Sprintf("containers (name, list of names, or RE2 regex if prefixed with %q)", chaos.Re2Prefix),
		Description: "send command to target container(s)",
		Parse:       parseRestartParams,
		Build:       buildRestartCommand,
	})
}

func parseRestartParams(c cliflags.Flags, _ *chaos.GlobalParams) (RestartParams, error) {
	return RestartParams{
		Timeout: c.Duration("timeout"),
		Limit:   c.Int("limit"),
	}, nil
}

func buildRestartCommand(client container.Client, gp *chaos.GlobalParams, p RestartParams) (chaos.Command, error) {
	return lifecycle.NewRestartCommand(client, gp, p.Timeout, p.Limit), nil
}
