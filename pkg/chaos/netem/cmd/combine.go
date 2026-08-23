package cmd

import (
	"context"
	"fmt"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/cliflags"
	chaoscmd "github.com/alexei-led/pumba/pkg/chaos/cmd"
	"github.com/alexei-led/pumba/pkg/chaos/netem"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/urfave/cli"
)

// CombineParams holds parameters for the explicit netem combine command.
type CombineParams struct {
	Base    *container.NetemRequest
	Limit   int
	Effects netem.CombinedSpec
}

// NewCombineCLICommand initializes the explicit combined-effects command.
func NewCombineCLICommand(ctx context.Context, runtime chaos.Runtime) *cli.Command {
	return chaoscmd.NewAction(ctx, runtime, chaoscmd.Spec[CombineParams]{
		Name:        "combine",
		Flags:       combineFlags(),
		Usage:       "apply two or more effects in one netem qdisc",
		ArgsUsage:   fmt.Sprintf("-- containers (name, list of names, or RE2 regex if prefixed with %q)", chaos.Re2Prefix),
		Description: "enable effects with --delay, --loss, --corrupt, --duplicate, and --rate; place containers after --",
		Parse:       parseCombineParams,
		Build:       buildCombineCommand,
	})
}

func parseCombineParams(c cliflags.Flags, gp *chaos.GlobalParams) (CombineParams, error) {
	base, limit, err := netem.ParseRequestBase(c.Parent(), gp)
	if err != nil {
		return CombineParams{}, fmt.Errorf("error parsing netem parameters: %w", err)
	}
	if err := validateCombineSwitches(c); err != nil {
		return CombineParams{}, err
	}
	return CombineParams{Base: base, Limit: limit, Effects: combinedSpec(c)}, nil
}

func buildCombineCommand(client container.Client, gp *chaos.GlobalParams, p CombineParams) (chaos.Command, error) {
	return netem.NewCombinedCommand(client, gp, p.Base, p.Limit, p.Effects)
}

func combinedSpec(c cliflags.Flags) netem.CombinedSpec {
	var spec netem.CombinedSpec
	if c.Bool("delay") {
		spec.Delay = &netem.DelayEffect{
			Time: c.Int("delay-time"), Jitter: c.Int("delay-jitter"),
			Correlation: c.Float64("delay-correlation"), Distribution: c.String("delay-distribution"),
		}
	}
	if c.Bool("loss") {
		spec.Loss = &netem.LossEffect{Percent: c.Float64("loss-percent"), Correlation: c.Float64("loss-correlation")}
	}
	if c.Bool("corrupt") {
		spec.Corrupt = &netem.CorruptEffect{Percent: c.Float64("corrupt-percent"), Correlation: c.Float64("corrupt-correlation")}
	}
	if c.Bool("duplicate") {
		spec.Duplicate = &netem.DuplicateEffect{Percent: c.Float64("duplicate-percent"), Correlation: c.Float64("duplicate-correlation")}
	}
	if c.Bool("rate") {
		spec.Rate = &netem.RateEffect{
			Rate: c.String("rate-value"), PacketOverhead: c.Int("rate-packet-overhead"),
			CellSize: c.Int("rate-cell-size"), CellOverhead: c.Int("rate-cell-overhead"),
		}
	}
	return spec
}

func validateCombineSwitches(c cliflags.Flags) error {
	checks := []struct {
		effect string
		flags  []string
	}{
		{effect: "delay", flags: []string{"delay-time", "delay-jitter", "delay-correlation", "delay-distribution"}},
		{effect: "loss", flags: []string{"loss-percent", "loss-correlation"}},
		{effect: "corrupt", flags: []string{"corrupt-percent", "corrupt-correlation"}},
		{effect: "duplicate", flags: []string{"duplicate-percent", "duplicate-correlation"}},
		{effect: "rate", flags: []string{"rate-value", "rate-packet-overhead", "rate-cell-size", "rate-cell-overhead"}},
	}
	for _, check := range checks {
		if c.Bool(check.effect) {
			continue
		}
		for _, name := range check.flags {
			if c.IsSet(name) {
				return fmt.Errorf("--%s requires --%s", name, check.effect)
			}
		}
	}
	return nil
}

func combineFlags() []cli.Flag {
	flags := delayCombineFlags()
	flags = append(flags, lossCombineFlags()...)
	flags = append(flags, corruptCombineFlags()...)
	flags = append(flags, duplicateCombineFlags()...)
	return append(flags, rateCombineFlags()...)
}

func delayCombineFlags() []cli.Flag {
	return []cli.Flag{
		cli.BoolFlag{Name: "delay", Usage: "enable delay"},
		cli.IntFlag{Name: "delay-time", Usage: "delay time in milliseconds", Value: defaultDelayTime},
		cli.IntFlag{Name: "delay-jitter", Usage: "delay jitter in milliseconds", Value: defaultDelayJitter},
		cli.Float64Flag{Name: "delay-correlation", Usage: "delay correlation percentage", Value: defaultDelayCorrelation},
		cli.StringFlag{Name: "delay-distribution", Usage: "delay distribution: uniform, normal, pareto, or paretonormal"},
	}
}

func lossCombineFlags() []cli.Flag {
	return []cli.Flag{
		cli.BoolFlag{Name: "loss", Usage: "enable random packet loss"},
		cli.Float64Flag{Name: "loss-percent", Usage: "packet loss percentage"},
		cli.Float64Flag{Name: "loss-correlation", Usage: "loss correlation percentage"},
	}
}

func corruptCombineFlags() []cli.Flag {
	return []cli.Flag{
		cli.BoolFlag{Name: "corrupt", Usage: "enable packet corruption"},
		cli.Float64Flag{Name: "corrupt-percent", Usage: "packet corruption percentage"},
		cli.Float64Flag{Name: "corrupt-correlation", Usage: "corruption correlation percentage"},
	}
}

func duplicateCombineFlags() []cli.Flag {
	return []cli.Flag{
		cli.BoolFlag{Name: "duplicate", Usage: "enable packet duplication"},
		cli.Float64Flag{Name: "duplicate-percent", Usage: "packet duplication percentage"},
		cli.Float64Flag{Name: "duplicate-correlation", Usage: "duplication correlation percentage"},
	}
}

func rateCombineFlags() []cli.Flag {
	return []cli.Flag{
		cli.BoolFlag{Name: "rate", Usage: "enable rate limiting"},
		cli.StringFlag{Name: "rate-value", Usage: "target egress rate", Value: defaultRate},
		cli.IntFlag{Name: "rate-packet-overhead", Usage: "per-packet overhead in bytes"},
		cli.IntFlag{Name: "rate-cell-size", Usage: "simulated link-layer cell size"},
		cli.IntFlag{Name: "rate-cell-overhead", Usage: "per-cell overhead in bytes"},
	}
}
