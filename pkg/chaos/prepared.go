package chaos

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/alexei-led/pumba/pkg/container"
	"golang.org/x/sync/errgroup"
)

// PreparedWork is a complete recovery inventory. Orphans identify real
// artifacts, not invented target containers. Every original target counts
// once toward the safety cap, even when it owns multiple artifacts.
type PreparedWork struct {
	Targets []*container.Container
	Orphans []container.SidecarIdentity
}

// CheckPrepared applies the canonical target cap before expensive inspection.
func CheckPrepared(gp *GlobalParams, work PreparedWork) error {
	ids := map[string]bool{}
	names := []string{}
	for _, c := range work.Targets {
		if c == nil || c.ID() == "" {
			return errors.New("invalid prepared target")
		}
		if ids[c.ID()] {
			return fmt.Errorf("duplicate prepared target %s", c.ID())
		}
		ids[c.ID()] = true
		if len(names) < maxTargetNames {
			names = append(names, c.Name())
		}
	}
	for _, a := range work.Orphans {
		if err := a.Validate(); err != nil {
			return err
		}
		if !ids[a.TargetID] && len(names) < maxTargetNames {
			names = append(names, a.TargetID)
		}
		ids[a.TargetID] = true
	}
	return checkTargetCount(len(ids), gp.MaxTargets, names)
}

// RunPrepared dispatches recovery serially after its complete preflight. It
// attempts independent work after failures, but never admits work after cancel.
// It emits only target.result and artifact.result, never target.applied.
func RunPrepared(ctx context.Context, gp *GlobalParams, work PreparedWork, target ContainerAction,
	artifact func(context.Context, container.SidecarIdentity) error,
) error {
	if err := CheckPrepared(gp, work); err != nil {
		return err
	}
	tick := gp.ticks.Add(1)
	count := len(work.Targets) + len(work.Orphans)
	run := func(i int) error {
		if i < len(work.Targets) {
			c := work.Targets[i]
			t := newTarget(gp, tick, c)
			err := target(ctx, c, t)
			t.finish(err)
			return err
		}
		a := work.Orphans[i-len(work.Targets)]
		started := time.Now().UTC()
		err := artifact(ctx, a)
		status := TargetOK
		var evErr *EventError
		if err != nil {
			status, evErr = TargetCleanupFailed, newEventError(CodeCleanupFailed, PhaseCleanup, err)
		} else if gp.DryRun {
			status = TargetPlanned
		}
		gp.Emit(&Event{Type: EventArtifactResult, Artifact: &ArtifactResult{Artifact: a, Started: started, Ended: time.Now().UTC(), Status: status, Error: evErr}})
		return err
	}
	skip := func(i int) {
		if i < len(work.Targets) {
			skipTargets(gp, tick, work.Targets[i:i+1])
			return
		}
		now := time.Now().UTC()
		gp.Emit(&Event{Type: EventArtifactResult, Artifact: &ArtifactResult{Artifact: work.Orphans[i-len(work.Targets)], Started: now, Ended: now, Status: TargetSkipped}})
	}
	err := dispatch(ctx, count, false, true, run, skip)
	return errors.Join(err, ctx.Err())
}

// dispatch is the common admission/dispatch loop for discovered and prepared
// work. Recovery keeps going after independent errors; injection stops serially.
func dispatch(ctx context.Context, count int, parallel, keepGoing bool, run func(int) error, skip func(int)) error {
	if parallel {
		var eg errgroup.Group
		for i := range count {
			eg.Go(func() error {
				return run(i)
			})
		}
		return eg.Wait()
	}
	var errs []error
	stopped := false
	for i := range count {
		if stopped || ctx.Err() != nil {
			skip(i)
			continue
		}
		if err := run(i); err != nil {
			errs = append(errs, err)
			stopped = !keepGoing
		}
	}
	if len(errs) == 1 {
		return errs[0]
	}
	return errors.Join(errs...)
}
