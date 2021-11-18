package cmd

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/docker"
	"github.com/urfave/cli"
)

type killContext struct {
	context context.Context
}

// NewKillCLICommand initialize CLI kill command and bind it to the killContext
func NewKillCLICommand(ctx context.Context) *cli.Command {
	cmdContext := &killContext{context: ctx}
	return &cli.Command{
		Name: "kill",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "signal, s",
				Usage: "termination signal, that will be sent by Pumba to the main process inside target container(s)",
				Value: docker.DefaultKillSignal,
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
		Action:      cmdContext.kill,
	}
}

// KILL Command
func (cmd *killContext) kill(c *cli.Context) error {
	// parse common chaos flags
	params, err := chaos.ParseGlobalParams(c)
	if err != nil {
		return errors.Wrap(err, "error parsing global parameters")
	}
	// get signal
	signal := c.String("signal")
	// get limit for number of containers to kill
	limit := c.Int("limit")
	// init kill command
	killCommand, err := docker.NewKillCommand(chaos.DockerClient, params, signal, limit)
	if err != nil {
		return errors.Wrap(err, "could not create kill command")
	}
	// run kill command
	err = chaos.RunChaosCommand(cmd.context, killCommand, params)
	if err != nil {
		return errors.Wrap(err, "could not kill containers")
	}
	return nil
}
