package cmd

import (
	"context"
	"fmt"

	"github.com/urfave/cli"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/netem"
)

type delayContext struct {
	context context.Context
}

// NewDelayCLICommand initialize CLI delay command and bind it to the delayContext
func NewDelayCLICommand(ctx context.Context) *cli.Command {
	cmdContext := &delayContext{context: ctx}
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
		Action:      cmdContext.delay,
	}
}

// NETEM DELAY Command - network emulation delay
func (cmd *delayContext) delay(c *cli.Context) error {
	// get random flag
	random := c.GlobalBool("random")
	// get dry-run mode
	dryRun := c.GlobalBool("dry-run")
	// get names or pattern
	names, pattern := chaos.GetNamesOrPattern(c)
	// get global chaos interval
	interval := c.GlobalString("interval")

	// get network interface from parent `netem` command
	iface := c.Parent().String("interface")
	// get ips list from parent `netem`` command `target` flag
	ips := c.Parent().StringSlice("target")
	// get duration from parent `netem`` command
	duration := c.Parent().String("duration")
	// get traffic control image from parent `netem` command
	image := c.Parent().String("tc-image")
	// get limit for number of containers to netem
	limit := c.Parent().Int("limit")

	// get delay time
	time := c.Int("time")
	// get delay jitter
	jitter := c.Int("jitter")
	// get delay time
	correlation := c.Float64("correlation")
	// get delay distribution
	distribution := c.String("distribution")

	// init netem delay command
	delayCommand, err := netem.NewDelayCommand(chaos.DockerClient, names, pattern, iface, ips, duration, interval, time, jitter, correlation, distribution, image, limit, dryRun)
	if err != nil {
		return err
	}
	// run netem delay command
	return chaos.RunChaosCommand(cmd.context, delayCommand, interval, random)
}
