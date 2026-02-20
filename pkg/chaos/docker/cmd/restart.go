package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/docker"
	"github.com/urfave/cli"
)

type restartContext struct {
	context context.Context
}

// NewRestartCLICommand initialize CLI restart command and bind it to the restartContext
func NewRestartCLICommand(ctx context.Context) *cli.Command {
	cmdContext := &restartContext{context: ctx}
	return &cli.Command{
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
		Action:      cmdContext.restart,
	}
}

// RESTART Command
func (cmd *restartContext) restart(c *cli.Context) error {
	// parse common chaos flags
	params, err := chaos.ParseGlobalParams(c)
	if err != nil {
		return fmt.Errorf("error parsing global parameters: %w", err)
	}
	// get timeout
	timeout := c.Duration("timeout")
	// get limit for number of containers to restart
	limit := c.Int("limit")
	// init restart command
	restartCommand := docker.NewRestartCommand(chaos.ContainerClient, params, timeout, limit)
	// run restart command
	err = chaos.RunChaosCommand(cmd.context, restartCommand, params)
	if err != nil {
		return fmt.Errorf("error running restart command: %w", err)
	}
	return nil
}
