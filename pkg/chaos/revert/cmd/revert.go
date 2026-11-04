// Package cmd builds the bounded run-scoped recovery command.
package cmd

import (
	"context"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/cliflags"
	chaoscmd "github.com/alexei-led/pumba/pkg/chaos/cmd"
	"github.com/alexei-led/pumba/pkg/chaos/revert"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/urfave/cli"
)

// NewRevertCLICommand initializes the CLI recovery command.
func NewRevertCLICommand(ctx context.Context, runtime chaos.Runtime, events chaos.EventSink) *cli.Command {
	return chaoscmd.NewAction(ctx, runtime, events, chaoscmd.Spec[revert.Params]{
		Name: "revert", Usage: "recover tagged iptables rules and sidecars of a stopped Pumba run",
		ArgsUsage:   "containers (exact names, full IDs or re2: patterns; default all running targets and matching orphan sidecars)",
		Description: "Stop the original writer first. Requires local --run-id FILTER, distinct from the invocation ID. Removes only validated filter-table rules and netem/iptables/stress sidecars, including orphans. Never removes qdiscs, targets or volumes. Missing namespace coverage is unresolved (exit 1). --dry-run inventories and prints plans without deleting faults. Rejects --interval, --random and --skip-error.",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "run-id", Usage: "required original run ID FILTER (not this invocation's global --run-id)"},
			cli.StringFlag{Name: "tc-image", Usage: "nettools image for read-only inspection and rule recovery", Value: "ghcr.io/alexei-led/pumba-alpine-nettools:latest"},
			cli.BoolTFlag{Name: "pull-image", Usage: "force pull tc-image"},
		},
		Parse: func(f cliflags.Flags, _ *chaos.GlobalParams) (revert.Params, error) {
			return revert.Params{RunFilter: f.String("run-id"), Sidecar: container.SidecarSpec{Image: f.String("tc-image"), Pull: f.BoolT("pull-image")}}, nil
		},
		Build: func(client container.Client, gp *chaos.GlobalParams, p revert.Params) (chaos.Command, error) {
			return revert.NewCommand(client, gp, p)
		},
	})
}
