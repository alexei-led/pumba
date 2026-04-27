package chaos

import (
	"context"
	"fmt"

	"github.com/alexei-led/pumba/pkg/container"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

// ContainerAction applies a chaos action to a single target container.
// Returning an error from any closure aborts a parallel run via errgroup
// and aborts a serial run on first error.
type ContainerAction func(ctx context.Context, c *container.Container) error

// RunOnContainers lists running containers matching gp.{Names,Pattern,Labels}
// (capped by limit), optionally narrows to a single random pick when random
// is true, then invokes fn for each container. parallel selects between
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
