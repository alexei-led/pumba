package cmd

import (
	"context"
	"fmt"

	"github.com/urfave/cli"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/docker"
	"github.com/alexei-led/pumba/pkg/container"
)

type pauseContext struct {
	client  container.Client
	context context.Context
}

// NewPauseCommand initialize CLI pause command and bind it to the CommandContext
func NewPauseCommand(ctx context.Context, client container.Client) *cli.Command {
	cmdContext := &pauseContext{client: client, context: ctx}
	return &cli.Command{
		Name: "pause",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "duration, d",
				Usage: "pause duration: must be shorter than recurrent interval; use with optional unit suffix: 'ms/s/m/h'",
			},
			cli.IntFlag{
				Name:  "limit, l",
				Usage: "limit to number of container to kill (0: kill all matching)",
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
	// get random flag
	random := c.GlobalBool("random")
	// get dry-run mode
	dryRun := c.GlobalBool("dry-run")
	// get global chaos interval
	interval := c.GlobalString("interval")
	// get limit for number of containers to kill
	limit := c.Int("limit")
	// get names or pattern
	names, pattern := chaos.GetNamesOrPattern(c)
	// get chaos command duration
	duration := c.String("duration")
	// init pause command
	pauseCommand, err := docker.NewPauseCommand(cmd.client, names, pattern, interval, duration, limit, dryRun)
	if err != nil {
		return err
	}
	// run pause command
	return chaos.RunChaosCommand(cmd.context, pauseCommand, interval, random)
}
