package cmd

import (
	"context"
	"fmt"

	"github.com/urfave/cli"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/netem"
)

type corruptContext struct {
	context context.Context
}

// NewCorruptCLICommand initialize CLI corrupt command and bind it to the corruptContext
func NewCorruptCLICommand(ctx context.Context) *cli.Command {
	cmdContext := &corruptContext{context: ctx}
	return &cli.Command{
		Name: "corrupt",
		Flags: []cli.Flag{
			cli.Float64Flag{
				Name:  "percent, p",
				Usage: "packet corrupt percentage",
				Value: 0.0,
			},
			cli.Float64Flag{
				Name:  "correlation, c",
				Usage: "corrupt correlation; in percentage",
				Value: 0.0,
			},
		},
		Usage:       "adds packet corruption",
		ArgsUsage:   fmt.Sprintf("containers (name, list of names, or RE2 regex if prefixed with %q", chaos.Re2Prefix),
		Description: "adds packet corruption, based on independent (Bernoulli) probability model\n \tsee:  http://www.voiptroubleshooter.com/indepth/burstloss.html",
		Action:      cmdContext.corrupt,
	}
}

// NETEM Corrupt Command - network emulation corrupt
func (cmd *corruptContext) corrupt(c *cli.Context) error {
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

	// get corrupt percentage
	percent := c.Float64("percent")
	// get delay variation
	correlation := c.Float64("correlation")

	// init netem corrupt command
	corruptCommand, err := netem.NewCorruptCommand(chaos.DockerClient, names, pattern, iface, ips, duration, interval, percent, correlation, image, limit, dryRun)
	if err != nil {
		return err
	}
	// run netem command
	return chaos.RunChaosCommand(cmd.context, corruptCommand, interval, random)
}
