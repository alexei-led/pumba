// Package cmd provides a generic CLI command builder shared by every chaos
// action. NewAction[P] removes the per-command boilerplate previously
// duplicated across 17 builders: resolve runtime, parse global + per-command
// params, build the chaos.Command, run it through chaos.RunChaosCommand.
package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/chaos/cliflags"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/urfave/cli"
)

// ParamParser turns CLI flag values plus already-parsed GlobalParams into the
// per-command parameter struct P. Returning an error short-circuits the action.
// The cliflags.Flags abstraction keeps parsers independent of the underlying
// CLI library version.
type ParamParser[P any] func(c cliflags.Flags, gp *chaos.GlobalParams) (P, error)

// CommandFactory builds a chaos.Command from the resolved client, GlobalParams,
// and per-command params P. Returning an error short-circuits the action.
type CommandFactory[P any] func(client container.Client, gp *chaos.GlobalParams, p P) (chaos.Command, error)

// Spec describes a single chaos CLI subcommand. Every field except Parse and
// Build is plumbed straight onto the resulting *cli.Command.
type Spec[P any] struct {
	Name        string
	Usage       string
	ArgsUsage   string
	Description string
	Flags       []cli.Flag
	// RequireArgs makes NewAction return the canonical
	// "container name ... required" error if no positional args were given.
	RequireArgs bool
	Parse       ParamParser[P]
	Build       CommandFactory[P]
}

// ErrContainerArgRequired is returned when a chaos action requires at least
// one container target (name, list, or re2: regex) and none were given. Same
// message previously hard-coded in every per-command Action closure.
var ErrContainerArgRequired = errors.New("container name, list of names, or RE2 regex is required")

// NewAction wires a chaos action to its CLI subcommand. The Action closure
// resolves the runtime client, parses global + per-command params, builds the
// chaos.Command, and runs it through chaos.RunChaosCommand.
func NewAction[P any](ctx context.Context, runtime chaos.Runtime, spec Spec[P]) *cli.Command {
	return &cli.Command{
		Name:        spec.Name,
		Usage:       spec.Usage,
		ArgsUsage:   spec.ArgsUsage,
		Description: spec.Description,
		Flags:       spec.Flags,
		Action: func(c *cli.Context) error {
			if spec.RequireArgs && !c.Args().Present() {
				return ErrContainerArgRequired
			}
			f := cliflags.NewV1(c)
			gp := chaos.ParseGlobalParams(f)
			p, err := spec.Parse(f, gp)
			if err != nil {
				return err
			}
			cmd, err := spec.Build(runtime(), gp, p)
			if err != nil {
				return err
			}
			if err := chaos.RunChaosCommand(ctx, cmd, gp); err != nil {
				return fmt.Errorf("running %s: %w", spec.Name, err)
			}
			return nil
		},
	}
}
