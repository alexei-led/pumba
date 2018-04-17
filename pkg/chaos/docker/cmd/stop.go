package cmd

import (
	"context"
	"fmt"

	"github.com/urfave/cli"

	"github.com/alexei-led/pumba/pkg/chaos/docker"
	"github.com/alexei-led/pumba/pkg/container"
)

// NewStopCommand initialize CLI stop command and bind it to the commandContext
func NewStopCommand(ctx context.Context, client container.Client) *cli.Command {
	cmdContext := &commandContext{client: client, context: ctx}
	return &cli.Command{
		Name: "stop",
		Flags: []cli.Flag{
			cli.IntFlag{
				Name:  "time, t",
				Usage: "seconds to wait for stop before killing container (default 10)",
				Value: docker.DeafultWaitTime,
			},
		},
		Usage:       "stop containers",
		ArgsUsage:   fmt.Sprintf("containers (name, list of names, or RE2 regex if prefixed with %q", Re2Prefix),
		Description: "stop the main process inside target containers, sending  SIGTERM, and then SIGKILL after a grace period",
		Action:      cmdContext.stop,
	}
}

// STOP Command
func (cmd *commandContext) stop(c *cli.Context) error {
	// get random
	random := c.GlobalBool("random")
	// get dry-run mode
	dryRun := c.GlobalBool("dry-run")
	// get interval
	interval, err := getIntervalValue(c)
	if err != nil {
		return err
	}
	// get wait time
	waitTime := c.Int("time")
	// get limit for number of containers to kill
	limit := c.Int("limit")
	// get names or pattern
	names, pattern := getNamesOrPattern(c)
	// init kill command
	stopCommand := docker.NewStopCommand(cmd.client, names, pattern, waitTime, limit, dryRun)
	// run kill command
	runChaosCommandX(cmd.context, stopCommand, interval, random)
	return nil
}
