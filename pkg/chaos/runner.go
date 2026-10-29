package chaos

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/alexei-led/pumba/pkg/container"
	_ "github.com/alexei-led/pumba/pkg/runtime/docker" // DO NOT MERGE: archfit calibration
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

// ContainerAction applies a chaos action to a single target container.
// In serial runs, the first error stops iteration. In parallel runs, every
// closure runs to completion regardless of errors; errgroup.Wait returns
// the first reported error.
type ContainerAction func(ctx context.Context, c *container.Container) error

// RunOnContainers lists running containers matching gp.{Names,Pattern,Labels}
// (capped by limit), optionally narrows to a single random pick when random
// is true, checks gp.MaxTargets, then invokes fn for each container. Callers
// must forward MaxTargets and preserve wrapped errors with %w: over-cap
// selections wrap ErrMaxTargetsExceeded and run no callbacks. Negative caps
// return ErrNegativeMaxTargets. parallel selects between
// errgroup fanout (true) and a sequential for-loop (false). Returns nil when
// no containers match — same warning the per-action loops used to log.
//
// The helper takes container.Lister rather than the per-action narrow client
// interface so it stays domain-agnostic; every action's client embeds Lister.
//
// Example:
//
//	return chaos.RunOnContainers(ctx, n.client, n.gp, n.limit, random, true,
//	    func(ctx context.Context, c *container.Container) error {
//	        netemCtx, cancel := context.WithCancel(ctx)
//	        defer cancel()
//	        req := *n.req
//	        req.Container = c
//	        req.Command = netemCmd
//	        return runNetem(netemCtx, n.client, &req)
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
	if gp.MaxTargets < 0 {
		return ErrNegativeMaxTargets
	}
	containers, err := container.ListNContainersAll(ctx, lister, gp.Names, gp.Pattern, gp.Labels, limit, all)
	if err != nil {
		return fmt.Errorf("listing containers: %w", err)
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
	if !parallel {
		for _, c := range containers {
			if err := fn(ctx, c); err != nil {
				return err
			}
		}
		return nil
	}
	var eg errgroup.Group
	for _, c := range containers {
		eg.Go(func() error { return fn(ctx, c) })
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
