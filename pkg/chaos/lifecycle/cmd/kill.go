package cmd

import (
	"context"
	"fmt"

	"github.com/alexei-led/pumba/pkg/chaos"
	chaoscmd "github.com/alexei-led/pumba/pkg/chaos/cmd"
	"github.com/alexei-led/pumba/pkg/chaos/lifecycle"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/urfave/cli"
)

// KillParams holds the per-command parameters for the kill CLI subcommand.
type KillParams struct {
	Signal string
	Limit  int
}

// NewKillCLICommand initialize CLI kill command.
func NewKillCLICommand(ctx context.Context, runtime chaos.Runtime) *cli.Command {
	return chaoscmd.NewAction(ctx, runtime, chaoscmd.Spec[KillParams]{
		Name: "kill",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "signal, s",
				Usage: "termination signal, that will be sent by Pumba to the main process inside target container(s)",
				Value: lifecycle.DefaultKillSignal,
			},
			cli.IntFlag{
				Name:  "limit, l",
				Usage: "limit number of container to kill (0: kill all matching)",
				Value: 0,
			},
		},
		Usage:       "kill specified containers",
		ArgsUsage:   fmt.Sprintf("containers (name, list of names, or RE2 regex if prefixed with %q)", chaos.Re2Prefix),
		Description: "send termination signal to the main process inside target container(s)",
		RequireArgs: true,
		Parse:       parseKillParams,
		Build:       buildKillCommand,
	})
}

func parseKillParams(c *cli.Context, _ *chaos.GlobalParams) (KillParams, error) {
	return KillParams{
		Signal: c.String("signal"),
		Limit:  c.Int("limit"),
	}, nil
}

func buildKillCommand(client container.Client, gp *chaos.GlobalParams, p KillParams) (chaos.Command, error) {
	return lifecycle.NewKillCommand(client, gp, p.Signal, p.Limit)
}
