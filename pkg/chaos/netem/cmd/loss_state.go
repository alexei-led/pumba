package cmd

import (
	"context"
	"fmt"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/netem"
	"github.com/urfave/cli"
)

type lossStateContext struct {
	context context.Context
}

// NewLossStateCLICommand initialize CLI loss command and bind it to the lossContext
func NewLossStateCLICommand(ctx context.Context) *cli.Command {
	cmdContext := &lossStateContext{context: ctx}
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
	// get labels
	labels := c.GlobalStringSlice("label")
	// get dry-run mode
	dryRun := c.GlobalBool("dry-run")
	// get skip error flag
	skipError := c.GlobalBool("skip-error")
	// get names or pattern
	names, pattern := chaos.GetNamesOrPattern(c)
	// get global chaos interval
	interval := c.GlobalString("interval")

	// get network interface from parent `netem` command
	iface := c.Parent().String("interface")
	// get ips list from parent `netem`` command `target` flag
	ips := c.Parent().StringSlice("target")
	// get egress port list from parent `netem` command `egressPort` flag
	sports := c.Parent().String("egressPort")
	// get ingress port list from parent `netem` command `ingressPort` flag
	dports := c.Parent().String("ingressPort")
	// get duration from parent `netem`` command
	duration := c.Parent().String("duration")
	// get traffic control image from parent `netem` command
	image := c.Parent().String("tc-image")
	// get pull tc image flag
	pull := c.Parent().BoolT("pull-image")
	// get limit for number of containers to netem
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
	lossStateCommand, err := netem.NewLossStateCommand(chaos.DockerClient, names, pattern, labels, iface, ips, sports, dports, duration, interval, p13, p31, p32, p23, p14, image, pull, limit, dryRun)
	if err != nil {
		return err
	}
	// run netem command
	return chaos.RunChaosCommand(cmd.context, lossStateCommand, interval, random, skipError)
}
