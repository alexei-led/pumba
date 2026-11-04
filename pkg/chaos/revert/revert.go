// Package revert implements bounded, run-scoped recovery. Stop the original
// writer before invoking it: ownership labels are not a liveness lock.
package revert

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
)

// Params separates the required recovery filter from the invocation ID.
type Params struct {
	RunFilter string
	Sidecar   container.SidecarSpec
}

type recoveryClient interface {
	container.Lister
	container.Inspector
	container.Recovery
}

type command struct {
	client recoveryClient
	gp     *chaos.GlobalParams
	p      Params
}

type targetPlan struct {
	request    *container.RuleRecoveryRequest
	plan       *container.CommandPlan
	sidecars   []container.SidecarIdentity
	unresolved []string
}

// NewCommand validates recovery-only scheduling and scope constraints.
func NewCommand(client recoveryClient, gp *chaos.GlobalParams, p Params) (chaos.Command, error) {
	if err := chaos.ValidateRunID(p.RunFilter); err != nil {
		return nil, fmt.Errorf("revert requires a local --run-id filter: %w", err)
	}
	if gp.Interval != 0 || gp.Random || gp.SkipErrors {
		return nil, errors.New("revert rejects --interval, --random and --skip-error")
	}
	if p.RunFilter == gp.RunID {
		return nil, errors.New("revert filter must differ from the invocation --run-id")
	}
	return &command{client: client, gp: gp, p: p}, nil
}

// Run inventories targets even without sidecars, preflights every selected
// namespace, then dispatches narrow typed work through the canonical guard.
func (c *command) Run(ctx context.Context, random bool) error {
	if random {
		return errors.New("revert rejects random selection")
	}
	work, byTarget, unresolved, err := c.inventory(ctx)
	if err != nil {
		return err
	}
	if err := chaos.CheckPrepared(c.gp, work); err != nil {
		return err
	}
	plans, preflightErr := c.preflight(ctx, work, byTarget)
	if preflightErr != nil {
		return preflightErr
	}
	runErr := chaos.RunPrepared(ctx, c.gp, work,
		func(ctx context.Context, target *container.Container, t *chaos.Target) error {
			return c.recoverTarget(ctx, plans[target.ID()], t)
		},
		func(ctx context.Context, artifact container.SidecarIdentity) error {
			if c.gp.DryRun {
				return nil
			}
			return c.client.RemoveRecoverySidecar(ctx, artifact)
		})
	if !c.gp.DryRun && ctx.Err() == nil {
		runErr = errors.Join(runErr, c.verifySidecars(ctx, work))
	}
	if len(unresolved) > 0 {
		runErr = errors.Join(runErr, fmt.Errorf("unresolved coverage: %s", strings.Join(unresolved, "; ")))
	}
	return runErr
}

func (c *command) preflight(ctx context.Context, work chaos.PreparedWork, sidecars map[string][]container.SidecarIdentity) (map[string]*targetPlan, error) {
	plans := map[string]*targetPlan{}
	var invalid []error
	for _, target := range work.Targets {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		p := &targetPlan{sidecars: sidecars[target.ID()], request: &container.RuleRecoveryRequest{
			Container: target, Sidecar: c.p.Sidecar, RunFilter: c.p.RunFilter, InvocationID: c.gp.RunID,
		}}
		plans[target.ID()] = p
		state, err := c.inspect(ctx, target)
		if err != nil {
			p.unresolved = append(p.unresolved, fmt.Sprintf("%s namespace inspection failed: %v", target.ID(), err))
			continue
		}
		for _, rule := range state.Rules {
			if rule.RunID == c.p.RunFilter {
				p.request.Rules = append(p.request.Rules, rule)
			}
		}
		if len(state.Qdiscs) > 0 {
			p.unresolved = append(p.unresolved, "qdiscs are not attributable to a run; not removed")
		}
		p.plan, err = c.client.PlanRecoveryRules(p.request)
		if err != nil {
			invalid = append(invalid, fmt.Errorf("%s invalid recovery plan: %w", target.ID(), err))
		}
	}
	return plans, errors.Join(invalid...)
}

func (c *command) inspect(ctx context.Context, target *container.Container) (*container.FaultState, error) {
	state, err := c.client.InspectNetns(ctx, &container.InspectRequest{Container: target, Sidecar: c.p.Sidecar, RunID: c.gp.RunID})
	if err == nil && state == nil {
		err = errors.New("empty namespace inspection response")
	}
	return state, err
}

func (c *command) recoverTarget(ctx context.Context, p *targetPlan, t *chaos.Target) error {
	t.SetRecovery(p.plan, p.sidecars, p.unresolved)
	var errs []error
	if !c.gp.DryRun {
		if p.plan != nil && ctx.Err() == nil {
			errs = append(errs, c.client.RecoverRules(ctx, p.request))
		}
		for _, sidecar := range p.sidecars {
			if ctx.Err() != nil {
				break
			}
			errs = append(errs, c.client.RemoveRecoverySidecar(ctx, sidecar))
		}
		if ctx.Err() == nil {
			state, err := c.inspect(ctx, p.request.Container)
			if err == nil {
				for _, rule := range state.Rules {
					if rule.RunID == c.p.RunFilter {
						err = errors.New("recovery verification found remaining rules")
						break
					}
				}
				if len(state.Qdiscs) > 0 {
					err = errors.Join(err, errors.New("unattributable qdiscs remain"))
				}
			}
			errs = append(errs, err)
		}
		errs = append(errs, ctx.Err())
		t.Reverted(errors.Join(errs...))
	}
	if len(p.unresolved) > 0 {
		errs = append(errs, fmt.Errorf("unresolved coverage: %s", strings.Join(p.unresolved, "; ")))
	}
	return errors.Join(errs...)
}
