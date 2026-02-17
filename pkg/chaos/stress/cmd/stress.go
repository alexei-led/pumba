package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/stress"
	"github.com/urfave/cli"
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
				Usage: "Docker image with stress-ng tool",
				Value: "ghcr.io/alexei-led/stress-ng:latest",
			},
			cli.BoolTFlag{
				Name:  "pull-image",
				Usage: "pull stress-image form Docker registry",
			},
			cli.StringFlag{
				Name:  "stressors",
				Usage: `stress-ng stressors; use = sign to pass values, e.g. --stressors="--cpu 4 --timeout 60s"; see https://kernel.ubuntu.com/~cking/stress-ng/`,
				Value: "--cpu 4 --timeout 60s",
			},
			cli.BoolFlag{
				Name:  "inject-cgroup",
				Usage: "Inject stress-ng into target container's cgroup (same cgroup, shared resource accounting). Requires stress image with cg-inject binary.",
			},
		},
		Usage:       "stress test specified containers",
		ArgsUsage:   fmt.Sprintf("containers (name, list of names, or RE2 regex if prefixed with %q)", chaos.Re2Prefix),
		Description: "stress test target container(s)",
		Action:      cmdContext.stress,
	}
}

// stress stressNg
func (cmd *stressContext) stress(c *cli.Context) error {
	// parse common chaos flags
	globalParams, err := chaos.ParseGlobalParams(c)
	if err != nil {
		return fmt.Errorf("error parsing global parameters: %w", err)
	}
	// get limit for number of containers to kill
	limit := c.Int("limit")
	// get stress-ng stressors
	stressors := c.String("stressors")
	// get stress duration
	duration := c.Duration("duration")
	if duration == 0 {
		return errors.New("unset or invalid duration value")
	}
	// get stress-ng image
	image := c.String("stress-image")
	// get pull tc image flag
	pull := c.BoolT("pull-image")
	// get inject-cgroup flag
	injectCgroup := c.Bool("inject-cgroup")
	// init stress command
	stressCommand := stress.NewStressCommand(chaos.DockerClient, globalParams, image, pull, stressors, duration, limit, injectCgroup)
	// run stress command
	err = chaos.RunChaosCommand(cmd.context, stressCommand, globalParams)
	if err != nil {
		return fmt.Errorf("error running stress command: %w", err)
	}
	return nil
}
