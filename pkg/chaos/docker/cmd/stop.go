package cmd

import (
	"context"
	"fmt"

	"github.com/urfave/cli"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/docker"
)

type stopContext struct {
	context context.Context
}

// NewStopCLICommand initialize CLI stop command and bind it to the CommandContext
func NewStopCLICommand(ctx context.Context) *cli.Command {
	cmdContext := &stopContext{ctx}
	return &cli.Command{
		Name: "stop",
		Flags: []cli.Flag{
			cli.IntFlag{
				Name:  "time, t",
				Usage: "seconds to wait for stop before killing container (default 5)",
				Value: docker.DeafultWaitTime,
			},
			cli.IntFlag{
				Name:  "limit, l",
				Usage: "limit number of container to stop (0: stop all matching)",
				Value: 0,
			},
			cli.BoolFlag{
				Name:  "restart, r",
				Usage: "restart stopped container after specified duration",
			},
			cli.StringFlag{
				Name:  "duration, d",
				Usage: "stop duration (works only with `restart` flag): must be shorter than recurrent interval; use with optional unit suffix: 'ms/s/m/h'",
				Value: "10s",
			},
		},
		Usage:       "stop containers",
		ArgsUsage:   fmt.Sprintf("containers (name, list of names, or RE2 regex if prefixed with %q", chaos.Re2Prefix),
		Description: "stop the main process inside target containers, sending  SIGTERM, and then SIGKILL after a grace period",
		Action:      cmdContext.stop,
	}
}

// STOP Command
func (cmd *stopContext) stop(c *cli.Context) error {
	// get random flag
	random := c.GlobalBool("random")
	// get labels
	labels := c.GlobalStringSlice("label")
	// get dry-run mode
	dryRun := c.GlobalBool("dry-run")
	// get global chaos interval
	interval := c.GlobalString("interval")
	// get wait time
	waitTime := c.Int("time")
	// get limit for number of containers to kill
	limit := c.Int("limit")
	// get names or pattern
	names, pattern := chaos.GetNamesOrPattern(c)
	// get restart flag
	restart := c.Bool("restart")
	// get chaos command duration
	duration := c.String("duration")
	// init stop command
	stopCommand, err := docker.NewStopCommand(chaos.DockerClient, names, pattern, labels, restart, interval, duration, waitTime, limit, dryRun)
	if err != nil {
		return err
	}
	// run stop command
	return chaos.RunChaosCommand(cmd.context, stopCommand, interval, random)
}
