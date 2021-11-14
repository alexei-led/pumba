package cmd

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/docker"
	"github.com/urfave/cli"
)

type pauseContext struct {
	context context.Context
}

// NewPauseCLICommand initialize CLI pause command and bind it to the CommandContext
func NewPauseCLICommand(ctx context.Context) *cli.Command {
	cmdContext := &pauseContext{context: ctx}
	return &cli.Command{
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
		Action:      cmdContext.pause,
	}
}

// PAUSE Command
func (cmd *pauseContext) pause(c *cli.Context) error {
	// parse common chaos flags
	params, err := chaos.ParseGlobalParams(c)
	// get limit for number of containers to kill
	limit := c.Int("limit")
	// get chaos command duration
	duration := c.String("duration")
	// init pause command
	pauseCommand, err := docker.NewPauseCommand(chaos.DockerClient, params, duration, limit)
	if err != nil {
		return errors.Wrap(err, "could not create pause command")
	}
	// run pause command
	err = chaos.RunChaosCommand(cmd.context, pauseCommand, params)
	if err != nil {
		return errors.Wrap(err, "could not pause containers")
	}
	return nil
}
