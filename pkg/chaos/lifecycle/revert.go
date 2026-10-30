package lifecycle

import (
	"context"
	"errors"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	log "github.com/sirupsen/logrus"
)

// heldTarget is a container a batch action (pause, stop --restart) changed and
// must revert after fanout, with the func that emits its terminal result.
type heldTarget struct {
	c    *container.Container
	done func(revertErr error)
}

func holdTarget(c *container.Container, t *chaos.Target) heldTarget {
	return heldTarget{c: c, done: t.DeferResult()}
}

// revertTimeout bounds each revert, so a slow unpause or start is not cut off
// by a short --duration and one stuck revert cannot starve the later ones.
const revertTimeout = 30 * time.Second

// detachedRevert runs revert on a context detached from ctx, so a SIGTERM or
// --max-duration deadline cannot cut it off, with its own timeout bound.
func detachedRevert(ctx context.Context, timeout time.Duration, c *container.Container,
	revert func(context.Context, *container.Container) error,
) error {
	revertCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()
	return revert(revertCtx, c)
}

// revertInterrupted undoes an action that failed while ctx was cancelled. The
// runtime call may have taken effect before it noticed the cancellation, so
// the container is reverted anyway, on a detached context bounded by timeout,
// and the target reports the action error with its cleanup. It returns err
// unchanged.
func revertInterrupted(ctx context.Context, timeout time.Duration, c *container.Container, t *chaos.Target, err error,
	revert func(context.Context, *container.Container) error,
) error {
	if ctx.Err() != nil {
		t.Reverted(detachedRevert(ctx, timeout, c, revert))
	}
	return err
}

// revertAfter waits for duration or ctx cancellation (dry-runs do not wait),
// then reverts each held target and reports each target's own outcome, so no
// target is reported before its own revert finished. Each revert runs on its
// own context detached from ctx (see detachedRevert).
func revertAfter(ctx context.Context, duration time.Duration, dryRun bool, held []heldTarget,
	revert func(context.Context, *container.Container) error,
) error {
	if len(held) == 0 {
		return nil
	}
	if !dryRun {
		timer := time.NewTimer(duration)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			log.Debug("reverting containers by stop event")
		case <-timer.C:
			log.WithField("duration", duration).Debug("reverting containers after duration")
		}
	}
	var err error
	for _, h := range held {
		e := detachedRevert(ctx, revertTimeout, h.c, revert)
		h.done(e)
		err = errors.Join(err, e)
	}
	return err
}
