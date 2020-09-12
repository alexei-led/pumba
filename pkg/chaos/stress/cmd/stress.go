package cmd

import (
	"context"
	"fmt"

	"github.com/urfave/cli"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/stress"
)

type stressContext struct {
	context context.Context
}

// NewStressCLICommand initialize CLI stress command and bind it to the stressContext
func NewStressCLICommand(ctx context.Context) *cli.Command {
	cmdContext := &stressContext{context: ctx}
	return &cli.Command{
		Name: "stress",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "duration, d",
				Usage: "stress duration: must be shorter than recurrent interval; use with optional unit suffix: 'ms/s/m/h'",
			},
			cli.StringFlag{
				Name:  "stress-image",
				Usage: "Docker image with stress-ng tool, cgroup-bin and docker packages, and dockhack script",
				Value: "alexeiled/stress-ng:latest-ubuntu",
			},
			cli.BoolTFlag{
				Name:  "pull-image",
				Usage: "pull stress-image form Docker registry",
			},
			cli.StringFlag{
				Name:  "stressors",
				Usage: "stress-ng stressors; see https://kernel.ubuntu.com/~cking/stress-ng/",
				Value: "--cpu 4 --timeout 60s",
			},
		},
		Usage:       "stress test a specified containers",
		ArgsUsage:   fmt.Sprintf("containers (name, list of names, or RE2 regex if prefixed with %q)", chaos.Re2Prefix),
		Description: "stress test target container(s)",
		Action:      cmdContext.stress,
	}
}

// stress Command
func (cmd *stressContext) stress(c *cli.Context) error {
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
	// get limit for number of containers to kill
	limit := c.Int("limit")
	// get stress-ng stressors
	stressors := c.String("stressors")
	// get stress duration
	duration := c.String("duration")
	// get stress-ng image
	image := c.String("stress-image")
	// get pull tc image flag
	pull := c.BoolT("pull-image")
	// init stress command
	stressCommand, err := stress.NewStressCommand(chaos.DockerClient, names, pattern, labels, image, pull, stressors, interval, duration, limit, dryRun)
	if err != nil {
		return err
	}
	// run stress command
	return chaos.RunChaosCommand(cmd.context, stressCommand, interval, random, skipError)
}
