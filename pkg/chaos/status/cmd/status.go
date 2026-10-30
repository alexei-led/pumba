// Package cmd builds the `pumba status` CLI command.
package cmd

import (
	"context"
	"fmt"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/cliflags"
	chaoscmd "github.com/alexei-led/pumba/pkg/chaos/cmd"
	"github.com/alexei-led/pumba/pkg/chaos/status"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/urfave/cli"
)

// NewStatusCLICommand initializes the CLI status command.
func NewStatusCLICommand(ctx context.Context, runtime chaos.Runtime, events chaos.EventSink) *cli.Command {
	return chaoscmd.NewAction(ctx, runtime, events, chaoscmd.Spec[status.Params]{
		Name: "status",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "run-id",
				Usage: "only report iptables rules and sidecars of this run (qdiscs carry no run and are always reported); unlike the global --run-id, this filters",
			},
			cli.BoolFlag{
				Name:  "check",
				Usage: "print a clean/dirty/unknown verdict and exit 0 only when clean; cannot be combined with --interval",
			},
			cli.StringFlag{
				Name:  "tc-image",
				Usage: "nettools image with tc and iptables, used to inspect each target's network namespace",
				Value: "ghcr.io/alexei-led/pumba-alpine-nettools:latest",
			},
			cli.BoolTFlag{
				Name:  "pull-image",
				Usage: "force pull tc-image",
			},
		},
		Usage:     "report Pumba faults and sidecars left on containers",
		ArgsUsage: fmt.Sprintf("containers (names and RE2 regexes prefixed with %q; may be mixed; default: all running)", chaos.Re2Prefix),
		Description: "inspect each selected running container for iptables rules tagged pumba:<run-id>, qdiscs under Pumba's root handle " +
			"and Pumba sidecars; read-only toward faults (starts one short-lived inspect sidecar per target); --random and --dry-run are ignored",
		Parse: parseStatusParams,
		Build: buildStatusCommand,
	})
}

func parseStatusParams(c cliflags.Flags, gp *chaos.GlobalParams) (status.Params, error) {
	check := c.Bool("check")
	if check && gp.Interval > 0 {
		return status.Params{}, status.ErrCheckWithInterval
	}
	runFilter := c.String("run-id")
	if runFilter != "" {
		if err := chaos.ValidateRunID(runFilter); err != nil {
			return status.Params{}, fmt.Errorf("status: %w", err)
		}
	}
	return status.Params{
		RunFilter: runFilter,
		Check:     check,
		Sidecar:   container.SidecarSpec{Image: c.String("tc-image"), Pull: c.BoolT("pull-image")},
	}, nil
}

func buildStatusCommand(client container.Client, gp *chaos.GlobalParams, p status.Params) (chaos.Command, error) {
	return status.NewCommand(client, gp, p), nil
}
