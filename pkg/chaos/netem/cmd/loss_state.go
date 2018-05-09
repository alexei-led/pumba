package cmd

import (
	"context"
	"fmt"

	"github.com/urfave/cli"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/netem"
	"github.com/alexei-led/pumba/pkg/container"
)

type lossStateContext struct {
	client  container.Client
	context context.Context
}

// NewLossStateCLICommand initialize CLI loss command and bind it to the lossContext
func NewLossStateCLICommand(ctx context.Context, client container.Client) *cli.Command {
	cmdContext := &lossStateContext{client: client, context: ctx}
	return &cli.Command{
		Name: "loss-state",
		Flags: []cli.Flag{
			cli.Float64Flag{
				Name:  "p13",
				Usage: "probability to go from state (1) to state (3)",
				Value: 0.0,
			},
			cli.Float64Flag{
				Name:  "p31",
				Usage: "probability to go from state (3) to state (1)",
				Value: 100.0,
			},
			cli.Float64Flag{
				Name:  "p32",
				Usage: "probability to go from state (3) to state (2)",
				Value: 0.0,
			},
			cli.Float64Flag{
				Name:  "p23",
				Usage: "probability to go from state (2) to state (3)",
				Value: 100.0,
			},
			cli.Float64Flag{
				Name:  "p14",
				Usage: "probability to go from state (1) to state (4)",
				Value: 0.0,
			},
		},
		Usage:     "adds packet losses, based on 4-state Markov probability model",
		ArgsUsage: fmt.Sprintf("containers (name, list of names, or RE2 regex if prefixed with %q", chaos.Re2Prefix),
		Description: `adds a packet losses, based on 4-state Markov probability model:

		state (1) – packet received successfully
		state (2) – packet received within a burst
		state (3) – packet lost within a burst
		tstate (4) – isolated packet lost within a gap

	 see detailed description: http://www.voiptroubleshooter.com/indepth/burstloss.html`,
		Action: cmdContext.lossState,
	}
}

// NETEM LOSS STATE Command - network emulation loss 4-state Markov
func (cmd *lossStateContext) lossState(c *cli.Context) error {
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
	// get limit for number of containers to kill
	limit := c.Parent().Int("limit")

	// get loss p13 state probability
	p13 := c.Float64("p13")
	// get loss p31 state probability
	p31 := c.Float64("p31")
	// get loss p32 state probability
	p32 := c.Float64("p32")
	// get loss p23 state probability
	p23 := c.Float64("p23")
	// get loss p23 state probability
	p14 := c.Float64("p14")

	// init netem loss state command
	lossStateCommand, err := netem.NewLossStateCommand(cmd.client, names, pattern, iface, ips, duration, interval, p13, p31, p32, p23, p14, image, limit, dryRun)
	if err != nil {
		return nil
	}
	// run netem command
	return chaos.RunChaosCommand(cmd.context, lossStateCommand, interval, random)
}
