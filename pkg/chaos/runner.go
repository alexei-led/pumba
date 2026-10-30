package chaos

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/alexei-led/pumba/pkg/container"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

// ContainerAction applies a chaos action to a single target container.
// In serial runs, the first error stops iteration. In parallel runs, every
// closure runs to completion regardless of errors; errgroup.Wait returns
// the first reported error.
//
// t reports beyond the error: call t.Applied() the moment the fault becomes
// active (one-shot actions: once the action ran) and t.Reverted(err) with the
// outcome of reverting it. RunOnContainers emits the target's terminal
// target.result when the action returns, unless it deferred that with
// t.DeferResult().
type ContainerAction func(ctx context.Context, c *container.Container, t *Target) error

// RunOnContainers lists running containers matching gp.{Names,Pattern,Labels}
// (capped by limit), optionally narrows to a single random pick when random
// is true, checks gp.MaxTargets, then invokes fn for each container. Callers
// must pass the *GlobalParams they received and preserve wrapped errors with
// %w: over-cap selections wrap ErrMaxTargetsExceeded and run no callbacks.
// Negative caps return ErrNegativeMaxTargets. parallel selects between
// errgroup fanout (true) and a sequential for-loop (false). Returns nil when
// no containers match — same warning the per-action loops used to log.
//
// Each call is one tick of the run: target events carry a tick counter kept
// in gp, starting at 1. Every selected target ends with one target.result;
// targets a serial run did not reach after a failure are reported skipped.
// Refusals and empty selections emit no target events.
//
// The helper takes container.Lister rather than the per-action narrow client
// interface so it stays domain-agnostic; every action's client embeds Lister.
//
// Example:
//
//	return chaos.RunOnContainers(ctx, n.client, n.gp, n.limit, random, true,
//	    func(ctx context.Context, c *container.Container, t *chaos.Target) error {
//	        netemCtx, cancel := context.WithCancel(ctx)
//	        defer cancel()
//	        req := *n.req
//	        req.Container = c
//	        req.Command = netemCmd
//	        return runNetem(netemCtx, n.client, &req, t)
//	    })
func RunOnContainers(
	ctx context.Context,
	lister container.Lister,
	gp *GlobalParams,
	limit int,
	random, parallel bool,
	fn ContainerAction,
) error {
	return runOnContainers(ctx, lister, gp, limit, false, random, parallel, fn)
}

// RunOnContainersAll behaves like RunOnContainers but also includes stopped
// containers in the candidate set. Used by lifecycle.remove which can target
// non-running containers.
func RunOnContainersAll(
	ctx context.Context,
	lister container.Lister,
	gp *GlobalParams,
	limit int,
	random, parallel bool,
	fn ContainerAction,
) error {
	return runOnContainers(ctx, lister, gp, limit, true, random, parallel, fn)
}

func runOnContainers(
	ctx context.Context,
	lister container.Lister,
	gp *GlobalParams,
	limit int,
	all, random, parallel bool,
	fn ContainerAction,
) error {
	tick := gp.ticks.Add(1)
	if gp.MaxTargets < 0 {
		return ErrNegativeMaxTargets
	}
	containers, err := container.ListNContainersAll(ctx, lister, gp.Names, gp.Pattern, gp.Labels, limit, all)
	if err != nil {
		return &codedError{code: CodeRuntimeUnavailable, phase: PhaseSelect, err: fmt.Errorf("listing containers: %w", err)}
	}
	if len(containers) == 0 {
		log.Warning("no containers found")
		return nil
	}
	if random {
		if c := container.RandomContainer(containers); c != nil {
			containers = []*container.Container{c}
		}
	}
	if err := checkMaxTargets(containers, gp.MaxTargets); err != nil {
		return err
	}
	run := func(c *container.Container) error {
		t := newTarget(gp, tick, c)
		err := fn(ctx, c, t)
		t.finish(err)
		return err
	}
	if !parallel {
		for i, c := range containers {
			if err := run(c); err != nil {
				skipTargets(gp, tick, containers[i+1:])
				return err
			}
		}
		return nil
	}
	var eg errgroup.Group
	for _, c := range containers {
		eg.Go(func() error { return run(c) })
	}
	return eg.Wait()
}

// ErrNegativeMaxTargets rejects invalid caps before listing or injecting.
var ErrNegativeMaxTargets = errors.New("--max-targets must be non-negative")

// ErrMaxTargetsExceeded marks safety refusals, which --skip-error must not suppress.
var ErrMaxTargetsExceeded = errors.New("refusing to run")

// Keep refusal messages bounded even when a selector matches thousands of containers.
const maxTargetNames = 10

func checkMaxTargets(containers []*container.Container, maxTargets int) error {
	if maxTargets == 0 || len(containers) <= maxTargets {
		return nil
	}
	names := make([]string, 0, min(len(containers), maxTargetNames))
	for _, c := range containers[:min(len(containers), maxTargetNames)] {
		names = append(names, c.Name())
	}
	return fmt.Errorf("%w: %d containers selected, --max-targets is %d (first %d: %s)", ErrMaxTargetsExceeded, len(containers), maxTargets, len(names), strings.Join(names, ", "))
}
