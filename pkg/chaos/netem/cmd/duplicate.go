package cmd

import (
	"context"
	"fmt"

	"github.com/urfave/cli"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/netem"
)

type duplicateContext struct {
	context context.Context
}

// NewDuplicateCLICommand initialize CLI duplicate command and bind it to the duplicateContext
func NewDuplicateCLICommand(ctx context.Context) *cli.Command {
	cmdContext := &duplicateContext{context: ctx}
	return &cli.Command{
		Name: "duplicate",
		Flags: []cli.Flag{
			cli.Float64Flag{
				Name:  "percent, p",
				Usage: "packet duplicate percentage",
				Value: 0.0,
			},
			cli.Float64Flag{
				Name:  "correlation, c",
				Usage: "duplicate correlation; in percentage",
				Value: 0.0,
			},
		},
		Usage:       "adds packet duplication",
		ArgsUsage:   fmt.Sprintf("containers (name, list of names, or RE2 regex if prefixed with %q", chaos.Re2Prefix),
		Description: "adds packet duplication, based on independent (Bernoulli) probability model\n \tsee:  http://www.voiptroubleshooter.com/indepth/burstloss.html",
		Action:      cmdContext.duplicate,
	}
}

// NETEM Duplicate Command - network emulation duplicate
func (cmd *duplicateContext) duplicate(c *cli.Context) error {
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

	// get duplicate percentage
	percent := c.Float64("percent")
	// get delay variation
	correlation := c.Float64("correlation")

	// init netem duplicate command
	duplicateCommand, err := netem.NewDuplicateCommand(chaos.DockerClient, names, pattern, iface, ips, duration, interval, percent, correlation, image, limit, dryRun)
	if err != nil {
		return err
	}
	// run netem command
	return chaos.RunChaosCommand(cmd.context, duplicateCommand, interval, random)
}
