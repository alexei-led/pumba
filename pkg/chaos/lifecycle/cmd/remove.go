package cmd

import (
	"context"
	"fmt"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/cliflags"
	chaoscmd "github.com/alexei-led/pumba/pkg/chaos/cmd"
	"github.com/alexei-led/pumba/pkg/chaos/lifecycle"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/urfave/cli"
)

// RemoveParams holds the per-command parameters for the rm CLI subcommand.
type RemoveParams struct {
	Force   bool
	Links   bool
	Volumes bool
	Limit   int
}

// NewRemoveCLICommand initialize CLI remove command.
func NewRemoveCLICommand(ctx context.Context, runtime chaos.Runtime) *cli.Command {
	return chaoscmd.NewAction(ctx, runtime, chaoscmd.Spec[RemoveParams]{
		Name: "rm",
		Flags: []cli.Flag{
			cli.BoolTFlag{
				Name:  "force, f",
				Usage: "force the removal of a running container (with SIGKILL)",
			},
			cli.BoolFlag{
				Name:  "links, n",
				Usage: "remove container links",
			},
			cli.BoolTFlag{
				Name:  "volumes, v",
				Usage: "remove volumes associated with the container",
			},
			cli.IntFlag{
				Name:  "limit, l",
				Usage: "limit number of container to remove (0: remove all matching)",
				Value: 0,
			},
		},
		Usage:       "remove containers",
		ArgsUsage:   fmt.Sprintf("containers (name, list of names, or RE2 regex if prefixed with %q", chaos.Re2Prefix),
		Description: "remove target containers, with links and volumes",
		RequireArgs: true,
		Parse:       parseRemoveParams,
		Build:       buildRemoveCommand,
	})
}

func parseRemoveParams(c cliflags.Flags, _ *chaos.GlobalParams) (RemoveParams, error) {
	return RemoveParams{
		Force:   c.BoolT("force"),
		Links:   c.BoolT("links"),
		Volumes: c.BoolT("volumes"),
		Limit:   c.Int("limit"),
	}, nil
}

func buildRemoveCommand(client container.Client, gp *chaos.GlobalParams, p RemoveParams) (chaos.Command, error) {
	return lifecycle.NewRemoveCommand(client, gp, p.Force, p.Links, p.Volumes, p.Limit), nil
}
