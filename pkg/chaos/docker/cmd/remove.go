package cmd

import (
	"context"
	"fmt"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/docker"
	"github.com/urfave/cli"
)

type removeContext struct {
	context context.Context
}

// NewRemoveCLICommand initialize CLI remove command and bind it to the remove4Context
func NewRemoveCLICommand(ctx context.Context) *cli.Command {
	cmdContext := &removeContext{context: ctx}
	return &cli.Command{
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
		Action:      cmdContext.remove,
	}
}

// REMOVE Command
func (cmd *removeContext) remove(c *cli.Context) error {
	// parse common chaos flags
	params, err := chaos.ParseGlobalParams(c)
	// get force flag
	force := c.BoolT("force")
	// get links flag
	links := c.BoolT("links")
	// get volumes flag
	volumes := c.BoolT("volumes")
	// get limit for number of containers to remove
	limit := c.Int("limit")
	// init remove command
	removeCommand, err := docker.NewRemoveCommand(chaos.DockerClient, params, force, links, volumes, limit)
	if err != nil {
		return err
	}
	// run remove command
	return chaos.RunChaosCommand(cmd.context, removeCommand, params)
}
