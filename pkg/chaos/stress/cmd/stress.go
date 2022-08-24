package cmd

import (
	"context"
	"fmt"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/stress"

	"github.com/pkg/errors"
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
				Usage: "Docker image with stress-ng tool, cgroup-bin and docker packages, and dockhack script",
				Value: "alexeiled/stress-ng:latest-ubuntu",
			},
			cli.BoolTFlag{
				Name:  "pull-image",
				Usage: "pull stress-image form Docker registry",
			},
                        cli.StringFlag{
                                Name:  "host-socket",
                                Usage: "daemon socket to connect to",
                                Value: "/var/run/docker.sock",
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

// stress stressNg
func (cmd *stressContext) stress(c *cli.Context) error {
	// parse common chaos flags
	globalParams, err := chaos.ParseGlobalParams(c)
	if err != nil {
		return errors.Wrap(err, "error parsing global parameters")
	}
	// get host socket to connect to
	hostSocket := c.String("host-socket")
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
	// init stress command
	stressCommand := stress.NewStressCommand(chaos.DockerClient, globalParams, hostSocket, image, pull, stressors, duration, limit)
	// run stress command
	err = chaos.RunChaosCommand(cmd.context, stressCommand, globalParams)
	if err != nil {
		return errors.Wrap(err, "error running stress command")
	}
	return nil
}
