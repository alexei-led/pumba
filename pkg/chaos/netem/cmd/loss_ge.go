package cmd

import (
	"context"
	"fmt"

	"github.com/urfave/cli"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/netem"
	"github.com/alexei-led/pumba/pkg/container"
)

type lossGEContext struct {
	client  container.Client
	context context.Context
}

// NewLossGECLICommand initialize CLI loss gemodel command and bind it to the lossContext
func NewLossGECLICommand(ctx context.Context, client container.Client) *cli.Command {
	cmdContext := &lossGEContext{client: client, context: ctx}
	return &cli.Command{
		Name: "loss-gemodel",
		Flags: []cli.Flag{
			cli.Float64Flag{
				Name:  "pg, p",
				Usage: "transition probability into the bad state",
				Value: 0.0,
			},
			cli.Float64Flag{
				Name:  "pb, r",
				Usage: "transition probability into the good state",
				Value: 100.0,
			},
			cli.Float64Flag{
				Name:  "one-h",
				Usage: "loss probability in the bad state",
				Value: 100.0,
			},
			cli.Float64Flag{
				Name:  "one-k",
				Usage: "loss probability in the good state",
				Value: 0.0,
			},
		},
		Usage:     "adds packet losses, according to the Gilbert-Elliot loss model",
		ArgsUsage: fmt.Sprintf("containers (name, list of names, or RE2 regex if prefixed with %q", chaos.Re2Prefix),
		Description: `adds packet losses, according to the Gilbert-Elliot loss model
	 see detailed description: http://www.voiptroubleshooter.com/indepth/burstloss.html`,
		Action: cmdContext.lossGE,
	}
}

// NETEM LOSS GEMODEL Command - network emulation loss by Gilbert-Elliot model
func (cmd *lossGEContext) lossGE(c *cli.Context) error {
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

	// Good State transition probability
	pg := c.Float64("pg")
	// Bad State transition probability
	pb := c.Float64("pb")
	// loss probability in Bad state
	oneH := c.Float64("one-h")
	// loss probability in Good state
	oneK := c.Float64("one-k")

	// init netem loss gemodel command
	lossGECommand, err := netem.NewLossGECommand(cmd.client, names, pattern, iface, ips, duration, interval, pg, pb, oneH, oneK, image, limit, dryRun)
	if err != nil {
		return nil
	}
	// run netem command
	return chaos.RunChaosCommand(cmd.context, lossGECommand, interval, random)
}
