package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/urfave/cli"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/docker"
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
			cli.IntFlag{
				Name:  "timeout, s",
				Usage: "restart timeout for target container(s)",
				Value: 1000,
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
	// get random
	random := c.GlobalBool("random")
	// get labels
	labels := c.GlobalStringSlice("label")
	// get dry-run mode
	dryRun := c.GlobalBool("dry-run")
	// get skip error flag
	skipError := c.GlobalBool("skip-error")
	// get interval
	interval := c.GlobalString("interval")
	// get names or pattern
	names, pattern := chaos.GetNamesOrPattern(c)
	// get command
	timeout := time.Duration(c.Int("timeout")) * time.Millisecond
	// get limit for number of containers to restart
	limit := c.Int("limit")
	// init restart command
	restartCommand, err := docker.NewRestartCommand(chaos.DockerClient, names, pattern, labels, timeout, limit, dryRun)
	if err != nil {
		return err
	}
	// run restart command
	return chaos.RunChaosCommand(cmd.context, restartCommand, interval, random, skipError)
}
