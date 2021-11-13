package cmd

import (
	"context"
	"fmt"
	"github.com/pkg/errors"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/docker"
	"github.com/urfave/cli"
)

type execContext struct {
	context context.Context
}

// NewExecCLICommand initialize CLI exec command and bind it to the execContext
func NewExecCLICommand(ctx context.Context) *cli.Command {
	cmdContext := &execContext{context: ctx}
	return &cli.Command{
		Name: "exec",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "command, s",
				Usage: "shell command, that will be sent by Pumba to the target container(s)",
				Value: "kill 1",
			},
			cli.IntFlag{
				Name:  "limit, l",
				Usage: "limit number of container to exec (0: exec all matching)",
				Value: 0,
			},
		},
		Usage:       "exec specified containers",
		ArgsUsage:   fmt.Sprintf("containers (name, list of names, or RE2 regex if prefixed with %q)", chaos.Re2Prefix),
		Description: "send command to target container(s)",
		Action:      cmdContext.exec,
	}
}

// EXEC Command
func (cmd *execContext) exec(c *cli.Context) error {
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
	command := c.String("command")
	// get limit for number of containers to exec
	limit := c.Int("limit")
	// init exec command
	execCommand, err := docker.NewExecCommand(chaos.DockerClient, names, pattern, labels, command, limit, dryRun)
	if err != nil {
		return errors.Wrap(err, "could not create exec command")
	}
	// run exec command
	err = chaos.RunChaosCommand(cmd.context, execCommand, interval, random, skipError)
	if err != nil {
		return errors.Wrap(err, "could not run exec command")
	}
	return nil
}
