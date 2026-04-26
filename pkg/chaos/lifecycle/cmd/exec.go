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

// ExecParams holds the per-command parameters for the exec CLI subcommand.
type ExecParams struct {
	Command string
	Args    []string
	Limit   int
}

// NewExecCLICommand initialize CLI exec command.
func NewExecCLICommand(ctx context.Context, runtime chaos.Runtime) *cli.Command {
	return chaoscmd.NewAction(ctx, runtime, chaoscmd.Spec[ExecParams]{
		Name: "exec",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "command, s",
				Usage: "shell command, that will be sent by Pumba to the target container(s)",
				Value: "kill 1",
			},
			cli.StringSliceFlag{
				Name:  "args, a",
				Usage: "additional arguments for the command",
			},
			cli.IntFlag{
				Name:  "limit, l",
				Usage: "limit number of container to exec (0: exec all matching)",
				Value: 0,
			},
		},
		Usage:       "exec specified containers",
		ArgsUsage:   fmt.Sprintf("containers (name, list of names, or RE2 regex if prefixed with %q)", chaos.Re2Prefix),
		Description: "send command to target container(s)",
		Parse:       parseExecParams,
		Build:       buildExecCommand,
	})
}

func parseExecParams(c *cli.Context, _ *chaos.GlobalParams) (ExecParams, error) {
	return ExecParams{
		Command: c.String("command"),
		Args:    c.StringSlice("args"),
		Limit:   c.Int("limit"),
	}, nil
}

func buildExecCommand(client container.Client, gp *chaos.GlobalParams, p ExecParams) (chaos.Command, error) {
	return lifecycle.NewExecCommand(client, gp, p.Command, p.Args, p.Limit), nil
}
