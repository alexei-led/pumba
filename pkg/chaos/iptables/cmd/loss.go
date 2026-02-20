package cmd

import (
	"context"
	"fmt"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/iptables"
	"github.com/urfave/cli"
)

type lossContext struct {
	context context.Context
}

// NewLossCLICommand initialize CLI loss command and bind it to the lossContext
func NewLossCLICommand(ctx context.Context) *cli.Command {
	cmdContext := &lossContext{context: ctx}
	return &cli.Command{
		Name: "loss",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "mode",
				Usage: "matching mode, supported modes are random and nth",
				Value: iptables.ModeRandom,
			},
			cli.Float64Flag{
				Name:  "probability",
				Usage: "set the probability for a packet to me matched in random mode, between 0.0 and 1.0",
				Value: 0.0,
			},
			cli.IntFlag{
				Name:  "every",
				Usage: "match one packet every nth packet, works only with nth mode",
				Value: 0,
			},
			cli.IntFlag{
				Name:  "packet",
				Usage: "set the initial counter value (0 <= packet <= n-1, default 0) for nth mode",
				Value: 0,
			},
		},
		Usage:       "adds iptables rules to generate packet loss on ingress traffic",
		ArgsUsage:   fmt.Sprintf("containers (name, list of names, or RE2 regex if prefixed with %q", chaos.Re2Prefix),
		Description: "adds packet losses on ingress traffic by setting iptable statistic rules\n \tsee:  https://www.man7.org/linux/man-pages/man8/iptables-extensions.8.html",
		Action:      cmdContext.loss,
	}
}

// IPTABLES LOSS Command - network emulation loss
func (cmd *lossContext) loss(c *cli.Context) error {
	// parse common chaos flags
	globalParams, err := chaos.ParseGlobalParams(c)
	if err != nil {
		return fmt.Errorf("error parsing global parameters: %w", err)
	}
	// parse iptables flags
	ipTablesParams, err := parseIPTablesParams(c.Parent(), globalParams.Interval)
	if err != nil {
		return fmt.Errorf("error parsing iptables parameters: %w", err)
	}

	// get mode
	mode := c.String("mode")
	// get loss probability
	probability := c.Float64("probability")
	// get every probability
	every := c.Int("every")
	// get packet
	packet := c.Int("packet")

	// init iptables loss command
	lossCommand, err := iptables.NewLossCommand(chaos.ContainerClient, globalParams, ipTablesParams, mode, probability, every, packet)
	if err != nil {
		return fmt.Errorf("error creating iptables loss command: %w", err)
	}
	// run iptables command
	err = chaos.RunChaosCommand(cmd.context, lossCommand, globalParams)
	if err != nil {
		return fmt.Errorf("error running iptables loss command: %w", err)
	}
	return nil
}
