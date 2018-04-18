package cmd

import (
	"context"
	"fmt"

	"github.com/urfave/cli"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
)

type delayContext struct {
	client  container.Client
	context context.Context
}

// NewDelayCommand initialize CLI delay command and bind it to the delayContext
func NewDelayCommand(ctx context.Context, client container.Client) *cli.Command {
	cmdContext := &delayContext{client: client, context: ctx}
	return &cli.Command{
		Name: "delay",
		Flags: []cli.Flag{
			cli.IntFlag{
				Name:  "time, t",
				Usage: "delay time; in milliseconds",
				Value: 100,
			},
			cli.IntFlag{
				Name:  "jitter, j",
				Usage: "random delay variation (jitter); in milliseconds; example: 100ms ± 10ms",
				Value: 10,
			},
			cli.Float64Flag{
				Name:  "correlation, c",
				Usage: "delay correlation; in percentage",
				Value: 20,
			},
			cli.StringFlag{
				Name:  "distribution, d",
				Usage: "delay distribution, can be one of {<empty> | uniform | normal | pareto |  paretonormal}",
				Value: "",
			},
		},
		Usage:       "delay egress traffic",
		ArgsUsage:   fmt.Sprintf("containers (name, list of names, or RE2 regex if prefixed with %q", chaos.Re2Prefix),
		Description: "delay egress traffic for specified containers; networks show variability so it is possible to add random variation; delay variation isn't purely random, so to emulate that there is a correlation",
		Action:      cmdContext.netemDelay,
	}
}

// NETEM DELAY Command - network emulation delay
func (cmd *delayContext) netemDelay(c *cli.Context) error {
	return nil
}
