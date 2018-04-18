package cmd

import (
	"context"
	"fmt"

	"github.com/urfave/cli"

	"github.com/alexei-led/pumba/pkg/chaos/docker"
	"github.com/alexei-led/pumba/pkg/container"
)

// NewRemoveCommand initialize CLI remove command and bind it to the remove4Context
func NewRemoveCommand(ctx context.Context, client container.Client) *cli.Command {
	cmdContext := &commandContext{client: client, context: ctx}
	return &cli.Command{
		Name: "rm",
		Flags: []cli.Flag{
			cli.BoolTFlag{
				Name:  "force, f",
				Usage: "force the removal of a running container (with SIGKILL)",
			},
			cli.BoolTFlag{
				Name:  "links, n",
				Usage: "remove container links",
			},
			cli.BoolTFlag{
				Name:  "volumes, v",
				Usage: "remove volumes associated with the container",
			},
			cli.IntFlag{
				Name:  "limit, l",
				Usage: "limit to number of container to kill (0: kill all matching)",
				Value: 0,
			},
		},
		Usage:       "remove containers",
		ArgsUsage:   fmt.Sprintf("containers (name, list of names, or RE2 regex if prefixed with %q", Re2Prefix),
		Description: "remove target containers, with links and volumes",
		Action:      cmdContext.remove,
	}
}

// REMOVE Command
func (cmd *commandContext) remove(c *cli.Context) error {
	// get random
	random := c.GlobalBool("random")
	// get dry-run mode
	dryRun := c.GlobalBool("dry-run")
	// get interval
	interval := c.GlobalString("interval")
	// get names or pattern
	names, pattern := getNamesOrPattern(c)
	// get force flag
	force := c.BoolT("force")
	// get links flag
	links := c.BoolT("links")
	// get volumes flag
	volumes := c.BoolT("volumes")
	// get limit for number of containers to remove
	limit := c.Int("limit")
	// init remove command
	removeCommand, err := docker.NewRemoveCommand(cmd.client, names, pattern, force, links, volumes, limit, dryRun)
	if err != nil {
		return nil
	}
	// run remove command
	return runChaosCommandX(cmd.context, removeCommand, interval, random)
}
