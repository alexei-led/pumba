package cmd

import (
	"context"
	"fmt"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/cliflags"
	chaoscmd "github.com/alexei-led/pumba/pkg/chaos/cmd"
	"github.com/alexei-led/pumba/pkg/chaos/iptables"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/urfave/cli"
)

// LossParams holds the per-command parameters for the iptables loss subcommand.
type LossParams struct {
	IPTables    *iptables.Params
	Mode        string
	Probability float64
	Every       int
	Packet      int
}

// NewLossCLICommand initialize CLI loss command.
func NewLossCLICommand(ctx context.Context, runtime chaos.Runtime) *cli.Command {
	return chaoscmd.NewAction(ctx, runtime, chaoscmd.Spec[LossParams]{
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
		Parse:       parseLossParams,
		Build:       buildLossCommand,
	})
}

func parseLossParams(c cliflags.Flags, gp *chaos.GlobalParams) (LossParams, error) {
	ipTablesParams, err := parseIPTablesParams(c.Parent(), gp.Interval)
	if err != nil {
		return LossParams{}, fmt.Errorf("error parsing iptables parameters: %w", err)
	}
	return LossParams{
		IPTables:    ipTablesParams,
		Mode:        c.String("mode"),
		Probability: c.Float64("probability"),
		Every:       c.Int("every"),
		Packet:      c.Int("packet"),
	}, nil
}

func buildLossCommand(client container.Client, gp *chaos.GlobalParams, p LossParams) (chaos.Command, error) {
	return iptables.NewLossCommand(client, gp, p.IPTables, p.Mode, p.Probability, p.Every, p.Packet)
}
